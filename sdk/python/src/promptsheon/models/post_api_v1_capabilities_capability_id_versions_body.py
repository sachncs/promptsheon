from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.object_ import Object


T = TypeVar("T", bound="PostApiV1CapabilitiesCapabilityIdVersionsBody")


@_attrs_define
class PostApiV1CapabilitiesCapabilityIdVersionsBody:
    """
    Attributes:
        version (float | Unset):
        manifest (Object | Unset):
        parents (list[Any] | Unset):
    """

    version: float | Unset = UNSET
    manifest: Object | Unset = UNSET
    parents: list[Any] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        version = self.version

        manifest: dict[str, Any] | Unset = UNSET
        if not isinstance(self.manifest, Unset):
            manifest = self.manifest.to_dict()

        parents: list[Any] | Unset = UNSET
        if not isinstance(self.parents, Unset):
            parents = self.parents

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if version is not UNSET:
            field_dict["version"] = version
        if manifest is not UNSET:
            field_dict["manifest"] = manifest
        if parents is not UNSET:
            field_dict["parents"] = parents

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.object_ import Object

        d = dict(src_dict)
        version = d.pop("version", UNSET)

        _manifest = d.pop("manifest", UNSET)
        manifest: Object | Unset
        if isinstance(_manifest, Unset):
            manifest = UNSET
        else:
            manifest = Object.from_dict(_manifest)

        parents = cast(list[Any], d.pop("parents", UNSET))

        post_api_v1_capabilities_capability_id_versions_body = cls(
            version=version,
            manifest=manifest,
            parents=parents,
        )

        post_api_v1_capabilities_capability_id_versions_body.additional_properties = d
        return post_api_v1_capabilities_capability_id_versions_body

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
