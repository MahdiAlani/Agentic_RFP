"""Load the Corvane demo into a running stack.

Creates a demo user, a workspace holding the Corvane knowledge base, and a
project for the City of Ashford RFP; then extracts the RFP's questions and
drafts every answer. Re-running replaces the demo workspace.

Run:  docker compose run --rm seed          (full Docker stack)
      cd workers && python seed.py         (API on localhost:8080)
"""

import logging
import os
import sys
from pathlib import Path

from src.api_client import ApiClient, wait_for

API_URL = os.getenv("API_URL", "http://localhost:8080")
DEMO_DIR = Path(os.getenv("DEMO_DIR", Path(__file__).resolve().parent.parent / "demo"))
DEMO_EMAIL = os.getenv("DEMO_EMAIL", "demo@corvane.example")
DEMO_PASSWORD = os.getenv("DEMO_PASSWORD", "")

WORKSPACE_NAME = "Corvane"
PROJECT_NAME = "City of Ashford - Data Analytics RFP"

logging.basicConfig(level=logging.INFO, format="%(message)s")
log = logging.getLogger("seed")


def main() -> None:
    if len(DEMO_PASSWORD) < 8:
        sys.exit("Set DEMO_PASSWORD (8+ characters) in .env; the seed signs in with it.")

    api = ApiClient(API_URL)
    api.login_or_register(DEMO_EMAIL, "Demo User", DEMO_PASSWORD)

    for ws in api.get("/workspaces") or []:
        if ws["name"] == WORKSPACE_NAME:
            log.info("replacing existing %r workspace", WORKSPACE_NAME)
            api.delete(f"/workspaces/{ws['id']}")
    workspace = api.post("/workspaces", {"name": WORKSPACE_NAME})

    docs = [
        api.upload_document(workspace["id"], path)
        for path in sorted((DEMO_DIR / "knowledge-base").glob("*.md"))
    ]
    log.info("uploaded %d knowledge base documents, waiting for embedding...", len(docs))
    for doc in docs:
        result = wait_for(lambda: api.get(f"/documents/{doc['id']}"),
                          lambda d: d["status"] in ("embedded", "failed"))
        if result["status"] == "failed":
            sys.exit(f"embedding failed for {doc['file_name']}; check the worker logs")

    project = api.post("/projects", {"workspace_id": workspace["id"], "name": PROJECT_NAME})
    rfp_path = next((DEMO_DIR / "rfp").glob("*.md"))
    rfp = api.upload_document(workspace["id"], rfp_path, document_type="rfp", project_id=project["id"])

    api.post(f"/projects/{project['id']}/answers/extract",
             {"workspace_id": workspace["id"], "document_id": rfp["id"]})
    log.info("extracting questions from %s...", rfp_path.name)
    questions = wait_for(lambda: api.get(f"/projects/{project['id']}/answers") or [], bool)
    # Extraction inserts every row in one transaction, so a non-empty list is complete.
    log.info("found %d questions, drafting answers...", len(questions))

    api.post(f"/projects/{project['id']}/answers/fill")
    answers = wait_for(
        lambda: api.get(f"/projects/{project['id']}/answers"),
        lambda rows: all(a["status"] not in ("new", "pending") for a in rows),
        timeout=600,
    )

    failed = [a for a in answers if a["status"] == "failed"]
    log.info("drafted %d of %d answers", len(answers) - len(failed), len(answers))
    log.info("\nSign in at http://localhost:5173 as %s with DEMO_PASSWORD from .env", DEMO_EMAIL)


if __name__ == "__main__":
    main()
