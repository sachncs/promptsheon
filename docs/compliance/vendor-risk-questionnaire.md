# Vendor security questionnaire

This is the questionnaire promptsheon sends to its tier-1
vendors (OpenAI, Anthropic, AWS for Bedrock, our infrastructure
providers). It mirrors the SIG / CAIQ structure but in
single-document form so vendors who don't use those tools can
fill it in directly.

## Section 1 — Vendor profile

| Field | Vendor response |
|---|---|
| Vendor name | |
| Vendor primary contact (security) | |
| Vendor primary contact (privacy) | |
| Product(s) promptsheon integrates with | |
| Data residency options for our tenant | |
| Subprocessors disclosed? Link: | |

## Section 2 — Certifications

For each, attach the most recent report on request.

| Certification | Status | Issue date | Expiry |
|---|---|---|---|
| SOC 2 Type II | | | |
| ISO/IEC 27001 | | | |
| ISO/IEC 27017 (cloud) | | | |
| ISO/IEC 27018 (PII) | | | |
| HIPAA BAA available? | | | |
| GDPR DPA available? | | | |
| PCI-DSS (if applicable) | | | |

## Section 3 — Data handling

| Question | Vendor response |
|---|---|
| Where is our data stored at rest? (region(s)) | |
| Where is our data processed? | |
| Is our data used for model training by default? | |
| Is there a per-tenant opt-out for training? | |
| How long is our data retained by default? | |
| How do we request deletion of our data? (URL/API) | |
| Is our data encrypted at rest? Algorithm + key length | |
| Is our data encrypted in transit? (TLS version + min) | |
| Are encryption keys customer-managed (BYOK)? | |

## Section 4 — Access control

| Question | Vendor response |
|---|---|
| Do you support SSO via SAML 2.0 or OIDC? | |
| Do you support SCIM 2.0 for user provisioning? | |
| RBAC granularity (roles, custom permissions)? | |
| Audit log retention period? | |
| Do you support log export to a SIEM (Splunk, Datadog, etc.)? | |

## Section 5 — Incident response

| Question | Vendor response |
|---|---|
| Breach notification SLA (hours)? | |
| Contact path for a confirmed breach affecting us? | |
| Have you had a notifiable breach in the last 24 months? | |
| Public trust centre URL: | |

## Section 6 — Business continuity

| Question | Vendor response |
|---|---|
| Published uptime SLA: | |
| Last 12 months' actual uptime: | |
| Disaster recovery RPO / RTO: | |
| Region failover capability: | |

## Section 7 — Vulnerability management

| Question | Vendor response |
|---|---|
| Annual external pen-test cadence? | |
| Bug-bounty program (Y/N, scope)? | |
| Mean time to patch High / Critical CVEs? | |
| Last CVE published by vendor: | |

## Section 8 — Sub-processor list

List every sub-processor that handles our data, with location:

| Sub-processor | Purpose | Data category | Location |
|---|---|---|---|
| | | | |

## Section 9 — Open-source components (if relevant)

| Question | Vendor response |
|---|---|
| List of open-source libraries embedded in the vendor's runtime | |
| Vendor's policy on CVE handling for those libraries | |

## Section 10 — Sub-processor change notification

| Question | Vendor response |
|---|---|
| Notification SLA for new sub-processors? | |
| Right to object to sub-processor changes? | |

## Section 11 — Data deletion / portability

| Question | Vendor response |
|---|---|
| API to bulk-export all our data? | |
| API to bulk-delete all our data? | |
| Confirmation receipt after deletion? | |
| Backup retention period after deletion request? | |

## Section 12 — Audit rights

| Question | Vendor response |
|---|---|
| Right to audit (on-site or remote)? | |
| Right to request third-party audit reports (SOC 2)? | |
| Frequency of standard audit reports issued? | |
| NDA required to receive audit reports? | |

## Sign-off

By signing below, vendor confirms the answers above are accurate
as of the date below and agrees to notify promptsheon of any
material change within 30 days.

| | |
|---|---|
| Vendor signatory (security): | |
| Name: | |
| Date: | |
| promptsheon signatory: | |
| Date: | |
