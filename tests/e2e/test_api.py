"""API behaviour that needs Postgres, Redis and MinIO but no workers or LLM."""

import pytest

from conftest import API_URL, Client, request


class TestAuth:
    def test_register_login_me(self, client):
        r = client.post("/auth/login", {"email": client.email, "password": client.password})
        assert r.status == 200
        me = client.get("/auth/me")
        assert me.status == 200
        assert me.json()["email"] == client.email
        assert "password" not in me.text.lower()

    def test_duplicate_email_is_conflict(self, client):
        r = request("POST", API_URL + "/auth/register",
                    json_body={"email": client.email, "name": "Dup", "password": "password123"})
        assert r.status == 409

    def test_wrong_password_and_unknown_email_look_the_same(self, client):
        wrong = request("POST", API_URL + "/auth/login",
                        json_body={"email": client.email, "password": "wrong-password"})
        unknown = request("POST", API_URL + "/auth/login",
                          json_body={"email": "nobody@example.test", "password": "wrong-password"})
        assert wrong.status == unknown.status == 401
        assert wrong.text == unknown.text

    @pytest.mark.parametrize("body", [
        {"email": "bad", "name": "X", "password": "password123"},
        {"email": "ok@example.test", "name": "X", "password": "short"},
        {"email": "ok@example.test", "name": "", "password": "password123"},
    ])
    def test_register_validation(self, body):
        assert request("POST", API_URL + "/auth/register", json_body=body).status == 400

    @pytest.mark.parametrize("path", ["/workspaces", "/auth/me", "/projects/00000000-0000-0000-0000-000000000000"])
    def test_protected_routes_require_token(self, path):
        assert request("GET", API_URL + path).status == 401
        assert request("GET", API_URL + path, headers={"Authorization": "Bearer forged"}).status == 401


class TestWorkspacesAndProjects:
    def test_crud(self, client):
        ws = client.workspace("Acme")
        assert [w["id"] for w in client.get("/workspaces").json()] == [ws["id"]]

        r = client.patch(f"/workspaces/{ws['id']}", {"name": "Acme Corp"})
        assert r.status == 200 and r.json()["name"] == "Acme Corp"

        proj = client.project(ws["id"], "Q3 RFP")
        listed = client.get(f"/workspaces/{ws['id']}/projects").json()
        assert [p["id"] for p in listed] == [proj["id"]]

        assert client.delete(f"/projects/{proj['id']}").status == 204
        assert client.get(f"/projects/{proj['id']}").status == 404
        assert client.delete(f"/workspaces/{ws['id']}").status == 204
        assert client.get(f"/workspaces/{ws['id']}").status == 404

    def test_project_in_missing_workspace_is_rejected(self, client):
        r = client.post("/projects", {"workspace_id": "00000000-0000-0000-0000-000000000001", "name": "x"})
        assert 400 <= r.status < 500

    def test_workspace_list_only_shows_own(self, client, other_client):
        client.workspace("mine")
        assert other_client.get("/workspaces").json() in ([], None)


