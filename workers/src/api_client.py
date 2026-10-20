"""Small client for the Go REST API.

Used by tools that act on behalf of a user rather than reading Postgres
directly: the demo seeder, the eval harness and the MCP server. Going through
the API keeps authentication and authorization in one place.
"""

import os
import time
from pathlib import Path
from typing import Callable, TypeVar
from urllib.parse import urlsplit, urlunsplit

import httpx

T = TypeVar("T")


class ApiError(Exception):
    def __init__(self, status: int, message: str):
        super().__init__(f"{status}: {message}")
        self.status = status
        self.message = message


class ApiClient:
    def __init__(self, base_url: str, token: str | None = None, timeout: float = 30.0):
        self.base_url = base_url.rstrip("/")
        self.token = token
        self._http = httpx.Client(timeout=timeout)

    # --- auth ---

    def login(self, email: str, password: str) -> dict:
        resp = self.request("POST", "/auth/login", {"email": email, "password": password})
        self.token = resp["token"]
        return resp["user"]

    def register(self, email: str, name: str, password: str) -> dict:
        resp = self.request("POST", "/auth/register", {"email": email, "name": name, "password": password})
        self.token = resp["token"]
        return resp["user"]

    def login_or_register(self, email: str, name: str, password: str) -> dict:
        try:
            return self.login(email, password)
        except ApiError as e:
            if e.status != 401:
                raise
            return self.register(email, name, password)

    # --- transport ---

    def request(self, method: str, path: str, body: dict | None = None):
        headers = {"Authorization": f"Bearer {self.token}"} if self.token else {}
        resp = self._http.request(method, self.base_url + path, json=body, headers=headers)
        if resp.status_code >= 400:
            raise ApiError(resp.status_code, resp.text.strip())
        return resp.json() if resp.content else None

    def get(self, path: str):
        return self.request("GET", path)

    def post(self, path: str, body: dict | None = None):
        return self.request("POST", path, body if body is not None else {})

    def patch(self, path: str, body: dict):
        return self.request("PATCH", path, body)

    def delete(self, path: str):
        return self.request("DELETE", path)

    def download(self, path: str) -> bytes:
        headers = {"Authorization": f"Bearer {self.token}"} if self.token else {}
        resp = self._http.get(self.base_url + path, headers=headers)
        if resp.status_code >= 400:
            raise ApiError(resp.status_code, resp.text.strip())
        return resp.content

    # --- documents ---

    def upload_document(
        self, workspace_id: str, path: Path, document_type: str = "other", project_id: str | None = None
    ) -> dict:
        """Run the presigned upload flow: create row, PUT bytes, confirm."""
        doc = self.post("/documents", {
            "workspace_id": workspace_id,
            "project_id": project_id,
            "file_name": path.name,
            "document_type": document_type,
        })
        url, headers = _storage_target(doc["upload_url"])
        put = self._http.put(url, content=path.read_bytes(), headers=headers)
        if put.status_code >= 400:
            raise ApiError(put.status_code, f"upload of {path.name} failed: {put.text.strip()}")
        return self.post(f"/documents/{doc['id']}/confirm")


def _storage_target(presigned_url: str) -> tuple[str, dict]:
    """Where to send a presigned request, and with which Host header.

    Presigned URLs are signed for the host browsers use (localhost:9000). Inside
    the compose network that host is unreachable, so STORAGE_INTERNAL_URL (e.g.
    http://minio:9000) redirects the connection while the original Host header
    is kept - the signature covers the Host header, not the address dialled.
    """
    internal = os.getenv("STORAGE_INTERNAL_URL")
    if not internal:
        return presigned_url, {}
    signed, target = urlsplit(presigned_url), urlsplit(internal)
    return urlunsplit(signed._replace(scheme=target.scheme, netloc=target.netloc)), {"Host": signed.netloc}


def wait_for(fetch: Callable[[], T], done: Callable[[T], bool], timeout: float = 180, interval: float = 2) -> T:
    """Poll `fetch` until `done(result)` holds; raises TimeoutError otherwise."""
    deadline = time.monotonic() + timeout
    while True:
        result = fetch()
        if done(result):
            return result
        if time.monotonic() >= deadline:
            raise TimeoutError(f"gave up waiting after {timeout:.0f}s")
        time.sleep(interval)
