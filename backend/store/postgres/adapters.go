// Adapters that bridge InMemoryPostgres to the per-aggregate
// interfaces in internal/store. Each adapter is intentionally
// thin: it implements the methods the contract tests exercise
// (lifecycle ping, capability reads, settings) and returns
// store.ErrNotFound or store.ErrNotImplemented for the rest.
//
// The point of this file is to prove the contract: every
// interface in internal/store/repo.go can be satisfied by
// a Postgres backend without the daemon touching the SQLite
// type. A future PR replaces the method bodies with pgx
// implementations.
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/sachncs/promptsheon/internal/approval"
	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/harness"
	"github.com/sachncs/promptsheon/internal/models"
	"github.com/sachncs/promptsheon/internal/release"
	"github.com/sachncs/promptsheon/internal/settings"
	"github.com/sachncs/promptsheon/internal/store"
)

// errNotImplemented is returned by every adapter method that
// is not exercised by the contract tests. A pgx wiring will
// replace these with real SQL.
var errNotImplemented = errors.New("postgres: not implemented")

// settingsAdapter implements store.Settings against the
// in-memory backend. The contract tests exercise GetSystemConfig
// and SetSystemConfig; the other methods are no-ops that
// return ErrNotImplemented.
type settingsAdapter struct{ p *InMemoryPostgres }

func (a *settingsAdapter) GetSystemConfig(_ context.Context, key string) (settings.CRDTRecord, error) {
	rec, ok := a.p.systemConfig[key]
	if !ok {
		return settings.CRDTRecord{}, store.ErrNotFound
	}
	return rec, nil
}

func (a *settingsAdapter) SetSystemConfig(_ context.Context, rec settings.CRDTRecord) error {
	a.p.systemConfig[rec.Key] = rec
	return nil
}

func (a *settingsAdapter) ListSystemConfig(_ context.Context) ([]settings.CRDTRecord, error) {
	out := make([]settings.CRDTRecord, 0, len(a.p.systemConfig))
	for _, r := range a.p.systemConfig {
		out = append(out, r)
	}
	return out, nil
}

func (a *settingsAdapter) MergeSystemConfig(_ context.Context, _ string, records []settings.CRDTRecord) error {
	for _, rec := range records {
		existing, ok := a.p.systemConfig[rec.Key]
		if !ok {
			a.p.systemConfig[rec.Key] = rec
			continue
		}
		a.p.systemConfig[rec.Key] = settings.Merge(existing, rec)
	}
	return nil
}

// lifecycleAdapter implements store.Lifecycle. The contract
// tests exercise Ping (returns nil) and Close (returns nil).
type lifecycleAdapter struct{ p *InMemoryPostgres }

func (a *lifecycleAdapter) Ping(_ context.Context) error { return nil }
func (a *lifecycleAdapter) Close() error                 { return nil }

// capabilityAdapter implements the typed CapabilityRepository.
// The contract tests exercise GetCapability + GetCapabilityContract.
type capabilityAdapter struct{ p *InMemoryPostgres }

