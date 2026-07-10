"""Minimal HTTP client for the Promptsheon v1 API.

The shape mirrors the TypeScript client in sdk/typescript: every
method returns the OpenAPI-typed response body; errors are raised
as PromptsheonAPIError with the underlying httpx.HTTPStatusError
captured for callers that want retry semantics.
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

    def list_capabilities(self, project_id: str) -> list[dict]:
        """List Capabilities in a Project.

        GET /v1/projects/{project_id}/capabilities
        """
        url = f"/v1/projects/{project_id}/capabilities"
        r = self._http.get(url, headers=self._headers())
        if r.status_code != 200:
            raise PromptsheonAPIError(r.status_code, "GET", url, r.text)
        return r.json()

    def invoke_release(
        self, release_id: str, inputs: dict[str, Any]
    ) -> dict:
        """Invoke a Release: bind input -> Provider -> Output.

        POST /v1/releases/{release_id}/invoke
        """
        url = f"/v1/releases/{release_id}/invoke"
        body = {"inputs": inputs}
        headers = self._headers()
        headers["Content-Type"] = "application/json"
        r = self._http.post(url, json=body, headers=headers)
        if r.status_code != 200:
            raise PromptsheonAPIError(r.status_code, "POST", url, r.text)
        return r.json()


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

    async def list_capabilities(self, project_id: str) -> list[dict]:
        url = f"/v1/projects/{project_id}/capabilities"
        r = await self._http.get(url, headers=self._headers())
        if r.status_code != 200:
            raise PromptsheonAPIError(r.status_code, "GET", url, r.text)
        return r.json()

    async def invoke_release(
        self, release_id: str, inputs: dict[str, Any]
    ) -> dict:
        url = f"/v1/releases/{release_id}/invoke"
        body = {"inputs": inputs}
        headers = self._headers()
        headers["Content-Type"] = "application/json"
        r = await self._http.post(url, json=body, headers=headers)
        if r.status_code != 200:
            raise PromptsheonAPIError(r.status_code, "POST", url, r.text)
        return r.json()
