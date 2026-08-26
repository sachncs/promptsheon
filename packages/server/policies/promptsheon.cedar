// promptsheon Cedar policy — single source of truth for every
// authorization decision in the platform. Authored once,
// evaluated by every route + every tool invocation + every
// SVID issuance.
//
// Default deny. Permit rules scope principals to the same org
// and require team-membership where the doc names it (Agent
// publishing, agent registry signing).
//
// Cedar 4.x does not support `Action::"..."` wildcards for the
// implicit Action type. `action` (unbound) matches any action;
// enumerated lists are used when the role's reach is limited.

permit (
    principal == promptsheon::Role::"admin",
    action,
    resource
);

// Org-scoped read for human users in the same org.
permit (
    principal == promptsheon::Role::"viewer",
    action in [
        promptsheon::Action::"Agent::Read",
        promptsheon::Action::"Tool::Read",
        promptsheon::Action::"Memory::Read",
        promptsheon::Action::"Audit::Read",
        promptsheon::Action::"Observability::Read"
    ],
    resource
);

// Editor + admin can mutate manifests + agent registry.
permit (
    principal == promptsheon::Role::"editor",
    action in [
        promptsheon::Action::"Agent::Read",
        promptsheon::Action::"Agent::Write",
        promptsheon::Action::"Tool::Read",
        promptsheon::Action::"Tool::Register",
        promptsheon::Action::"Memory::Read",
        promptsheon::Action::"Memory::Write",
        promptsheon::Action::"Audit::Read",
        promptsheon::Action::"Observability::Read",
        promptsheon::Action::"Gateway::Invoke",
        promptsheon::Action::"Armor::Configure"
    ],
    resource
);

// Approver + admin can publish agents + manage budgets.
permit (
    principal == promptsheon::Role::"approver",
    action in [
        promptsheon::Action::"Agent::Read",
        promptsheon::Action::"Agent::Write",
        promptsheon::Action::"Agent::Publish",
        promptsheon::Action::"Tool::Read",
        promptsheon::Action::"Tool::Register",
        promptsheon::Action::"Memory::Read",
        promptsheon::Action::"Memory::Write",
        promptsheon::Action::"Audit::Read",
        promptsheon::Action::"Observability::Read",
        promptsheon::Action::"Budget::Create",
        promptsheon::Action::"Budget::Read",
        promptsheon::Action::"Budget::Update",
        promptsheon::Action::"Budget::Delete",
        promptsheon::Action::"Armor::Configure",
        promptsheon::Action::"Registry::Sign"
    ],
    resource
);

// Agent principal: can invoke tools + read/write its own memory
// scoped to its classification. Cross-agent actions require
// `target.classification` ≤ principal's `classification` (enforced
// at the gateway).
permit (
    principal is promptsheon::Agent,
    action in [
        promptsheon::Action::"Agent::Invoke",
        promptsheon::Action::"Agent::Resume",
        promptsheon::Action::"Tool::Call",
        promptsheon::Action::"Memory::Read",
        promptsheon::Action::"Memory::Write",
        promptsheon::Action::"Gateway::Invoke",
        promptsheon::Action::"Observability::Export"
    ],
    resource
);

// System principal: full reach, used for bootstrap + replication.
// Disabled unless PROMPTSHEON_ALLOW_SYSTEM_ACTOR=true.
permit (
    principal is promptsheon::System,
    action,
    resource
);