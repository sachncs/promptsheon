from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="PostApiV1WebhooksBody")


@_attrs_define
class PostApiV1WebhooksBody:
    """
    Attributes:
        u_rl (str | Unset):
        events (list[Any] | Unset):
        secret (str | Unset):
    """

    u_rl: str | Unset = UNSET
    events: list[Any] | Unset = UNSET
    secret: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        u_rl = self.u_rl

        events: list[Any] | Unset = UNSET
        if not isinstance(self.events, Unset):
            events = self.events

        secret = self.secret

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if u_rl is not UNSET:
            field_dict["uRL"] = u_rl
        if events is not UNSET:
            field_dict["events"] = events
        if secret is not UNSET:
            field_dict["secret"] = secret

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        u_rl = d.pop("uRL", UNSET)

        events = cast(list[Any], d.pop("events", UNSET))

        secret = d.pop("secret", UNSET)

        post_api_v1_webhooks_body = cls(
            u_rl=u_rl,
            events=events,
            secret=secret,
        )

        post_api_v1_webhooks_body.additional_properties = d
        return post_api_v1_webhooks_body

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
