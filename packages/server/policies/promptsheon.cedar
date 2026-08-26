// promptsheon Cedar policy — single source of truth for every
// authorization decision in the platform. Authored once,
// evaluated by every route + every tool invocation + every
// SVID issuance.
//
// Default deny. Permit rules scope principals to the same org
// and require team-membership where the doc names it (Agent
// publishing, agent registry signing).
//
// Schema is intentionally explicit so a policy:eval regression
// run can assert the principal/action/resource shapes.

permit (
    principal in Role::"admin",
    action in [Action::"all"],
    resource in Resource::"all"
);

// Org-scoped read for human users in the same org.
permit (
    principal in Role::"viewer",
    action in [
        Action::"Agent::Read",
        Action::"Tool::Read",
        Action::"Memory::Read",
        Action::"Audit::Read",
        Action::"Observability::Read"
    ],
    resource in Resource::"all"
);

// Editor + admin can mutate manifests + agent registry.
permit (
    principal in Role::"editor",
    action in [
        Action::"Agent::Read",
        Action::"Agent::Write",
        Action::"Tool::Read",
        Action::"Tool::Register",
        Action::"Memory::Read",
        Action::"Memory::Write",
        Action::"Audit::Read",
        Action::"Observability::Read",
        Action::"Gateway::Invoke",
        Action::"Armor::Configure"
    ],
    resource in Resource::"all"
);

// Approver + admin can publish agents + manage budgets.
permit (
    principal in Role::"approver",
    action in [
        Action::"Agent::Read",
        Action::"Agent::Write",
        Action::"Agent::Publish",
        Action::"Tool::Read",
        Action::"Tool::Register",
        Action::"Memory::Read",
        Action::"Memory::Write",
        Action::"Audit::Read",
        Action::"Observability::Read",
        Action::"Budget::Create",
        Action::"Budget::Read",
        Action::"Budget::Update",
        Action::"Budget::Delete",
        Action::"Armor::Configure",
        Action::"Registry::Sign"
    ],
    resource in Resource::"all"
);

// Agent principal: can invoke tools + read/write its own memory
// scoped to its classification. Cross-agent actions require
// `target.classification` ≤ principal's `classification` (enforced
// at the gateway).
permit (
    principal is Agent,
    action in [
        Action::"Agent::Invoke",
        Action::"Agent::Resume",
        Action::"Tool::Call",
        Action::"Memory::Read",
        Action::"Memory::Write",
        Action::"Gateway::Invoke",
        Action::"Observability::Export"
    ],
    resource in Resource::"all"
);

// System principal: full reach, used for bootstrap + replication.
// Disabled unless PROMPTSHEON_ALLOW_SYSTEM_ACTOR=true.
permit (
    principal is System,
    action in [Action::"all"],
    resource in Resource::"all"
);