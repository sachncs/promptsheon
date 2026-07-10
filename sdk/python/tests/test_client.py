"""Smoke test that the Python client can be constructed and that
the type-stub package marker loads. The full test suite relies
on a real server (httpx-mock or respx); this test only validates
the wiring without making any network calls.
"""
import pytest

from promptsheon import Client, AsyncClient, ClientConfig


def test_client_config_defaults():
    cfg = ClientConfig(base_url="https://api.example.com")
    assert cfg.base_url == "https://api.example.com"
    assert cfg.timeout_seconds == 30.0
    assert cfg.api_key is None


def test_sync_client_close_no_leak():
    """Closing a Client without using it must not raise."""
    client = Client(ClientConfig(base_url="https://api.example.com"))
    client.close()


@pytest.mark.asyncio
async def test_async_client_aclose_no_leak():
    client = AsyncClient(ClientConfig(base_url="https://api.example.com"))
    await client.aclose()
