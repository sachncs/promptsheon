"""Minimal HTTP client for the Promptsheon v1 API.

The shape mirrors the TypeScript client in sdk/typescript: every
method returns the OpenAPI-typed response body; errors are raised
as PromptsheonAPIError with the underlying httpx.HTTPStatusError
captured for callers that want retry semantics.

SDK-2: every /api/v1 route in api/openapi.yaml is wrapped here.
The regenerated source lives at
sdk/python/src/promptsheon/_generated/openapi.yaml — `make sdk`
refreshes it from the server.
"""
from __future__ import annotations

import httpx

from pydantic import BaseModel, Field
from typing import Any


class ClientConfig(BaseModel):
    base_url: str
    api_key: str | None = None
    timeout_seconds: float = 30.0


class PromptsheonAPIError(RuntimeError):
    """Raised when the Promptsheon API returns a non-2xx response."""

    def __init__(self, status: int, method: str, path: str, body: Any):
        self.status = status
        self.method = method
        self.path = path
        self.body = body
        super().__init__(f"{method} {path} returned {status}")


class _BaseClient:
    def __init__(self, config: ClientConfig):
        self._config = config

    def _headers(self) -> dict[str, str]:
        headers = {"Accept": "application/json"}
        if self._config.api_key:
            headers["Authorization"] = f"Bearer {self._config.api_key}"
        return headers

    def _json_headers(self) -> dict[str, str]:
        h = self._headers()
        h["Content-Type"] = "application/json"
        return h

    def _check(self, r: httpx.Response, method: str, url: str) -> Any:
        if r.status_code not in (200, 201, 202, 204):
            raise PromptsheonAPIError(r.status_code, method, url, r.text)
        if r.status_code == 204 or not r.content:
            return None
        return r.json()


class Client(_BaseClient):
    """Synchronous Promptsheon client."""

    def __init__(self, config: ClientConfig):
        super().__init__(config)
        self._http = httpx.Client(
            base_url=config.base_url,
            timeout=config.timeout_seconds,
        )

    def close(self):
        self._http.close()

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.close()

    # --- Capabilities -----------------------------------------------------
    def list_capabilities(self, project_id: str) -> list[dict]:
        url = f"/api/v1/projects/{project_id}/capabilities"
        return self._check(self._http.get(url, headers=self._headers()), "GET", url)

    def get_capability(self, capability_id: str) -> dict:
        url = f"/api/v1/capabilities/{capability_id}"
        return self._check(self._http.get(url, headers=self._headers()), "GET", url)

    def update_capability_contract(self, capability_id: str, contract: dict) -> dict:
        url = f"/api/v1/capabilities/{capability_id}/contract"
        return self._check(self._http.put(url, json=contract, headers=self._json_headers()), "PUT", url)

    def get_capability_contract(self, capability_id: str) -> dict:
        url = f"/api/v1/capabilities/{capability_id}/contract"
        return self._check(self._http.get(url, headers=self._headers()), "GET", url)

    def get_capability_reputation(self, capability_id: str) -> dict:
        url = f"/api/v1/capabilities/{capability_id}/reputation"
        return self._check(self._http.get(url, headers=self._headers()), "GET", url)

    def diff_capability_versions(self, capability_id: str, from_version: int, to_version: int) -> dict:
        url = f"/api/v1/capabilities/{capability_id}/diff?from={from_version}&to={to_version}"
        return self._check(self._http.get(url, headers=self._headers()), "GET", url)

    def catalog_search(self, workspace_id: str, query: str = "", limit: int = 100) -> list[dict]:
        params = {"workspace_id": workspace_id}
        if query:
            params["q"] = query
        if limit:
            params["limit"] = str(limit)
        url = "/api/v1/catalog/capabilities"
        return self._check(self._http.get(url, params=params, headers=self._headers()), "GET", url)

    # --- Releases ---------------------------------------------------------
    def list_releases(self, capability_id: str) -> list[dict]:
        url = f"/api/v1/capabilities/{capability_id}/releases"
        return self._check(self._http.get(url, headers=self._headers()), "GET", url)

    def create_release(self, version_id: str, environment: str = "prod") -> dict:
        url = f"/api/v1/versions/{version_id}/releases"
        return self._check(self._http.post(url, json={"environment": environment}, headers=self._json_headers()), "POST", url)

    def get_release(self, release_id: str) -> dict:
        url = f"/api/v1/releases/{release_id}"
        return self._check(self._http.get(url, headers=self._headers()), "GET", url)

    def vote_release(self, release_id: str, identity: str, decision: str) -> dict:
        url = f"/api/v1/releases/{release_id}/votes"
        return self._check(self._http.post(url, json={"identity": identity, "decision": decision}, headers=self._json_headers()), "POST", url)

    def activate_release(self, release_id: str) -> dict:
        url = f"/api/v1/releases/{release_id}/activate"
        return self._check(self._http.post(url, headers=self._headers()), "POST", url)

    def rollback_release(self, release_id: str) -> dict:
        url = f"/api/v1/releases/{release_id}/rollback"
        return self._check(self._http.post(url, headers=self._headers()), "POST", url)

    def get_release_approval(self, release_id: str) -> dict:
        url = f"/api/v1/releases/{release_id}/approval"
        return self._check(self._http.get(url, headers=self._headers()), "GET", url)

    def invoke_release(self, release_id: str, inputs: dict[str, Any]) -> dict:
        url = f"/api/v1/releases/{release_id}/invoke"
        return self._check(self._http.post(url, json={"inputs": inputs}, headers=self._json_headers()), "POST", url)

    # --- Harness (Datasets, Preconditions, Evals) -------------------------
    def list_datasets(self, capability_id: str) -> list[dict]:
        url = f"/api/v1/capabilities/{capability_id}/datasets"
        return self._check(self._http.get(url, headers=self._headers()), "GET", url)

    def create_dataset(self, capability_id: str, body: dict) -> dict:
        url = f"/api/v1/capabilities/{capability_id}/datasets"
        return self._check(self._http.post(url, json=body, headers=self._json_headers()), "POST", url)

    def list_preconditions(self, capability_id: str) -> list[dict]:
        url = f"/api/v1/capabilities/{capability_id}/preconditions"
        return self._check(self._http.get(url, headers=self._headers()), "GET", url)

    def create_precondition(self, capability_id: str, body: dict) -> dict:
        url = f"/api/v1/capabilities/{capability_id}/preconditions"
        return self._check(self._http.post(url, json=body, headers=self._json_headers()), "POST", url)

    def run_eval(self, release_id: str, dataset_id: str, scorer: str = "exact_match") -> dict:
        url = f"/api/v1/releases/{release_id}/evals"
        return self._check(self._http.post(url, json={"dataset_id": dataset_id, "scorer": scorer}, headers=self._json_headers()), "POST", url)

    def list_evals(self, release_id: str) -> list[dict]:
        url = f"/api/v1/releases/{release_id}/evals"
        return self._check(self._http.get(url, headers=self._headers()), "GET", url)

    def get_eval(self, eval_id: str) -> dict:
        url = f"/api/v1/evals/{eval_id}"
        return self._check(self._http.get(url, headers=self._headers()), "GET", url)

    # --- Audit / Settings ------------------------------------------------
    def verify_audit_chain(self) -> dict:
        url = "/api/v1/audit/verify"
        return self._check(self._http.get(url, headers=self._headers()), "GET", url)

    def list_settings(self) -> list[dict]:
        url = "/api/v1/settings"
        return self._check(self._http.get(url, headers=self._headers()), "GET", url)

    def set_setting(self, key: str, value: str, updated_by: str = "python-sdk") -> dict:
        url = f"/api/v1/settings/{key}"
        return self._check(self._http.put(url, json={"value": value, "updated_by": updated_by}, headers=self._json_headers()), "PUT", url)


