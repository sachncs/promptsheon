from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.object_ import Object


T = TypeVar("T", bound="PostApiV1ApikeysBody")


@_attrs_define
class PostApiV1ApikeysBody:
    """
    Attributes:
        name (str | Unset):
        user_id (str | Unset):
        role (str | Unset):
        expires_at (Object | Unset):
    """

    name: str | Unset = UNSET
    user_id: str | Unset = UNSET
    role: str | Unset = UNSET
    expires_at: Object | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        user_id = self.user_id

        role = self.role

        expires_at: dict[str, Any] | Unset = UNSET
        if not isinstance(self.expires_at, Unset):
            expires_at = self.expires_at.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if name is not UNSET:
            field_dict["name"] = name
        if user_id is not UNSET:
            field_dict["userID"] = user_id
        if role is not UNSET:
            field_dict["role"] = role
        if expires_at is not UNSET:
            field_dict["expiresAt"] = expires_at

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.object_ import Object

        d = dict(src_dict)
        name = d.pop("name", UNSET)

        user_id = d.pop("userID", UNSET)

        role = d.pop("role", UNSET)

        _expires_at = d.pop("expiresAt", UNSET)
        expires_at: Object | Unset
        if isinstance(_expires_at, Unset):
            expires_at = UNSET
        else:
            expires_at = Object.from_dict(_expires_at)

        post_api_v1_apikeys_body = cls(
            name=name,
            user_id=user_id,
            role=role,
            expires_at=expires_at,
        )

        post_api_v1_apikeys_body.additional_properties = d
        return post_api_v1_apikeys_body

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
