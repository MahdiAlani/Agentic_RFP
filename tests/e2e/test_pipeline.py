"""Full RAG pipeline: upload -> embed -> extract questions -> draft answers -> chat.

Needs the Python workers running and a real OPENAI_API_KEY, and costs a few
cents per run, so it only runs with RUN_LLM_E2E=1.
"""

import os

import pytest

from conftest import wait_for

pytestmark = pytest.mark.skipif(os.getenv("RUN_LLM_E2E") != "1", reason="set RUN_LLM_E2E=1 to run")

KNOWLEDGE = """Acme Vault Security Overview

Acme Vault encrypts all customer data at rest using AES-256 and in transit using TLS 1.3.

Acme Vault is SOC 2 Type II certified; the most recent audit was completed in March 2026.

Backups are taken every 6 hours and retained for 35 days in a separate AWS region (us-west-2).

Single sign-on is supported via SAML 2.0 and OIDC, including Okta and Azure AD."""

RFP = """Request for Proposal - Data Platform

Section 3: Security

3.1 Describe how customer data is encrypted at rest and in transit.

3.2 What is your backup frequency and retention period?

3.3 Describe your on-site physical datacenter guard staffing levels."""


def test_rfp_is_answered_from_the_knowledge_base(client):
    ws = client.workspace()
    kb = client.upload(ws["id"], "acme_security.txt", KNOWLEDGE)

    doc = wait_for(lambda: (d := client.get(f"/documents/{kb['id']}").json())["status"] in ("embedded", "failed") and d)
    assert doc and doc["status"] == "embedded", doc

    proj = client.project(ws["id"])
    rfp = client.upload(ws["id"], "rfp.txt", RFP, document_type="rfp", project_id=proj["id"])
    r = client.post(f"/projects/{proj['id']}/answers/extract", {"workspace_id": ws["id"], "document_id": rfp["id"]})
    assert r.status == 202, r.text

    questions = wait_for(lambda: len(a := client.get(f"/projects/{proj['id']}/answers").json() or []) >= 3 and a)
    assert questions, "no questions extracted"
    assert [q["position"] for q in questions] == sorted(q["position"] for q in questions)

    assert client.post(f"/projects/{proj['id']}/answers/fill").json()["queued"] == len(questions)

    def finished():
        a = client.get(f"/projects/{proj['id']}/answers").json()
        return all(x["status"] in ("done", "failed") for x in a) and a

    answers = wait_for(finished, timeout=180)
    assert answers and all(a["status"] == "done" for a in answers), answers

    by_q = {a["question"][:3]: a for a in answers}
    assert "AES-256" in by_q["3.1"]["answer_text"]
    assert "6 hours" in by_q["3.2"]["answer_text"] and "35 days" in by_q["3.2"]["answer_text"]
    for q in ("3.1", "3.2"):
        cites = by_q[q].get("citations") or []
        assert {c["file_name"] for c in cites} == {"acme_security.txt"}, "RFP itself must not be cited"
        # Citations are numbered to match the [n] markers in the text.
        assert [c["position"] for c in cites] == list(range(1, len(cites) + 1))
        assert all(f"[{c['position']}" in by_q[q]["answer_text"] for c in cites)
    # The knowledge base says nothing about guard staffing: no sources, so it needs a person.
    assert not by_q["3.3"].get("citations")

    # A second fill is a no-op: finished answers are not re-billed.
    assert client.post(f"/projects/{proj['id']}/answers/fill").json()["queued"] == 0


def test_chat_reply_is_grounded(client):
    ws = client.workspace()
    kb = client.upload(ws["id"], "acme_security.txt", KNOWLEDGE)
    assert wait_for(lambda: client.get(f"/documents/{kb['id']}").json()["status"] == "embedded")

    proj = client.project(ws["id"])
    conv = client.post(f"/projects/{proj['id']}/conversations", {}).json()
    client.post(f"/conversations/{conv['id']}/messages", {"content": "Which SSO providers do we support?"})

    def reply():
        msgs = client.get(f"/conversations/{conv['id']}/messages").json()
        last = msgs[-1]
        return last["role"] == "assistant" and last["status"] != "pending" and last

    msg = wait_for(reply)
    assert msg and msg["status"] == "complete", msg
    assert "Okta" in msg["content"]
