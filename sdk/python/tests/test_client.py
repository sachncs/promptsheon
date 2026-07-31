"""Smoke test that the Python client package imports cleanly.

The actual client methods are exercised by the daemon's
contract test (tests/contract/) against a live daemon. This
file just verifies the generated SDK wires up.
"""
from promptsheon import Client, AuthenticatedClient


def test_package_imports():
    # If the generated package is broken, the import above fails.
    assert Client is not None
    assert AuthenticatedClient is not None