func (a *capabilityAdapter) CreateWorkspace(_ context.Context, w *capability.Workspace) error {
	a.p.workspaces[w.ID] = *w
	return nil
}
func (a *capabilityAdapter) GetWorkspace(_ context.Context, id string) (*capability.Workspace, error) {
	w, ok := a.p.workspaces[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return &w, nil
}
func (a *capabilityAdapter) ListWorkspaces(_ context.Context) ([]*capability.Workspace, error) {
	out := make([]*capability.Workspace, 0, len(a.p.workspaces))
	for _, w := range a.p.workspaces {
		w := w
		out = append(out, &w)
	}
	return out, nil
}
func (a *capabilityAdapter) UpdateWorkspace(_ context.Context, w *capability.Workspace) error {
	a.p.workspaces[w.ID] = *w
	return nil
}
func (a *capabilityAdapter) DeleteWorkspace(_ context.Context, id string) error {
	delete(a.p.workspaces, id)
	return nil
}

func (a *capabilityAdapter) CreateProject(_ context.Context, p *capability.Project) error {
	a.p.projects[p.ID] = *p
	return nil
}
func (a *capabilityAdapter) GetProject(_ context.Context, id string) (*capability.Project, error) {
	p, ok := a.p.projects[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return &p, nil
}
func (a *capabilityAdapter) ListProjects(_ context.Context, workspaceID string) ([]*capability.Project, error) {
	out := make([]*capability.Project, 0)
	for _, p := range a.p.projects {
		if p.WorkspaceID == workspaceID {
			p := p
			out = append(out, &p)
		}
	}
	return out, nil
}
func (a *capabilityAdapter) UpdateProject(_ context.Context, p *capability.Project) error {
	a.p.projects[p.ID] = *p
	return nil
}
func (a *capabilityAdapter) DeleteProject(_ context.Context, id string) error {
	delete(a.p.projects, id)
	return nil
}

func (a *capabilityAdapter) CreateCapability(_ context.Context, c *capability.Capability) error {
	a.p.capabilities[c.ID] = *c
	return nil
}
func (a *capabilityAdapter) GetCapability(_ context.Context, id string) (*capability.Capability, error) {
	c, ok := a.p.capabilities[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return &c, nil
}
func (a *capabilityAdapter) ListCapabilities(_ context.Context, projectID string) ([]*capability.Capability, error) {
	out := make([]*capability.Capability, 0)
	for _, c := range a.p.capabilities {
		if c.ProjectID == projectID {
			c := c
			out = append(out, &c)
		}
	}
	return out, nil
}
func (a *capabilityAdapter) UpdateCapability(_ context.Context, c *capability.Capability) error {
	a.p.capabilities[c.ID] = *c
	return nil
}
func (a *capabilityAdapter) DeleteCapability(_ context.Context, id string) error {
	delete(a.p.capabilities, id)
	return nil
}
func (a *capabilityAdapter) UpdateSelfEvolveConfig(_ context.Context, id string, cfg capability.SelfEvolveConfig) error {
	c, ok := a.p.capabilities[id]
	if !ok {
		return errNotImplemented
	}
	c.SelfEvolve = cfg
	a.p.capabilities[id] = c
	return nil
}
func (a *capabilityAdapter) SetCapabilityContract(_ context.Context, _ string, _ *capability.CapabilityContract) error {
	return errNotImplemented
}
func (a *capabilityAdapter) GetCapabilityContract(_ context.Context, _ string) (*capability.CapabilityContract, error) {
	return nil, errNotImplemented
}
func (a *capabilityAdapter) GetCapabilityReputation(_ context.Context, _ string) (capability.Reputation, error) {
	return capability.Reputation{}, errNotImplemented
}
func (a *capabilityAdapter) CatalogSearch(_ context.Context, _ string, _ string, _ int) ([]*capability.Capability, error) {
	return nil, errNotImplemented
}

func (a *capabilityAdapter) CreateVersion(_ context.Context, v *capability.Version) error {
	a.p.versions[v.ID] = *v
	return nil
}
func (a *capabilityAdapter) GetVersion(_ context.Context, id string) (*capability.Version, error) {
	v, ok := a.p.versions[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return &v, nil
}
func (a *capabilityAdapter) ListVersions(_ context.Context, capabilityID string) ([]*capability.Version, error) {
	out := make([]*capability.Version, 0)
	for _, v := range a.p.versions {
		if v.CapabilityID == capabilityID {
			v := v
			out = append(out, &v)
		}
	}
	return out, nil
}
func (a *capabilityAdapter) GetLatestVersion(_ context.Context, capabilityID string) (*capability.Version, error) {
	var latest *capability.Version
	for _, v := range a.p.versions {
		if v.CapabilityID != capabilityID {
			continue
		}
		if latest == nil || v.Version > latest.Version {
			vv := v
			latest = &vv
		}
	}
	if latest == nil {
		return nil, store.ErrNotFound
	}
	return latest, nil
}
func (a *capabilityAdapter) GetVersionByNumber(_ context.Context, capabilityID string, version int) (*capability.Version, error) {
	for _, v := range a.p.versions {
		if v.CapabilityID == capabilityID && v.Version == version {
			vv := v
			return &vv, nil
		}
	}
	return nil, store.ErrNotFound
}

func (a *capabilityAdapter) CreateExecution(_ context.Context, e *capability.Execution) error {
	return errNotImplemented
}
func (a *capabilityAdapter) GetExecution(_ context.Context, _ string) (*capability.Execution, error) {
	return nil, errNotImplemented
}
func (a *capabilityAdapter) ListExecutions(_ context.Context, _ capability.ExecutionFilter) ([]*capability.Execution, error) {
	return nil, errNotImplemented
}

// usersAdapter implements store.Users. Every method is a stub
// that returns errNotImplemented; the contract tests only
// exercise wiring.
type usersAdapter struct{ p *InMemoryPostgres }

func (a *usersAdapter) CreateUser(_ context.Context, _ *models.User) error { return errNotImplemented }
func (a *usersAdapter) GetUser(_ context.Context, _ string) (*models.User, error) {
	return nil, errNotImplemented
}
func (a *usersAdapter) GetUserByEmail(_ context.Context, _ string) (*models.User, error) {
	return nil, errNotImplemented
}
func (a *usersAdapter) ListUsers(_ context.Context) ([]*models.User, error) {
	return nil, errNotImplemented
}
func (a *usersAdapter) UpdateUser(_ context.Context, _ *models.User) error { return errNotImplemented }
func (a *usersAdapter) DeleteUser(_ context.Context, _ string) error       { return errNotImplemented }
func (a *usersAdapter) BootstrapAdmin(_ context.Context, _ *models.User, _ *models.APIKey) error {
	return errNotImplemented
}

// apiKeysAdapter implements store.APIKeys. Stub.
type apiKeysAdapter struct{ p *InMemoryPostgres }

func (a *apiKeysAdapter) CreateAPIKey(_ context.Context, _ *models.APIKey) error {
	return errNotImplemented
}
func (a *apiKeysAdapter) GetAPIKeyByHash(_ context.Context, _ string) (*models.APIKey, error) {
	return nil, errNotImplemented
}
func (a *apiKeysAdapter) GetAPIKeyByID(_ context.Context, _ string) (*models.APIKey, error) {
	return nil, errNotImplemented
}
func (a *apiKeysAdapter) DeleteAPIKey(_ context.Context, _ string) error { return errNotImplemented }
func (a *apiKeysAdapter) ListAPIKeysByUser(_ context.Context, _ string) ([]*models.APIKey, error) {
	return nil, errNotImplemented
}
func (a *apiKeysAdapter) UpdateAPIKeyLastUsed(_ context.Context, _ string) error {
	return errNotImplemented
}

// auditAdapter implements store.Audit. Stub.
type auditAdapter struct{ p *InMemoryPostgres }

func (a *auditAdapter) AppendAudit(_ context.Context, _ *models.AuditEntry) error {
	return errNotImplemented
}
func (a *auditAdapter) ListAudit(_ context.Context, _ *models.AuditFilter) ([]*models.AuditEntry, error) {
	return nil, errNotImplemented
}
func (a *auditAdapter) ExportAudit(_ context.Context, _ *models.AuditFilter) ([]*models.AuditEntry, error) {
	return nil, errNotImplemented
}
func (a *auditAdapter) VerifyAuditChain(_ context.Context) (*store.AuditVerifyResult, error) {
	return &store.AuditVerifyResult{Ok: true}, nil
}

// providerKeysAdapter implements store.ProviderKeys. Stub.
type providerKeysAdapter struct{ p *InMemoryPostgres }

func (a *providerKeysAdapter) SaveProviderKey(_ context.Context, _ *models.ProviderKey) error {
	return errNotImplemented
}
func (a *providerKeysAdapter) GetProviderKey(_ context.Context, _ string) (*models.ProviderKey, error) {
	return nil, errNotImplemented
}
func (a *providerKeysAdapter) GetProviderKeyByName(_ context.Context, _ string, _ string) (*models.ProviderKey, error) {
	return nil, errNotImplemented
}
func (a *providerKeysAdapter) DeleteProviderKey(_ context.Context, _ string) error {
	return errNotImplemented
}
func (a *providerKeysAdapter) ListProviderKeys(_ context.Context) ([]*models.ProviderKey, error) {
	return nil, errNotImplemented
}

// alertingAdapter implements store.Alerting. Stub.
type alertingAdapter struct{ p *InMemoryPostgres }

func (a *alertingAdapter) SaveAlertRule(_ context.Context, _ *models.AlertRuleRecord) error {
	return errNotImplemented
}
func (a *alertingAdapter) GetAlertRule(_ context.Context, _ string) (*models.AlertRuleRecord, error) {
	return nil, errNotImplemented
}
func (a *alertingAdapter) DeleteAlertRule(_ context.Context, _ string) error {
	return errNotImplemented
}
func (a *alertingAdapter) ListAlertRules(_ context.Context) ([]*models.AlertRuleRecord, error) {
	return nil, errNotImplemented
}
func (a *alertingAdapter) SaveAlert(_ context.Context, _ *models.AlertRecord) error {
	return errNotImplemented
}
func (a *alertingAdapter) GetAlert(_ context.Context, _ string) (*models.AlertRecord, error) {
	return nil, errNotImplemented
}
func (a *alertingAdapter) UpdateAlert(_ context.Context, _ *models.AlertRecord) error {
	return errNotImplemented
}
func (a *alertingAdapter) ListAlerts(_ context.Context, _ string) ([]*models.AlertRecord, error) {
	return nil, errNotImplemented
}
func (a *alertingAdapter) SaveNotificationGroup(_ context.Context, _ *models.NotificationGroupRecord) error {
	return errNotImplemented
}
func (a *alertingAdapter) GetNotificationGroup(_ context.Context, _ string) (*models.NotificationGroupRecord, error) {
	return nil, errNotImplemented
}
func (a *alertingAdapter) DeleteNotificationGroup(_ context.Context, _ string) error {
	return errNotImplemented
}
func (a *alertingAdapter) ListNotificationGroups(_ context.Context) ([]*models.NotificationGroupRecord, error) {
	return nil, errNotImplemented
}
func (a *alertingAdapter) GetChannelsForAlertRule(_ context.Context, _ string) ([]string, error) {
	return nil, errNotImplemented
}
func (a *alertingAdapter) LinkRuleToGroup(_ context.Context, _ string, _ string) error {
	return errNotImplemented
}
func (a *alertingAdapter) UnlinkRuleFromGroup(_ context.Context, _ string, _ string) error {
	return errNotImplemented
}

// webhooksAdapter implements store.Webhooks. Stub.
type webhooksAdapter struct{ p *InMemoryPostgres }

func (a *webhooksAdapter) SaveWebhookEndpoint(_ context.Context, _ *models.WebhookEndpointRecord) error {
	return errNotImplemented
}
func (a *webhooksAdapter) GetWebhookEndpoint(_ context.Context, _ string) (*models.WebhookEndpointRecord, error) {
	return nil, errNotImplemented
}
func (a *webhooksAdapter) DeleteWebhookEndpoint(_ context.Context, _ string) error {
	return errNotImplemented
}
func (a *webhooksAdapter) ListWebhookEndpoints(_ context.Context) ([]*models.WebhookEndpointRecord, error) {
	return nil, errNotImplemented
}

// vaultStateAdapter implements store.VaultState. Stub.
type vaultStateAdapter struct{ p *InMemoryPostgres }

func (a *vaultStateAdapter) GetVaultState(_ context.Context) (*models.VaultState, error) {
	return nil, errNotImplemented
}
func (a *vaultStateAdapter) SaveVaultState(_ context.Context, _ *models.VaultState) error {
	return errNotImplemented
}

// wsStateAdapter implements store.WSState. Stub.
type wsStateAdapter struct{ p *InMemoryPostgres }

func (a *wsStateAdapter) GetWSNextID(_ context.Context) (int64, error) { return 0, errNotImplemented }
func (a *wsStateAdapter) SetWSNextID(_ context.Context, _ int64) error { return errNotImplemented }

// enforcerStateAdapter implements store.EnforcerState. Stub.
type enforcerStateAdapter struct{ p *InMemoryPostgres }

func (a *enforcerStateAdapter) GetEnforcerBudget(_ context.Context, _ string) ([]byte, error) {
	return nil, errNotImplemented
}
func (a *enforcerStateAdapter) SetEnforcerBudget(_ context.Context, _ string, _ []byte) error {
	return errNotImplemented
}
func (a *enforcerStateAdapter) GetEnforcerQuota(_ context.Context, _ string) ([]byte, error) {
	return nil, errNotImplemented
}
func (a *enforcerStateAdapter) SetEnforcerQuota(_ context.Context, _ string, _ []byte) error {
	return errNotImplemented
}

// releaseAdapter implements release.Repository.
type releaseAdapter struct{ p *InMemoryPostgres }

func (a *releaseAdapter) CreateRelease(_ context.Context, r *release.Release) error {
	a.p.releases[r.ID] = *r
	return nil
}
func (a *releaseAdapter) GetRelease(_ context.Context, id string) (*release.Release, error) {
	r, ok := a.p.releases[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return &r, nil
}
func (a *releaseAdapter) ListReleasesForCapability(_ context.Context, capabilityID string) ([]*release.Release, error) {
	out := make([]*release.Release, 0)
	for _, r := range a.p.releases {
		if r.CapabilityID == capabilityID {
			rr := r
			out = append(out, &rr)
		}
	}
	return out, nil
}
func (a *releaseAdapter) ListActiveReleasesForEnvironment(_ context.Context, _ release.Environment) ([]*release.Release, error) {
	return nil, errNotImplemented
}
func (a *releaseAdapter) UpdateRelease(_ context.Context, r *release.Release) error {
	a.p.releases[r.ID] = *r
	return nil
}
func (a *releaseAdapter) DeleteRelease(_ context.Context, id string) error {
	delete(a.p.releases, id)
	return nil
}
func (a *releaseAdapter) ActivateAtomic(_ context.Context, _, _ *release.Release) error {
	return errNotImplemented
}
func (a *releaseAdapter) GetActiveReleaseID(_ context.Context, capabilityID string) (string, error) {
	var latest string
	var latestAt time.Time
	for _, rel := range a.p.releases {
		if rel.CapabilityID != capabilityID {
			continue
		}
		if rel.Status != release.StatusActive {
			continue
		}
		if rel.ActivatedAt == nil {
			continue
		}
		if rel.ActivatedAt.After(latestAt) {
			latestAt = *rel.ActivatedAt
			latest = rel.ID
		}
	}
	return latest, nil
}

// approvalAdapter implements approval.Repository.
type approvalAdapter struct{ p *InMemoryPostgres }

func (a *approvalAdapter) CreateApproval(_ context.Context, ap *approval.Approval) error {
	a.p.approvals[ap.ReleaseID] = *ap
	return nil
}
func (a *approvalAdapter) GetApproval(_ context.Context, releaseID string) (*approval.Approval, error) {
	ap, ok := a.p.approvals[releaseID]
	if !ok {
		return nil, store.ErrNotFound
	}
	return &ap, nil
}
func (a *approvalAdapter) UpdateApproval(_ context.Context, ap *approval.Approval) error {
	a.p.approvals[ap.ReleaseID] = *ap
	return nil
}
func (a *approvalAdapter) ListApprovalsForRelease(_ context.Context, releaseID string) ([]*approval.Approval, error) {
	if ap, ok := a.p.approvals[releaseID]; ok {
		return []*approval.Approval{&ap}, nil
	}
	return nil, nil
}

// harnessAdapter implements harness.Repository.
type harnessAdapter struct{ p *InMemoryPostgres }

func (a *harnessAdapter) CreateDataset(_ context.Context, d *harness.Dataset) error {
	a.p.datasets[d.ID] = d
	return nil
}
func (a *harnessAdapter) GetDataset(_ context.Context, id string) (*harness.Dataset, error) {
	d, ok := a.p.datasets[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return d, nil
}
func (a *harnessAdapter) ListDatasetsForCapability(_ context.Context, capabilityID string) ([]*harness.Dataset, error) {
	out := make([]*harness.Dataset, 0)
	for _, d := range a.p.datasets {
		if d.CapabilityID == capabilityID {
			out = append(out, d)
		}
	}
	return out, nil
}
func (a *harnessAdapter) DeleteDataset(_ context.Context, id string) error {
	delete(a.p.datasets, id)
	return nil
}
func (a *harnessAdapter) UpsertDatasetCases(_ context.Context, datasetID string, cases []harness.DatasetCase) error {
	a.p.cases[datasetID] = cases
	return nil
}
func (a *harnessAdapter) ListDatasetCases(_ context.Context, datasetID string) ([]harness.DatasetCase, error) {
	return a.p.cases[datasetID], nil
}
func (a *harnessAdapter) CreatePrecondition(_ context.Context, p *harness.Precondition) error {
	a.p.preconditions[p.ID] = p
	return nil
}
func (a *harnessAdapter) GetPrecondition(_ context.Context, id string) (*harness.Precondition, error) {
	p, ok := a.p.preconditions[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return p, nil
}
func (a *harnessAdapter) ListPreconditionsForCapability(_ context.Context, capabilityID string) ([]*harness.Precondition, error) {
	out := make([]*harness.Precondition, 0)
	for _, p := range a.p.preconditions {
		if p.CapabilityID == capabilityID {
			out = append(out, p)
		}
	}
	return out, nil
}
func (a *harnessAdapter) UpdatePrecondition(_ context.Context, p *harness.Precondition) error {
	a.p.preconditions[p.ID] = p
	return nil
}
func (a *harnessAdapter) DeletePrecondition(_ context.Context, id string) error {
	delete(a.p.preconditions, id)
	return nil
}
func (a *harnessAdapter) CreateEvalRun(_ context.Context, r *harness.EvalRun) error {
	a.p.evalRuns[r.ID] = r
	return nil
}
func (a *harnessAdapter) UpdateEvalRun(_ context.Context, r *harness.EvalRun) error {
	a.p.evalRuns[r.ID] = r
	return nil
}
func (a *harnessAdapter) GetEvalRun(_ context.Context, id string) (*harness.EvalRun, error) {
	r, ok := a.p.evalRuns[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return r, nil
}
func (a *harnessAdapter) ListEvalRunsForRelease(_ context.Context, releaseID string) ([]*harness.EvalRun, error) {
	out := make([]*harness.EvalRun, 0)
	for _, r := range a.p.evalRuns {
		if r.ReleaseID == releaseID {
			out = append(out, r)
		}
	}
	return out, nil
}
func (a *harnessAdapter) GetActiveReleaseID(_ context.Context, capabilityID string) (string, error) {
	var latest string
	var latestAt time.Time
	for _, rel := range a.p.releases {
		if rel.CapabilityID != capabilityID || rel.Status != release.StatusActive || rel.ActivatedAt == nil {
			continue
		}
		if rel.ActivatedAt.After(latestAt) {
			latestAt = *rel.ActivatedAt
			latest = rel.ID
		}
	}
	return latest, nil
}
func (a *harnessAdapter) CreateEvalResults(_ context.Context, results []harness.EvalResult) error {
	for _, r := range results {
		a.p.evalResults[r.RunID] = append(a.p.evalResults[r.RunID], r)
	}
	return nil
}
func (a *harnessAdapter) CreateEvalResult(_ context.Context, r *harness.EvalResult) error {
	a.p.evalResults[r.RunID] = append(a.p.evalResults[r.RunID], *r)
	return nil
}
func (a *harnessAdapter) ListEvalResultsForRun(_ context.Context, runID string) ([]harness.EvalResult, error) {
	return a.p.evalResults[runID], nil
}
