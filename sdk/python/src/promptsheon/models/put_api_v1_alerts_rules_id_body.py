from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.put_api_v1_alerts_rules_id_body_config import PutApiV1AlertsRulesIdBodyConfig


T = TypeVar("T", bound="PutApiV1AlertsRulesIdBody")


@_attrs_define
class PutApiV1AlertsRulesIdBody:
    """
    Attributes:
        name (str | Unset):
        enabled (bool | Unset):
        severity (str | Unset):
        threshold (float | Unset):
        config (PutApiV1AlertsRulesIdBodyConfig | Unset):
    """

    name: str | Unset = UNSET
    enabled: bool | Unset = UNSET
    severity: str | Unset = UNSET
    threshold: float | Unset = UNSET
    config: PutApiV1AlertsRulesIdBodyConfig | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        enabled = self.enabled

        severity = self.severity

        threshold = self.threshold

        config: dict[str, Any] | Unset = UNSET
        if not isinstance(self.config, Unset):
            config = self.config.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if name is not UNSET:
            field_dict["name"] = name
        if enabled is not UNSET:
            field_dict["enabled"] = enabled
        if severity is not UNSET:
            field_dict["severity"] = severity
        if threshold is not UNSET:
            field_dict["threshold"] = threshold
        if config is not UNSET:
            field_dict["config"] = config

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.put_api_v1_alerts_rules_id_body_config import PutApiV1AlertsRulesIdBodyConfig

        d = dict(src_dict)
        name = d.pop("name", UNSET)

        enabled = d.pop("enabled", UNSET)

        severity = d.pop("severity", UNSET)

        threshold = d.pop("threshold", UNSET)

        _config = d.pop("config", UNSET)
        config: PutApiV1AlertsRulesIdBodyConfig | Unset
        if isinstance(_config, Unset):
            config = UNSET
        else:
            config = PutApiV1AlertsRulesIdBodyConfig.from_dict(_config)

        put_api_v1_alerts_rules_id_body = cls(
            name=name,
            enabled=enabled,
            severity=severity,
            threshold=threshold,
            config=config,
        )

        put_api_v1_alerts_rules_id_body.additional_properties = d
        return put_api_v1_alerts_rules_id_body

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
