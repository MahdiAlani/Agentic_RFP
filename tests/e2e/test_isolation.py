"""One user must never see or change another user's data.

KNOWN GAP: only workspace list/create are scoped to the caller today. Every
by-ID route trusts the ID, so any logged-in user can read, edit or delete
another user's workspaces, projects, documents, answers, chats, and accounts.

These tests are strict xfails: they document the hole, and the suite fails
(XPASS) the moment a fix lands, as a reminder to drop the marker.
"""

import pytest

KNOWN_GAP = pytest.mark.xfail(strict=True, reason="no ownership checks on by-ID routes yet")


@pytest.fixture
def victim(client):
    """A workspace with a project, document, answer and conversation owned by `client`."""
    ws = client.workspace("victim")
    proj = client.project(ws["id"])
    doc = client.upload(ws["id"], "secret.txt", "confidential pricing", confirm=False)
    ans = client.post("/answers", {"workspace_id": ws["id"], "project_id": proj["id"], "question": "Q?"}).json()
    conv = client.post(f"/projects/{proj['id']}/conversations", {}).json()
    return {"ws": ws["id"], "proj": proj["id"], "doc": doc["id"], "ans": ans["id"], "conv": conv["id"]}


READS = [
    "/workspaces/{ws}",
    "/workspaces/{ws}/projects",
    "/workspaces/{ws}/documents",
    "/projects/{proj}",
    "/projects/{proj}/documents",
    "/projects/{proj}/answers",
    "/projects/{proj}/conversations",
    "/documents/{doc}",
    "/documents/{doc}/download",
    "/answers/{ans}",
    "/conversations/{conv}/messages",
]


@KNOWN_GAP
@pytest.mark.parametrize("template", READS)
def test_cannot_read_other_users_data(victim, other_client, template):
    r = other_client.get(template.format(**victim))
    assert r.status in (403, 404), f"{template} -> {r.status}"


@KNOWN_GAP
@pytest.mark.parametrize("template", ["/documents/{doc}", "/projects/{proj}", "/workspaces/{ws}"])
def test_cannot_delete_other_users_data(victim, client, other_client, template):
    path = template.format(**victim)
    other_client.delete(path)
    assert client.get(path).status == 200, f"{path} was deleted by another user"


@KNOWN_GAP
def test_cannot_delete_another_account(client, other_client):
    other_client.delete(f"/users/{client.user_id}")
    assert client.get("/auth/me").status == 200
