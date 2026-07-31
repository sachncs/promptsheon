from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.post_api_v1_versions_version_id_executions_body_inputs import (
        PostApiV1VersionsVersionIdExecutionsBodyInputs,
    )


T = TypeVar("T", bound="PostApiV1VersionsVersionIdExecutionsBody")


@_attrs_define
class PostApiV1VersionsVersionIdExecutionsBody:
    """
    Attributes:
        inputs (PostApiV1VersionsVersionIdExecutionsBodyInputs | Unset):
        model (str | Unset):
        provider (str | Unset):
    """

    inputs: PostApiV1VersionsVersionIdExecutionsBodyInputs | Unset = UNSET
    model: str | Unset = UNSET
    provider: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        inputs: dict[str, Any] | Unset = UNSET
        if not isinstance(self.inputs, Unset):
            inputs = self.inputs.to_dict()

        model = self.model

        provider = self.provider

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if inputs is not UNSET:
            field_dict["inputs"] = inputs
        if model is not UNSET:
            field_dict["model"] = model
        if provider is not UNSET:
            field_dict["provider"] = provider

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.post_api_v1_versions_version_id_executions_body_inputs import (
            PostApiV1VersionsVersionIdExecutionsBodyInputs,
        )

        d = dict(src_dict)
        _inputs = d.pop("inputs", UNSET)
        inputs: PostApiV1VersionsVersionIdExecutionsBodyInputs | Unset
        if isinstance(_inputs, Unset):
            inputs = UNSET
        else:
            inputs = PostApiV1VersionsVersionIdExecutionsBodyInputs.from_dict(_inputs)

        model = d.pop("model", UNSET)

        provider = d.pop("provider", UNSET)

        post_api_v1_versions_version_id_executions_body = cls(
            inputs=inputs,
            model=model,
            provider=provider,
        )

        post_api_v1_versions_version_id_executions_body.additional_properties = d
        return post_api_v1_versions_version_id_executions_body

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