class TestDocuments:
    def test_presigned_upload_confirm_and_download(self, client):
        ws = client.workspace()
        doc = client.upload(ws["id"], "security.txt", "AES-256 everywhere.")

        got = client.get(f"/documents/{doc['id']}").json()
        assert got["status"] in ("uploaded", "embedded", "failed")  # a worker may already have it

        listed = client.get(f"/workspaces/{ws['id']}/documents").json()
        assert [d["id"] for d in listed] == [doc["id"]]

        dl = client.get(f"/documents/{doc['id']}/download")
        assert dl.status == 200
        assert request("GET", dl.json()["download_url"]).text == "AES-256 everywhere."

    def test_confirm_before_upload_is_conflict(self, client):
        ws = client.workspace()
        r = client.post("/documents", {"workspace_id": ws["id"], "file_name": "a.txt", "document_type": "other"})
        assert client.post(f"/documents/{r.json()['id']}/confirm").status == 409

    def test_project_documents_stay_out_of_workspace_knowledge_base(self, client):
        ws = client.workspace()
        proj = client.project(ws["id"])
        kb = client.upload(ws["id"], "kb.txt", "kb", confirm=False)
        rfp = client.upload(ws["id"], "rfp.txt", "rfp", document_type="rfp", project_id=proj["id"], confirm=False)

        assert [d["id"] for d in client.get(f"/workspaces/{ws['id']}/documents").json()] == [kb["id"]]
        assert [d["id"] for d in client.get(f"/projects/{proj['id']}/documents").json()] == [rfp["id"]]

    def test_rename_and_delete(self, client):
        ws = client.workspace()
        doc = client.upload(ws["id"], "old.txt", "x", confirm=False)
        assert client.patch(f"/documents/{doc['id']}", {"file_name": "new.txt"}).json()["file_name"] == "new.txt"
        assert client.delete(f"/documents/{doc['id']}").status == 204
        assert client.get(f"/documents/{doc['id']}").status == 404

    @pytest.mark.parametrize("missing", ["workspace_id", "file_name", "document_type"])
    def test_create_validation(self, client, missing):
        ws = client.workspace()
        body = {"workspace_id": ws["id"], "file_name": "a.txt", "document_type": "other"}
        body.pop(missing)
        assert client.post("/documents", body).status == 400


class TestChatAndAnswers:
    def test_send_message_queues_pending_reply(self, client):
        ws = client.workspace()
        proj = client.project(ws["id"])
        conv = client.post(f"/projects/{proj['id']}/conversations", {}).json()
        assert conv["title"] == "New chat"

        r = client.post(f"/conversations/{conv['id']}/messages", {"content": "Hello?"})
        assert r.status == 201
        body = r.json()
        assert body["user"]["content"] == "Hello?"
        assert body["assistant"]["role"] == "assistant"
        assert body["assistant"]["status"] in ("pending", "complete", "failed")

        roles = [m["role"] for m in client.get(f"/conversations/{conv['id']}/messages").json()]
        assert roles == ["user", "assistant"]

    def test_empty_message_rejected(self, client):
        ws = client.workspace()
        proj = client.project(ws["id"])
        conv = client.post(f"/projects/{proj['id']}/conversations", {}).json()
        assert client.post(f"/conversations/{conv['id']}/messages", {"content": "  "}).status == 400

    def test_manual_answer_edit(self, client):
        ws = client.workspace()
        proj = client.project(ws["id"])
        a = client.post("/answers", {"workspace_id": ws["id"], "project_id": proj["id"], "question": "Q?"})
        assert a.status == 201
        r = client.patch(f"/answers/{a.json()['id']}", {"answer_text": "Hand-written answer."})
        assert r.status == 200
        got = client.get(f"/answers/{a.json()['id']}").json()
        assert got["answer_text"] == "Hand-written answer." and got["status"] == "done"

    def test_approval_flow(self, client):
        ws = client.workspace()
        proj = client.project(ws["id"])
        aid = client.post("/answers", {"workspace_id": ws["id"], "project_id": proj["id"], "question": "Q?"}).json()["id"]

        client.patch(f"/answers/{aid}", {"answer_text": "Final answer."})
        r = client.post(f"/answers/{aid}/approve")
        assert r.status == 200 and r.json()["status"] == "approved"
        assert client.post(f"/answers/{aid}/approve").status == 400  # already approved

        # Editing an approved answer sends it back for review.
        client.patch(f"/answers/{aid}", {"answer_text": "Changed after sign-off."})
        assert client.get(f"/answers/{aid}").json()["status"] == "done"

    def test_deleting_workspace_cascades(self, client):
        ws = client.workspace()
        proj = client.project(ws["id"])
        doc = client.upload(ws["id"], "a.txt", "x", confirm=False)
        conv = client.post(f"/projects/{proj['id']}/conversations", {}).json()

        assert client.delete(f"/workspaces/{ws['id']}").status == 204
        for path in (f"/projects/{proj['id']}", f"/documents/{doc['id']}", f"/conversations/{conv['id']}"):
            assert client.get(path).status == 404, path
