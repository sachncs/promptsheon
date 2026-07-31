# PHASE-4B — Handler Tests: users/workspaces/projects/capabilities/versions

**5 commits.** Adds persisted-state + audit-row assertions to handler tests for
the 5 most-touched handler files. Runs after PR-4A.

## Commits

```
c4.11 test(handlers): users — persisted + audit asserts
      Refs: PLAN-49/F1 F2
c4.12 test(handlers): workspaces — persisted + audit asserts
      Refs: PLAN-49/F1
c4.13 test(handlers): projects — persisted + audit asserts
      Refs: PLAN-49/F1
c4.14 test(handlers): capabilities — persisted + audit asserts
      Refs: PLAN-49/F1 F6 F23
c4.15 test(handlers): versions — persisted + audit asserts (incl. Knowledge inheritance)
      Refs: PLAN-49/F1 F37 CRIT-2
```

## Files touched

| File | Commits |
|---|---|
| `backend/handlers_users_test.go` | c4.11 |
| `backend/handlers_workspaces_test.go` | c4.12 |
| `backend/handlers_projects_test.go` | c4.13 |
| `backend/handlers_capabilities_test.go` | c4.14 |
| `backend/handlers_versions_test.go` | c4.15 |

## Test shape (every commit)

```go
func TestHandleCreateUser_RoundTrip(t *testing.T) {
    s := newTestServerWithSeam()
    body := `{"email":"u@x.com","role":"reader"}`
    req := httptest.NewRequest("POST", "/api/v1/users", strings.NewReader(body))
    rr := httptest.NewRecorder()
    s.ServeHTTP(rr, req)

    if rr.Code != http.StatusCreated {
        t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
    }
    var resp map[string]any
    json.NewDecoder(rr.Body).Decode(&resp)

    // (b) persisted-state assertion
    id := resp["id"].(string)
    user, err := s.DB.GetUser(context.Background(), id)
    if err != nil {
        t.Fatalf("GetUser after create: %v", err)
    }
    if user.Email != "u@x.com" {
        t.Errorf("persisted email = %q, want u@x.com", user.Email)
    }

    // (c) audit-row assertion
    entries, _ := s.DB.ListAudit(context.Background(), &models.AuditFilter{Resource: "user:" + id})
    if len(entries) != 1 {
        t.Errorf("audit entries = %d, want 1", len(entries))
    }
    if entries[0].Action != "create" {
        t.Errorf("audit action = %q, want create", entries[0].Action)
    }

    // (d) response-shape assertion (no secrets leaked)
    if _, ok := resp["key_hash"]; ok {
        t.Error("response leaked key_hash")
    }
}
```

## Knowledge inheritance test (c4.15)

```go
func TestHandleCreateVersion_InheritsKnowledge(t *testing.T) {
    s := newTestServerWithSeam()
    ctx := context.Background()

    // Seed parent version with Knowledge
    parent := createTestCapabilityWithVersion(t, s, "test-cap",
        &capability.Manifest{
            Prompt:   capability.PromptRef{Hash: "p1"},
            Tools:    []capability.ToolRef{{Name: "tool1"}},
            Knowledge: []capability.KnowledgeRef{{Source: "doc1"}, {Source: "doc2"}},
        })

    // Create child version with no own Knowledge
    body := `{"version": 2, "parents": ["` + parent.ID + `"], "manifest": {"prompt_hash": "p2"}}`
    req := httptest.NewRequest("POST", "/api/v1/capabilities/test-cap/versions", strings.NewReader(body))
    rr := httptest.NewRecorder()
    s.ServeHTTP(rr, req)

    if rr.Code != http.StatusCreated {
        t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
    }

    // Verify child version's manifest includes inherited Knowledge
    versions, _ := s.DB.ListVersions(ctx, "test-cap")
    child := versions[1]  // newest
    if len(child.Manifest.Knowledge) != 2 {
        t.Errorf("child.Knowledge = %d entries, want 2 (inherited)", len(child.Manifest.Knowledge))
    }
}
```

## Per-handler matrix

| Handler | Persisted assertion | Audit assertion | Response shape |
|---|---|---|---|
| `handleCreateUser` | `repo.users[id].Email == "u@x.com"` | `action=="create"` | no `password_hash` |
| `handleUpdateUser` | `repo.users[id].Role == "writer"` | `action=="update"` | – |
| `handleDeleteUser` | `GetUser(id)` returns `ErrStoreNotFound` | `action=="delete"` | – |
| `handleCreateWorkspace` | `repo.workspaces[id].Name == "test"` | `action=="create"` | – |
| `handleUpdateWorkspace` | `repo.workspaces[id].Description == "new"` | `action=="update"` | – |
| `handleCreateProject` | `repo.projects[id].WorkspaceID == "ws-1"` | `action=="create"` | – |
| `handleUpdateProject` | `repo.projects[id].Description == "new"` | `action=="update"` | – |
| `handleCreateCapability` | `repo.capabilities[id].Name == "test"` | `action=="create"` | no `state` field |
| `handleUpdateCapability` | `repo.capabilities[id].Description == "new"` | `action=="update"` | – |
| `handleDeleteCapability` | `GetCapability(id)` returns error | `action=="delete"` | – |
| `handleUpdateSelfEvolveConfig` | `repo.capabilities[id].SelfEvolveConfig.MaxAttempts == 5` | `action=="update"` | – |
| `handleUpdateCapabilityContract` | `repo.capabilities[id].Contract.InputSchema != ""` | `action=="update"` | – |
| `handleCreateVersion` | `repo.versions[id].ManifestHash != ""` | `action=="create"` | – |
| `handleCreateVersion (inheritance)` | child inherits parent's Knowledge | `action=="create"` | – |

## Exit criterion

```bash
go test -race -count=1 ./backend/handlers_users_test.go ./backend/handlers_workspaces_test.go ./backend/handlers_projects_test.go ./backend/handlers_capabilities_test.go ./backend/handlers_versions_test.go
bash scripts/check-coverage.sh coverage.out  # handlers_*.go ≥ 75%
```

## Parallelization

2 agents:

| Agent | Files |
|---|---|
| 4B1 | handlers_users_test.go, handlers_workspaces_test.go, handlers_projects_test.go |
| 4B2 | handlers_capabilities_test.go, handlers_versions_test.go |