from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.post_api_v1_alerts_rules_body_config import PostApiV1AlertsRulesBodyConfig


T = TypeVar("T", bound="PostApiV1AlertsRulesBody")


@_attrs_define
class PostApiV1AlertsRulesBody:
    """
    Attributes:
        name (str | Unset):
        type_ (str | Unset):
        severity (str | Unset):
        threshold (float | Unset):
        duration (float | Unset):
        window (float | Unset):
        config (PostApiV1AlertsRulesBodyConfig | Unset):
    """

    name: str | Unset = UNSET
    type_: str | Unset = UNSET
    severity: str | Unset = UNSET
    threshold: float | Unset = UNSET
    duration: float | Unset = UNSET
    window: float | Unset = UNSET
    config: PostApiV1AlertsRulesBodyConfig | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        type_ = self.type_

        severity = self.severity

        threshold = self.threshold

        duration = self.duration

        window = self.window

        config: dict[str, Any] | Unset = UNSET
        if not isinstance(self.config, Unset):
            config = self.config.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if name is not UNSET:
            field_dict["name"] = name
        if type_ is not UNSET:
            field_dict["type"] = type_
        if severity is not UNSET:
            field_dict["severity"] = severity
        if threshold is not UNSET:
            field_dict["threshold"] = threshold
        if duration is not UNSET:
            field_dict["duration"] = duration
        if window is not UNSET:
            field_dict["window"] = window
        if config is not UNSET:
            field_dict["config"] = config

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.post_api_v1_alerts_rules_body_config import PostApiV1AlertsRulesBodyConfig

        d = dict(src_dict)
        name = d.pop("name", UNSET)

        type_ = d.pop("type", UNSET)

        severity = d.pop("severity", UNSET)

        threshold = d.pop("threshold", UNSET)

        duration = d.pop("duration", UNSET)

        window = d.pop("window", UNSET)

        _config = d.pop("config", UNSET)
        config: PostApiV1AlertsRulesBodyConfig | Unset
        if isinstance(_config, Unset):
            config = UNSET
        else:
            config = PostApiV1AlertsRulesBodyConfig.from_dict(_config)

        post_api_v1_alerts_rules_body = cls(
            name=name,
            type_=type_,
            severity=severity,
            threshold=threshold,
            duration=duration,
            window=window,
            config=config,
        )

        post_api_v1_alerts_rules_body.additional_properties = d
        return post_api_v1_alerts_rules_body

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
