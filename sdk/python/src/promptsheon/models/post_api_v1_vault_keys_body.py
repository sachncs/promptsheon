from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="PostApiV1VaultKeysBody")


@_attrs_define
class PostApiV1VaultKeysBody:
    """
    Attributes:
        provider_name (str | Unset):
        key_name (str | Unset):
        plaintext_key (str | Unset):
    """

    provider_name: str | Unset = UNSET
    key_name: str | Unset = UNSET
    plaintext_key: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        provider_name = self.provider_name

        key_name = self.key_name

        plaintext_key = self.plaintext_key

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if provider_name is not UNSET:
            field_dict["providerName"] = provider_name
        if key_name is not UNSET:
            field_dict["keyName"] = key_name
        if plaintext_key is not UNSET:
            field_dict["plaintextKey"] = plaintext_key

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        provider_name = d.pop("providerName", UNSET)

        key_name = d.pop("keyName", UNSET)

        plaintext_key = d.pop("plaintextKey", UNSET)

        post_api_v1_vault_keys_body = cls(
            provider_name=provider_name,
            key_name=key_name,
            plaintext_key=plaintext_key,
        )

        post_api_v1_vault_keys_body.additional_properties = d
        return post_api_v1_vault_keys_body

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
