"""One user must never see or change another user's data.

Another user's resources answer 404, as if they didn't exist.
"""

import pytest


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


@pytest.mark.parametrize("template", READS)
def test_cannot_read_other_users_data(victim, other_client, template):
    r = other_client.get(template.format(**victim))
    assert r.status == 404, f"{template} -> {r.status}"


@pytest.mark.parametrize("template", ["/documents/{doc}", "/projects/{proj}", "/workspaces/{ws}"])
def test_cannot_delete_other_users_data(victim, client, other_client, template):
    path = template.format(**victim)
    other_client.delete(path)
    assert client.get(path).status == 200, f"{path} was deleted by another user"


@pytest.mark.parametrize("template", ["/projects/{proj}", "/workspaces/{ws}"])
def test_cannot_rename_other_users_data(victim, client, other_client, template):
    path = template.format(**victim)
    assert other_client.patch(path, {"name": "hijacked"}).status == 404
    assert client.get(path).json()["name"] != "hijacked"


def test_cannot_create_in_other_users_workspace(victim, other_client):
    r = other_client.post("/projects", {"workspace_id": victim["ws"], "name": "intruder"})
    assert r.status == 404, r.text
    r = other_client.post("/answers", {"workspace_id": victim["ws"], "question": "Q?"})
    assert r.status == 404, r.text


def test_owner_still_has_access(victim, client):
    for template in READS:
        path = template.format(**victim)
        assert client.get(path).status == 200, path


def test_cannot_delete_another_account(client, other_client):
    other_client.delete(f"/users/{client.user_id}")
    assert client.get("/auth/me").status == 200
