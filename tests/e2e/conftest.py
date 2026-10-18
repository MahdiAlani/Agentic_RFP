"""End-to-end tests against a running stack (API + Postgres + Redis + MinIO).

Start the stack (./start.ps1, or see README), then from the repo root:

    pytest tests/e2e                    # API tests; skipped if the API is down
    RUN_LLM_E2E=1 pytest tests/e2e      # also run the worker + OpenAI pipeline

Set E2E_REQUIRED=1 (as CI does) to fail instead of skip when the API is down.
"""

import json
import os
import secrets
import time
import urllib.error
import urllib.request

import pytest

API_URL = os.getenv("API_URL", "http://localhost:8080").rstrip("/")


class Response:
    def __init__(self, status: int, body: str):
        self.status = status
        self.text = body

    def json(self):
        return json.loads(self.text)


def request(method, url, *, json_body=None, data=None, headers=None, timeout=30) -> Response:
    headers = dict(headers or {})
    if json_body is not None:
        data = json.dumps(json_body).encode()
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return Response(r.status, r.read().decode())
    except urllib.error.HTTPError as e:
        return Response(e.code, e.read().decode())


class Client:
    """A registered user talking to the API with their own token."""

    def __init__(self):
        self.email = f"e2e-{secrets.token_hex(6)}@example.test"
        self.password = secrets.token_urlsafe(16)
        r = self.post("/auth/register", {"email": self.email, "name": "E2E", "password": self.password})
        assert r.status == 201, r.text
        self.token = r.json()["token"]
        self.user_id = r.json()["user"]["id"]

    def _headers(self):
        return {"Authorization": f"Bearer {self.token}"} if getattr(self, "token", None) else {}

    def get(self, path):
        return request("GET", API_URL + path, headers=self._headers())

    def post(self, path, body=None):
        return request("POST", API_URL + path, json_body=body if body is not None else {}, headers=self._headers())

    def patch(self, path, body):
        return request("PATCH", API_URL + path, json_body=body, headers=self._headers())

    def delete(self, path):
        return request("DELETE", API_URL + path, headers=self._headers())

    # --- helpers ---

    def workspace(self, name="E2E workspace"):
        r = self.post("/workspaces", {"name": name})
        assert r.status == 201, r.text
        return r.json()

    def project(self, workspace_id, name="E2E RFP"):
        r = self.post("/projects", {"workspace_id": workspace_id, "name": name})
        assert r.status == 201, r.text
        return r.json()

    def upload(self, workspace_id, file_name, content: str, document_type="other", project_id=None, confirm=True):
        r = self.post("/documents", {
            "workspace_id": workspace_id,
            "project_id": project_id,
            "file_name": file_name,
            "document_type": document_type,
        })
        assert r.status == 201, r.text
        doc = r.json()
        put = request("PUT", doc["upload_url"], data=content.encode(), headers={"Content-Type": "text/plain"})
        assert put.status == 200, put.text
        if confirm:
            r = self.post(f"/documents/{doc['id']}/confirm")
            assert r.status == 200, r.text
        return doc


def wait_for(fn, timeout=120, interval=2):
    """Poll fn until it returns something truthy; returns that value or None."""
    deadline = time.time() + timeout
    while time.time() < deadline:
        value = fn()
        if value:
            return value
        time.sleep(interval)
    return None


@pytest.fixture(scope="session", autouse=True)
def _api_up():
    try:
        urllib.request.urlopen(API_URL + "/auth/me", timeout=3)
    except urllib.error.HTTPError:
        return  # any HTTP answer (401) means the API is up
    except OSError as e:
        msg = f"API not reachable at {API_URL}: {e}"
        if os.getenv("E2E_REQUIRED") == "1":
            pytest.fail(msg)
        pytest.skip(msg)


@pytest.fixture
def client():
    c = Client()
    yield c
    # Best-effort cleanup so repeated runs don't pile up rows and objects.
    for ws in (c.get("/workspaces").json() or []):
        c.delete(f"/workspaces/{ws['id']}")


@pytest.fixture
def other_client():
    return Client()
