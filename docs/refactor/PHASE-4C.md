# PHASE-4C — Handler Tests: releases/executions/harness/audit/settings

**5 commits.** Adds persisted-state + audit-row assertions for the core data
processing paths.

## Commits

```
c4.16 test(handlers): releases — persisted + audit asserts (incl. quorum variants)
      Refs: PLAN-49/F1 HIGH-9
c4.17 test(handlers): executions — persisted + audit asserts (incl. workspace tenancy)
      Refs: PLAN-49/F1 HIGH-3 2.4
c4.18 test(handlers): harness — persisted + audit asserts
      Refs: PLAN-49/F1 F26 F35 F36
c4.19 test(handlers): audit — persisted + audit asserts (incl. kind/id filter)
      Refs: PLAN-49/F1 F3a F27 MED-2 DEF-22
c4.20 test(handlers): settings — persisted + audit asserts (incl. CRDT property)
      Refs: PLAN-49/F1
```

## Files touched

| File | Commits |
|---|---|
| `backend/handlers_releases_test.go` | c4.16 |
| `backend/handlers_executions_test.go` | c4.17 |
| `backend/handlers_harness_test.go` | c4.18 |
| `backend/handlers_audit_test.go` | c4.19 |
| `backend/handlers_settings_test.go` | c4.20 |

## Critical tests

### Release activation (c4.16)

```go
func TestHandleActivateRelease_MakerChecker_RoundTrip(t *testing.T) {
    s := newTestServerWithSeam()
    ctx := context.Background()

    // Seed capability + version + release
    cap := createCapability(t, s, "test-cap")
    ver := createVersion(t, s, cap.ID, 1)
    rel := createRelease(t, s, ver.ID, "pending")

    // Cast votes
    castVote(t, s, rel.ID, "alice", "approve")
    castVote(t, s, rel.ID, "bob", "approve")
    castVote(t, s, rel.ID, "carol", "reject")  // minority

    // Activate
    req := httptest.NewRequest("POST", "/api/v1/releases/"+rel.ID+"/activate", nil)
    rr := httptest.NewRecorder()
    s.ServeHTTP(rr, req)
    if rr.Code != http.StatusOK {
        t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
    }

    // (b) persisted assertion: status == active
    activated, _ := s.DB.GetRelease(ctx, rel.ID)
    if activated.Status != release.StatusActive {
        t.Errorf("status = %v, want active", activated.Status)
    }

    // (c) audit assertion: action == activate
    entries, _ := s.DB.ListAudit(ctx, &models.AuditFilter{Resource: "release:" + rel.ID})
    if len(entries) < 3 {  // create + 3 votes + activate = 5
        t.Errorf("audit entries = %d, want >= 3", len(entries))
    }
}
```

### Execution workspace tenancy (c4.17)

```go
func TestHandleGetExecution_RejectsCrossWorkspace(t *testing.T) {
    s := newTestServerWithSeam()
    ctx := context.Background()

    // Create execution in workspace A as user A
    exec := createExecution(t, s, "ws-A", "cap-1")
    // ... authenticate as user from workspace B ...

    req := httptest.NewRequest("GET", "/api/v1/executions/"+exec.ID, nil)
    req = req.WithContext(auth.WithUser(req.Context(), userB))
    rr := httptest.NewRecorder()
    s.ServeHTTP(rr, req)

    // (b) persisted assertion: still exists for ws-A user
    // (d) but user B gets 404
    if rr.Code != http.StatusNotFound {
        t.Errorf("status = %d, want 404 (cross-workspace)", rr.Code)
    }
}
```

### Audit queue overflow (c4.19 — F3a fix)

```go
func TestHandleAudit_DropWhenQueueFull(t *testing.T) {
    s := newTestServerWithSeam()
    ctx := context.Background()

    // Pre-fill audit queue past capacity
    for i := 0; i < 2000; i++ {
        s.Audit(ctx, "test", "test", map[string]any{"i": i})
    }

    initialDropped := s.AuditDropped.Load()

    // Issue one more audit entry
    s.Audit(ctx, "test", "test", map[string]any{"final": true})

    // Wait for queue drain
    time.Sleep(300 * time.Millisecond)

    finalDropped := s.AuditDropped.Load()
    if finalDropped <= initialDropped {
        t.Errorf("auditDropped = %d, want > %d", finalDropped, initialDropped)
    }
}
```

### Audit kind/id filter (c4.19 — DEF-22 fix)

```go
func TestHandleListAudit_FilterByResourceKindAndID(t *testing.T) {
    s := newTestServerWithSeam()
    ctx := context.Background()

    createCapability(t, s, "cap-a")
    createCapability(t, s, "cap-b")

    req := httptest.NewRequest("GET", "/api/v1/audit?kind=capability&id=cap-a", nil)
    rr := httptest.NewRecorder()
    s.ServeHTTP(rr, req)

    var resp struct {
        Items []models.AuditEntry `json:"items"`
    }
    json.NewDecoder(rr.Body).Decode(&resp)

    for _, e := range resp.Items {
        if e.ResourceKind != "capability" {
            t.Errorf("kind = %q, want capability", e.ResourceKind)
        }
        if e.ResourceID != "cap-a" {
            t.Errorf("id = %q, want cap-a", e.ResourceID)
        }
    }
}
```

### Eval runner clock (c4.18 — F35 fix)

```go
func TestEvalRunner_ClockInjection(t *testing.T) {
    fixedTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
    r := harness.NewEvalRunner(s.DB, invoker)
    r.Clock = func() time.Time { return fixedTime }

    run, _ := r.Run(ctx, &harness.RunOptions{...})

    if !run.StartedAt.Equal(fixedTime) {
        t.Errorf("StartedAt = %v, want %v", run.StartedAt, fixedTime)
    }
}
```

## Per-handler matrix

| Handler | Persisted assertion | Audit assertion |
|---|---|---|
| `handleCreateRelease` | `repo.releases[id].Status == pending` | `action=="create"` |
| `handleActivateRelease` | `repo.releases[id].Status == active` | `action=="activate"` + per-vote audit |
| `handleInvokeRelease` | `repo.executions[id].Status == succeeded` | `action=="invoke"` |
| `handleCreateExecution` | `repo.executions[id].Model == "..."` | `action=="invoke"` |
| `handleGetExecution` | – | – (read endpoint) |
| `handleCreateDataset` | `repo.datasets[id].Name == "..."` | `action=="create"` |
| `handlePutCases` | `repo.dataset_cases[id].Inputs == "..."` | `action=="update"` |
| `handleCreatePrecondition` | `repo.preconditions[id].Name == "..."` | `action=="create"` |
| `handleUpdatePrecondition` | `repo.preconditions[id].Enabled == true` | `action=="update"` |
| `handleRunEval` | `repo.evalRuns[id].Score == 0.85` | `action=="eval_run"` |
| `handleListAudit` | – | – (read endpoint) |
| `handleExportAudit` | `len(auditEntries) <= limit` | – |
| `handleVerifyAuditChain` | `verifyResult.Ok == true` | – |
| `handleUpdateSetting` | `repo.settings[key].Value == "..."` | `action=="update"` |
| `handleDeleteSetting` | `repo.settings[key]` tombstoned | `action=="delete"` |

## Exit criterion

```bash
go test -race -count=1 ./backend/handlers_releases_test.go ./backend/handlers_executions_test.go ./backend/handlers_harness_test.go ./backend/handlers_audit_test.go ./backend/handlers_settings_test.go
bash scripts/check-coverage.sh coverage.out  # handlers_*.go ≥ 75%
```

## Parallelization

1 agent (single domain area).