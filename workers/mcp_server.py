"""MCP server: lets AI agents work an RFP through the platform.

Seven tools take an agent from a project's questions to signed-off answers:

  list_projects        workspaces and their RFP projects
  get_rfp_questions    every question in a project with its answer and status
  search_documents     semantic search over the project's knowledge base
  draft_answer         have the platform draft (or redraft) a cited answer
  edit_answer          replace an answer's text
  run_quality_checks   the 13 pre-approval checks (src/quality.py)
  approve_answer       sign off, refused while any blocking check fails

Everything goes through the REST API as the configured user, so the agent can
only see and change what that user can. Search is the exception: it runs the
workers' retrieval directly, after the API confirms the user can open the
project.

Run over stdio (how MCP clients such as Claude Code launch it):
    RFP_EMAIL=... RFP_PASSWORD=... python workers/mcp_server.py
"""

import logging
import os
from functools import cache

from mcp.server.mcpserver import MCPServer
from mcp.server.mcpserver.exceptions import ToolError
from mcp.types import ToolAnnotations

from src.api_client import ApiClient, ApiError, wait_for
from src.quality import WARNING, Draft, blockers, run_checks
from src.retrieval import search

API_URL = os.getenv("RFP_API_URL", "http://localhost:8080")

logging.getLogger("httpx").setLevel(logging.WARNING)  # one log line per API call drowns the client's view

server = MCPServer(
    name="agentic-rfp",
    instructions=(
        "Tools for answering RFPs (requests for proposal) from a company's own documentation. "
        "Typical flow: list_projects -> get_rfp_questions -> draft_answer for each question -> "
        "run_quality_checks -> fix problems with edit_answer or search_documents -> approve_answer. "
        "Answers cite their sources inline as [n]; never invent facts or citations."
    ),
)

READ_ONLY = ToolAnnotations(read_only_hint=True, open_world_hint=False)
WRITES = ToolAnnotations(read_only_hint=False, destructive_hint=False, open_world_hint=False)


@cache
def api() -> ApiClient:
    """The signed-in API client, created on first use."""
    client = ApiClient(API_URL, token=os.getenv("RFP_TOKEN"))
    if not client.token:
        email, password = os.getenv("RFP_EMAIL"), os.getenv("RFP_PASSWORD")
        if not email or not password:
            raise ToolError("Set RFP_TOKEN, or RFP_EMAIL and RFP_PASSWORD, for the MCP server.")
        client.login(email, password)
    return client


def call(method: str, path: str, body: dict | None = None):
    """Make an API call, turning HTTP errors into tool errors the agent can read."""
    try:
        return api().request(method, path, body)
    except ApiError as e:
        if e.status == 404:
            raise ToolError(f"Not found: {path}") from e
        raise ToolError(e.message or f"API error {e.status}") from e


def answer_view(row: dict) -> dict:
    """The shape every tool returns for an answer."""
    return {
        "answer_id": row["id"],
        "question": row["question"],
        "status": row["status"],
        "answer": row.get("answer_text") or "",
        "citations": [
            {"marker": c["position"], "file": c["file_name"], "passage": c.get("content", "")}
            for c in row.get("citations") or []
        ],
    }


def review(answer_id: str) -> tuple[dict, list]:
    """Load an answer with the rest of its RFP and run every quality check on it."""
    row = call("GET", f"/answers/{answer_id}")
    siblings = call("GET", f"/projects/{row['project_id']}/answers") if row.get("project_id") else []
    others = [a["answer_text"] for a in siblings or []
              if a["id"] != answer_id and a.get("answer_text") and a["status"] in ("done", "approved")]
    draft = Draft(question=row["question"], text=row.get("answer_text") or "",
                  citations=row.get("citations") or [], other_answers=others)
    return row, run_checks(draft)


def report(results: list) -> dict:
    failing = blockers(results)
    return {
        "ready_for_approval": not failing,
        "blockers": [r.to_dict() for r in failing],
        "warnings": [r.to_dict() for r in results if not r.passed and r.severity == WARNING],
        "passed": [r.name for r in results if r.passed],
    }


# --- tools --------------------------------------------------------------------

@server.tool(annotations=READ_ONLY)
def list_projects() -> list[dict]:
    """List the user's workspaces (knowledge bases) and the RFP projects in each."""
    return [
        {
            "workspace_id": ws["id"],
            "workspace": ws["name"],
            "projects": [{"project_id": p["id"], "name": p["name"]}
                         for p in call("GET", f"/workspaces/{ws['id']}/projects") or []],
        }
        for ws in call("GET", "/workspaces") or []
    ]


@server.tool(annotations=READ_ONLY)
def get_rfp_questions(project_id: str) -> list[dict]:
    """Every question in an RFP project, in document order, with its current answer,
    status (new, pending, done, approved or failed) and cited sources."""
    return [answer_view(a) for a in call("GET", f"/projects/{project_id}/answers") or []]


@server.tool(annotations=READ_ONLY)
def search_documents(project_id: str, query: str, limit: int = 5) -> list[dict]:
    """Semantic search over the documentation available to a project (its workspace
    knowledge base plus the project's own supporting files). Use it to find facts
    for an answer or to check a claim."""
    project = call("GET", f"/projects/{project_id}")  # authorizes access before touching the DB
    return [
        {"file": c["file_name"], "chunk": c["chunk_index"], "passage": c["content"]}
        for c in search(project["workspace_id"], query, project_id=project_id, limit=max(1, min(limit, 20)))
    ]


@server.tool(annotations=WRITES)
def draft_answer(answer_id: str) -> dict:
    """Have the platform draft (or redraft) an answer from the knowledge base. Waits for
    the draft and returns it with its citations. A draft with no citations means the
    documentation doesn't cover the question; a person needs to answer it."""
    call("POST", f"/answers/{answer_id}/regenerate")
    try:
        row = wait_for(lambda: call("GET", f"/answers/{answer_id}"),
                       lambda a: a["status"] != "pending", timeout=120)
    except TimeoutError as e:
        raise ToolError("Drafting is taking longer than expected; check again with get_rfp_questions.") from e
    if row["status"] == "failed":
        raise ToolError("Drafting failed. Check that the workers are running and the OpenAI key is set.")
    return answer_view(row)


@server.tool(annotations=WRITES)
def edit_answer(answer_id: str, text: str) -> dict:
    """Replace an answer's text. Keep inline [n] markers pointing at the existing
    citations. Editing an approved answer sends it back for review."""
    return answer_view(call("PATCH", f"/answers/{answer_id}", {"answer_text": text}))


@server.tool(annotations=READ_ONLY)
def run_quality_checks(answer_id: str) -> dict:
    """Run the 13 pre-approval checks on an answer: citations present and valid, every
    figure found in a source, no placeholders or AI phrasing, length, plus LLM review
    of grounding, completeness, tone and consistency with the RFP's other answers.
    Blockers must be fixed before approval; warnings are advisory."""
    _, results = review(answer_id)
    return report(results)


@server.tool(annotations=WRITES)
def approve_answer(answer_id: str) -> dict:
    """Sign off an answer. Runs the quality checks first and refuses if any blocker
    fails, returning the report so the problems can be fixed."""
    row, results = review(answer_id)
    result = report(results)
    if not result["ready_for_approval"]:
        return {"approved": False, **result}
    approved = call("POST", f"/answers/{answer_id}/approve")
    return {"approved": True, **answer_view({**row, "status": approved["status"]})}


if __name__ == "__main__":
    server.run("stdio")