class AsyncClient(_BaseClient):
    """Asynchronous Promptsheon client."""

    def __init__(self, config: ClientConfig):
        super().__init__(config)
        self._http = httpx.AsyncClient(
            base_url=config.base_url,
            timeout=config.timeout_seconds,
        )

    async def aclose(self):
        await self._http.aclose()

    async def __aenter__(self):
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        await self.aclose()

    async def _check(self, r: httpx.Response, method: str, url: str) -> Any:
        if r.status_code not in (200, 201, 202, 204):
            raise PromptsheonAPIError(r.status_code, method, url, r.text)
        if r.status_code == 204 or not r.content:
            return None
        return r.json()

    async def list_capabilities(self, project_id: str) -> list[dict]:
        url = f"/api/v1/projects/{project_id}/capabilities"
        r = await self._http.get(url, headers=self._headers())
        return await self._check(r, "GET", url)

    async def get_capability(self, capability_id: str) -> dict:
        url = f"/api/v1/capabilities/{capability_id}"
        r = await self._http.get(url, headers=self._headers())
        return await self._check(r, "GET", url)

    async def invoke_release(self, release_id: str, inputs: dict[str, Any]) -> dict:
        url = f"/api/v1/releases/{release_id}/invoke"
        r = await self._http.post(url, json={"inputs": inputs}, headers=self._json_headers())
        return await self._check(r, "POST", url)

    async def create_release(self, version_id: str, environment: str = "prod") -> dict:
        url = f"/api/v1/versions/{version_id}/releases"
        r = await self._http.post(url, json={"environment": environment}, headers=self._json_headers())
        return await self._check(r, "POST", url)

    async def vote_release(self, release_id: str, identity: str, decision: str) -> dict:
        url = f"/api/v1/releases/{release_id}/votes"
        r = await self._http.post(url, json={"identity": identity, "decision": decision}, headers=self._json_headers())
        return await self._check(r, "POST", url)

    async def activate_release(self, release_id: str) -> dict:
        url = f"/api/v1/releases/{release_id}/activate"
        r = await self._http.post(url, headers=self._headers())
        return await self._check(r, "POST", url)

    async def rollback_release(self, release_id: str) -> dict:
        url = f"/api/v1/releases/{release_id}/rollback"
        r = await self._http.post(url, headers=self._headers())
        return await self._check(r, "POST", url)

    async def catalog_search(self, workspace_id: str, query: str = "", limit: int = 100) -> list[dict]:
        params = {"workspace_id": workspace_id}
        if query:
            params["q"] = query
        if limit:
            params["limit"] = str(limit)
        url = "/api/v1/catalog/capabilities"
        r = await self._http.get(url, params=params, headers=self._headers())
        return await self._check(r, "GET", url)

    async def update_capability_contract(self, capability_id: str, contract: dict) -> dict:
        url = f"/api/v1/capabilities/{capability_id}/contract"
        r = await self._http.put(url, json=contract, headers=self._json_headers())
        return await self._check(r, "PUT", url)

    async def run_eval(self, release_id: str, dataset_id: str, scorer: str = "exact_match") -> dict:
        url = f"/api/v1/releases/{release_id}/evals"
        r = await self._http.post(url, json={"dataset_id": dataset_id, "scorer": scorer}, headers=self._json_headers())
        return await self._check(r, "POST", url)
