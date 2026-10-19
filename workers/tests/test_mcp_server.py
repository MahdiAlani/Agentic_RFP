"""MCP tool behaviour with the API and the quality checks faked."""

import asyncio

import pytest
from mcp import Client

import mcp_server
from src.api_client import ApiError
from src.quality import BLOCKER, WARNING, CheckResult

ANSWER = {
    "id": "a1", "project_id": "p1", "question": "Uptime?", "status": "done",
    "answer_text": "99.95% [1].",
    "citations": [{"position": 1, "file_name": "sla.md", "content": "99.95% uptime"}],
}
SIBLING = {"id": "a2", "project_id": "p1", "question": "Backups?", "status": "approved", "answer_text": "Hourly [1]."}


class FakeApi:
    """Answers API calls from a route table and records every call."""

    def __init__(self, routes: dict):
        self.routes = routes
        self.calls = []

    def request(self, method, path, body=None):
        self.calls.append((method, path, body))
        result = self.routes.get((method, path))
        if isinstance(result, Exception):
            raise result
        if result is None:
            raise ApiError(404, "not found")
        return result() if callable(result) else result


@pytest.fixture
def fake_api(monkeypatch):
    def install(routes):
        api = FakeApi(routes)
        monkeypatch.setattr(mcp_server, "api", lambda: api)
        return api
    return install


def checks(*failing: tuple[str, str]):
    """Stub run_checks: everything passes except the given (name, severity) pairs."""
    names = dict(failing)
    return lambda draft: [
        CheckResult(n, n not in names, names.get(n, BLOCKER), "bad" if n in names else "")
        for n in ("has_citations", "grounded", "no_hedging")
    ]


def call(name, **args):
    async def go():
        async with Client(mcp_server.server) as c:
            return await c.call_tool(name, args)
    return asyncio.run(go())


def test_exposes_seven_tools():
    async def names():
        async with Client(mcp_server.server) as c:
            return {t.name for t in (await c.list_tools()).tools}
    assert asyncio.run(names()) == {
        "list_projects", "get_rfp_questions", "search_documents", "draft_answer",
        "edit_answer", "run_quality_checks", "approve_answer",
    }


def test_list_projects_nests_projects_under_workspaces(fake_api):
    fake_api({
        ("GET", "/workspaces"): [{"id": "w1", "name": "Corvane"}],
        ("GET", "/workspaces/w1/projects"): [{"id": "p1", "name": "Ashford RFP"}],
    })
    assert mcp_server.list_projects() == [
        {"workspace_id": "w1", "workspace": "Corvane", "projects": [{"project_id": "p1", "name": "Ashford RFP"}]},
    ]


def test_answer_view_shape():
    view = mcp_server.answer_view(ANSWER)
    assert view["answer_id"] == "a1"
    assert view["citations"] == [{"marker": 1, "file": "sla.md", "passage": "99.95% uptime"}]


def test_search_checks_access_through_the_api_first(fake_api, monkeypatch):
    api = fake_api({})  # project lookup 404s
    monkeypatch.setattr(mcp_server, "search", lambda *a, **k: pytest.fail("searched without access"))
    result = call("search_documents", project_id="p-other", query="uptime")
    assert result.is_error and "Not found" in result.content[0].text
    assert api.calls == [("GET", "/projects/p-other", None)]


def test_search_is_scoped_to_the_projects_workspace(fake_api, monkeypatch):
    fake_api({("GET", "/projects/p1"): {"id": "p1", "workspace_id": "w1"}})
    seen = {}

    def fake_search(workspace_id, query, project_id, limit):
        seen.update(workspace_id=workspace_id, project_id=project_id, limit=limit)
        return [{"file_name": "sla.md", "chunk_index": 0, "content": "99.95%"}]

    monkeypatch.setattr(mcp_server, "search", fake_search)
    hits = mcp_server.search_documents("p1", "uptime", limit=500)
    assert hits == [{"file": "sla.md", "chunk": 0, "passage": "99.95%"}]
    assert seen == {"workspace_id": "w1", "project_id": "p1", "limit": 20}  # limit is capped


def test_draft_answer_waits_for_the_worker(fake_api, monkeypatch):
    monkeypatch.setattr("src.api_client.time.sleep", lambda s: None)
    states = iter([{**ANSWER, "status": "pending"}, ANSWER])
    api = fake_api({
        ("POST", "/answers/a1/regenerate"): {**ANSWER, "status": "pending"},
        ("GET", "/answers/a1"): lambda: next(states),
    })
    assert mcp_server.draft_answer("a1")["status"] == "done"
    assert api.calls[0] == ("POST", "/answers/a1/regenerate", None)


def test_draft_answer_reports_worker_failure(fake_api):
    fake_api({
        ("POST", "/answers/a1/regenerate"): ANSWER,
        ("GET", "/answers/a1"): {**ANSWER, "status": "failed"},
    })
    result = call("draft_answer", answer_id="a1")
    assert result.is_error and "Drafting failed" in result.content[0].text


def test_quality_report_passes_sibling_answers_for_consistency(fake_api, monkeypatch):
    fake_api({("GET", "/answers/a1"): ANSWER, ("GET", "/projects/p1/answers"): [ANSWER, SIBLING]})
    seen = {}
    monkeypatch.setattr(mcp_server, "run_checks", lambda d: seen.setdefault("draft", d) and checks()(d))
    report = mcp_server.run_quality_checks("a1")
    assert report["ready_for_approval"] is True
    assert seen["draft"].other_answers == ["Hourly [1]."]  # itself excluded
    assert seen["draft"].citations == ANSWER["citations"]


def test_approve_refuses_while_a_blocker_fails(fake_api, monkeypatch):
    api = fake_api({("GET", "/answers/a1"): ANSWER, ("GET", "/projects/p1/answers"): [ANSWER]})
    monkeypatch.setattr(mcp_server, "run_checks", checks(("grounded", BLOCKER), ("no_hedging", WARNING)))
    result = mcp_server.approve_answer("a1")
    assert result["approved"] is False
    assert [b["name"] for b in result["blockers"]] == ["grounded"]
    assert [w["name"] for w in result["warnings"]] == ["no_hedging"]
    assert ("POST", "/answers/a1/approve", None) not in api.calls


def test_approve_signs_off_when_only_warnings_fail(fake_api, monkeypatch):
    api = fake_api({
        ("GET", "/answers/a1"): ANSWER,
        ("GET", "/projects/p1/answers"): [ANSWER],
        ("POST", "/answers/a1/approve"): {**ANSWER, "status": "approved", "citations": None},
    })
    monkeypatch.setattr(mcp_server, "run_checks", checks(("no_hedging", WARNING)))
    result = mcp_server.approve_answer("a1")
    assert result["approved"] is True and result["status"] == "approved"
    assert result["citations"], "citations from the reviewed answer are kept"
    assert ("POST", "/answers/a1/approve", None) in api.calls


def test_api_errors_become_readable_tool_errors(fake_api):
    fake_api({("PATCH", "/answers/a1"): ApiError(400, "invalid input: answer_text is required")})
    result = call("edit_answer", answer_id="a1", text="")
    assert result.is_error and "answer_text is required" in result.content[0].text
