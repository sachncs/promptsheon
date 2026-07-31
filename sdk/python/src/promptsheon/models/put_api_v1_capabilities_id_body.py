from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="PutApiV1CapabilitiesIdBody")


@_attrs_define
class PutApiV1CapabilitiesIdBody:
    """
    Attributes:
        name (str | Unset):
        description (str | Unset):
        owner (str | Unset):
        tags (list[Any] | Unset):
    """

    name: str | Unset = UNSET
    description: str | Unset = UNSET
    owner: str | Unset = UNSET
    tags: list[Any] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        description = self.description

        owner = self.owner

        tags: list[Any] | Unset = UNSET
        if not isinstance(self.tags, Unset):
            tags = self.tags

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if name is not UNSET:
            field_dict["name"] = name
        if description is not UNSET:
            field_dict["description"] = description
        if owner is not UNSET:
            field_dict["owner"] = owner
        if tags is not UNSET:
            field_dict["tags"] = tags

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        name = d.pop("name", UNSET)

        description = d.pop("description", UNSET)

        owner = d.pop("owner", UNSET)

        tags = cast(list[Any], d.pop("tags", UNSET))

        put_api_v1_capabilities_id_body = cls(
            name=name,
            description=description,
            owner=owner,
            tags=tags,
        )

        put_api_v1_capabilities_id_body.additional_properties = d
        return put_api_v1_capabilities_id_body

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
