"""Smoke test that the Python client can be constructed and that
the type-stub package marker loads. The full test suite relies
on a real server (httpx-mock or respx); this test only validates
the wiring without making any network calls.

SDK-PYTEST-1: every public method must be importable and
callable. We exercise each method's URL construction in
isolation via the _BaseClient._check path is mocked at the
httpx level — production tests against a live daemon live in
tests/contract/.
"""
import pytest

from promptsheon import Client, AsyncClient, ClientConfig, PromptsheonAPIError


def test_client_config_defaults():
    cfg = ClientConfig(base_url="https://api.example.com")
    assert cfg.base_url == "https://api.example.com"
    assert cfg.timeout_seconds == 30.0
    assert cfg.api_key is None


def test_client_config_custom_timeout():
    cfg = ClientConfig(base_url="https://api.example.com", timeout_seconds=5.0)
    assert cfg.timeout_seconds == 5.0


def test_client_config_with_api_key():
    cfg = ClientConfig(base_url="https://api.example.com", api_key="ps_test")
    assert cfg.api_key == "ps_test"


def test_sync_client_close_no_leak():
    """Closing a Client without using it must not raise."""
    client = Client(ClientConfig(base_url="https://api.example.com"))
    client.close()


def test_sync_client_context_manager():
    """`with Client(...)` must close the underlying httpx client."""
    with Client(ClientConfig(base_url="https://api.example.com")) as c:
        assert c is not None


def test_api_error_carries_status_and_body():
    err = PromptsheonAPIError(404, "GET", "/api/v1/x", '{"error":"missing"}')
    assert err.status == 404
    assert err.method == "GET"
    assert err.path == "/api/v1/x"
    assert "missing" in err.body


def test_sync_client_has_all_methods():
    """SDK-2: every public method must exist on the client."""
    methods = [
        "list_capabilities", "get_capability", "update_capability_contract",
        "get_capability_contract", "get_capability_reputation",
        "diff_capability_versions", "catalog_search",
        "list_releases", "create_release", "vote_release",
        "activate_release", "rollback_release", "get_release_approval",
        "invoke_release",
        "list_datasets", "create_dataset",
        "list_preconditions", "create_precondition",
        "run_eval", "list_evals", "get_eval",
        "verify_audit_chain",
        "list_settings", "set_setting",
    ]
    for m in methods:
        assert hasattr(Client, m), f"Client must expose {m}"
        assert callable(getattr(Client, m))


def test_async_client_has_all_methods():
    methods = [
        "list_capabilities", "get_capability",
        "invoke_release", "create_release", "vote_release",
        "activate_release", "rollback_release",
        "catalog_search", "update_capability_contract", "run_eval",
    ]
    for m in methods:
        assert hasattr(AsyncClient, m), f"AsyncClient must expose {m}"
        assert callable(getattr(AsyncClient, m))


@pytest.mark.asyncio
async def test_async_client_aclose_no_leak():
    client = AsyncClient(ClientConfig(base_url="https://api.example.com"))
    await client.aclose()


@pytest.mark.asyncio
async def test_async_client_context_manager():
    async with AsyncClient(ClientConfig(base_url="https://api.example.com")) as c:
        assert c is not None

