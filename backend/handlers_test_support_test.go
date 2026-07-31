package backend

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"testing"

	"github.com/sachncs/promptsheon/backend/approval"
	"github.com/sachncs/promptsheon/backend/capability"
	"github.com/sachncs/promptsheon/backend/harness"
	"github.com/sachncs/promptsheon/backend/llm"
	"github.com/sachncs/promptsheon/backend/models"
	"github.com/sachncs/promptsheon/backend/release"
	"github.com/sachncs/promptsheon/backend/settings"
	"github.com/sachncs/promptsheon/backend/store"
	"github.com/sachncs/promptsheon/backend/vault"
	"github.com/sachncs/promptsheon/backend/errs"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type mockRepo struct {
	mu            sync.Mutex
	users         map[string]*models.User
	apiKeys       map[string]*models.APIKey
	apiKeysByHash map[string]*models.APIKey
	providerKeys  map[string]*models.ProviderKey
	auditEntries  []*models.AuditEntry
	workspaces    map[string]*capability.Workspace
	projects      map[string]*capability.Project
	capabilities  map[string]*capability.Capability
	versions      map[string]*capability.Version
	executions    map[string]*capability.Execution
	versionsByCap map[string][]*capability.Version
	releases      map[string]*release.Release
	releasesByCap map[string][]*release.Release
	approvals     map[string]*approval.Approval
	datasets      map[string]*harness.Dataset
	datasetCases  map[string][]harness.DatasetCase
	preconditions map[string]*harness.Precondition
	evalRuns      map[string]*harness.EvalRun
	evalResults   []harness.EvalResult
	contracts     map[string]*capability.CapabilityContract
	pingErr       error
	closeErr      error
}

func newRepositories(repo *mockRepo) *store.Repositories {
	return &store.Repositories{
		Users:                repo,
		APIKeys:              repo,
		Audit:                repo,
		ProviderKeys:         repo,
		Alerting:             repo,
		Webhooks:             repo,
		VaultState:           repo,
		WSState:              repo,
		EnforcerState:        repo,
		Settings:             repo,
		Lifecycle:            repo,
		CapabilityRepository: repo,
		ReleaseRepository:    repo,
		ApprovalRepository:   repo,
		HarnessRepository:    repo,
	}
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		users:         make(map[string]*models.User),
		apiKeys:       make(map[string]*models.APIKey),
		apiKeysByHash: make(map[string]*models.APIKey),
		providerKeys:  make(map[string]*models.ProviderKey),
		workspaces:    make(map[string]*capability.Workspace),
		projects:      make(map[string]*capability.Project),
		capabilities:  make(map[string]*capability.Capability),
		versions:      make(map[string]*capability.Version),
		executions:    make(map[string]*capability.Execution),
		versionsByCap: make(map[string][]*capability.Version),
		releases:      make(map[string]*release.Release),
		releasesByCap: make(map[string][]*release.Release),
		approvals:     make(map[string]*approval.Approval),
		datasets:      make(map[string]*harness.Dataset),
		datasetCases:  make(map[string][]harness.DatasetCase),
		contracts:     make(map[string]*capability.CapabilityContract),
		preconditions: make(map[string]*harness.Precondition),
		evalRuns:      make(map[string]*harness.EvalRun),
	}
}

func (m *mockRepo) Close() error                 { return m.closeErr }
func (m *mockRepo) Ping(_ context.Context) error { return m.pingErr }

// Settings (operator-tunable runtime config, A1).
func (m *mockRepo) GetSystemConfig(_ context.Context, _ string) (settings.CRDTRecord, error) {
	return settings.CRDTRecord{}, sql.ErrNoRows
}
func (m *mockRepo) SetSystemConfig(_ context.Context, _ settings.CRDTRecord) error {
	return nil
}
func (m *mockRepo) ListSystemConfig(_ context.Context) ([]settings.CRDTRecord, error) {
	return nil, nil
}
func (m *mockRepo) MergeSystemConfig(_ context.Context, _ string, _ []settings.CRDTRecord) error {
	return nil
}

// Users
func (m *mockRepo) CreateUser(_ context.Context, u *models.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[u.ID] = u
	return nil
}
func (m *mockRepo) BootstrapAdmin(_ context.Context, u *models.User, key *models.APIKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.users) > 0 {
		return errs.ErrorStoreConflict
	}
	m.users[u.ID] = u
	m.apiKeys[key.ID] = key
	return nil
}
func (m *mockRepo) GetUser(_ context.Context, id string) (*models.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return u, nil
}
func (m *mockRepo) GetUserByEmail(_ context.Context, email string) (*models.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, sql.ErrNoRows
}
func (m *mockRepo) ListUsers(_ context.Context) ([]*models.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	users := make([]*models.User, 0, len(m.users))
	for _, u := range m.users {
		users = append(users, u)
	}
	return users, nil
}
func (m *mockRepo) UpdateUser(_ context.Context, u *models.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[u.ID] = u
	return nil
}
func (m *mockRepo) DeleteUser(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users[id]; !ok {
		return sql.ErrNoRows
	}
	delete(m.users, id)
	return nil
}

// API Keys
func (m *mockRepo) CreateAPIKey(_ context.Context, key *models.APIKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.apiKeys[key.ID] = key
	m.apiKeysByHash[key.KeyHash] = key
	return nil
}
func (m *mockRepo) GetAPIKeyByHash(_ context.Context, keyHash string) (*models.APIKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key, ok := m.apiKeysByHash[keyHash]
	if !ok {
		return nil, nil
	}
	return key, nil
}
func (m *mockRepo) GetAPIKeyByID(_ context.Context, id string) (*models.APIKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key, ok := m.apiKeys[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return key, nil
}
func (m *mockRepo) DeleteAPIKey(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.apiKeys, id)
	for k, v := range m.apiKeysByHash {
		if v.ID == id {
			delete(m.apiKeysByHash, k)
			break
		}
	}
	return nil
}
func (m *mockRepo) ListAPIKeysByUser(_ context.Context, userID string) ([]*models.APIKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var keys []*models.APIKey
	for _, k := range m.apiKeys {
		if k.UserID == userID {
			keys = append(keys, k)
		}
	}
	return keys, nil
}
func (m *mockRepo) UpdateAPIKeyLastUsed(_ context.Context, _ string) error { return nil }

// Audit
func (m *mockRepo) AppendAudit(_ context.Context, entry *models.AuditEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.auditEntries = append(m.auditEntries, entry)
	return nil
}
func (m *mockRepo) ListAudit(_ context.Context, _ *models.AuditFilter) ([]*models.AuditEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.auditEntries, nil
}
func (m *mockRepo) ExportAudit(_ context.Context, _ *models.AuditFilter) ([]*models.AuditEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.auditEntries, nil
}
func (m *mockRepo) VerifyAuditChain(_ context.Context) (*store.AuditVerifyResult, error) {
	return &store.AuditVerifyResult{Ok: true}, nil
}

// Provider Keys
func (m *mockRepo) SaveProviderKey(_ context.Context, pk *models.ProviderKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providerKeys[pk.ID] = pk
	return nil
}
func (m *mockRepo) GetProviderKey(_ context.Context, id string) (*models.ProviderKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pk, ok := m.providerKeys[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return pk, nil
}
func (m *mockRepo) GetProviderKeyByName(_ context.Context, providerName, keyName string) (*models.ProviderKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, pk := range m.providerKeys {
		if pk.ProviderName == providerName && pk.KeyName == keyName {
			return pk, nil
		}
	}
	return nil, sql.ErrNoRows
}
func (m *mockRepo) DeleteProviderKey(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.providerKeys[id]; !ok {
		return sql.ErrNoRows
	}
	delete(m.providerKeys, id)
	return nil
}
func (m *mockRepo) ListProviderKeys(_ context.Context) ([]*models.ProviderKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]*models.ProviderKey, 0, len(m.providerKeys))
	for _, k := range m.providerKeys {
		keys = append(keys, k)
	}
	return keys, nil
}

// Alert Rules (unused in this mock - alerting manager is in-memory)
func (m *mockRepo) SaveAlertRule(_ context.Context, _ *models.AlertRuleRecord) error { return nil }
func (m *mockRepo) GetAlertRule(_ context.Context, _ string) (*models.AlertRuleRecord, error) {
	return nil, sql.ErrNoRows
}
func (m *mockRepo) DeleteAlertRule(_ context.Context, _ string) error { return nil }
func (m *mockRepo) ListAlertRules(_ context.Context) ([]*models.AlertRuleRecord, error) {
	return nil, nil
}
func (m *mockRepo) SaveAlert(_ context.Context, _ *models.AlertRecord) error { return nil }
func (m *mockRepo) GetAlert(_ context.Context, _ string) (*models.AlertRecord, error) {
	return nil, sql.ErrNoRows
}
func (m *mockRepo) UpdateAlert(_ context.Context, _ *models.AlertRecord) error { return nil }
func (m *mockRepo) ListAlerts(_ context.Context, _ string) ([]*models.AlertRecord, error) {
	return nil, nil
}
func (m *mockRepo) SaveNotificationGroup(_ context.Context, _ *models.NotificationGroupRecord) error {
	return nil
}
func (m *mockRepo) GetNotificationGroup(_ context.Context, _ string) (*models.NotificationGroupRecord, error) {
	return nil, sql.ErrNoRows
}
func (m *mockRepo) DeleteNotificationGroup(_ context.Context, _ string) error { return nil }
func (m *mockRepo) ListNotificationGroups(_ context.Context) ([]*models.NotificationGroupRecord, error) {
	return nil, nil
}
func (m *mockRepo) SaveWebhookEndpoint(_ context.Context, _ *models.WebhookEndpointRecord) error {
	return nil
}
func (m *mockRepo) GetWebhookEndpoint(_ context.Context, _ string) (*models.WebhookEndpointRecord, error) {
	return nil, sql.ErrNoRows
}
func (m *mockRepo) DeleteWebhookEndpoint(_ context.Context, _ string) error { return nil }
func (m *mockRepo) ListWebhookEndpoints(_ context.Context) ([]*models.WebhookEndpointRecord, error) {
	return nil, nil
}

// Capability repository methods
func (m *mockRepo) CreateWorkspace(_ context.Context, w *capability.Workspace) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.workspaces[w.ID] = w
	return nil
}
func (m *mockRepo) GetWorkspace(_ context.Context, id string) (*capability.Workspace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w, ok := m.workspaces[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return w, nil
}
func (m *mockRepo) ListWorkspaces(_ context.Context) ([]*capability.Workspace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ws := make([]*capability.Workspace, 0, len(m.workspaces))
	for _, w := range m.workspaces {
		ws = append(ws, w)
	}
	return ws, nil
}
func (m *mockRepo) UpdateWorkspace(_ context.Context, w *capability.Workspace) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.workspaces[w.ID] = w
	return nil
}
func (m *mockRepo) DeleteWorkspace(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.workspaces[id]; !ok {
		return sql.ErrNoRows
	}
	delete(m.workspaces, id)
	return nil
}
func (m *mockRepo) CreateProject(_ context.Context, p *capability.Project) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.projects[p.ID] = p
	return nil
}
func (m *mockRepo) GetProject(_ context.Context, id string) (*capability.Project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.projects[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return p, nil
}
func (m *mockRepo) ListProjects(_ context.Context, workspaceID string) ([]*capability.Project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var projs []*capability.Project
	for _, p := range m.projects {
		if p.WorkspaceID == workspaceID {
			projs = append(projs, p)
		}
	}
	return projs, nil
}
func (m *mockRepo) UpdateProject(_ context.Context, p *capability.Project) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.projects[p.ID] = p
	return nil
}
func (m *mockRepo) DeleteProject(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.projects[id]; !ok {
		return sql.ErrNoRows
	}
	delete(m.projects, id)
	return nil
}
func (m *mockRepo) CreateCapability(_ context.Context, c *capability.Capability) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.capabilities[c.ID] = c
	return nil
}
func (m *mockRepo) GetCapability(_ context.Context, id string) (*capability.Capability, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.capabilities[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return c, nil
}
func (m *mockRepo) ListCapabilities(_ context.Context, projectID string) ([]*capability.Capability, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var caps []*capability.Capability
	for _, c := range m.capabilities {
		if c.ProjectID == projectID {
			caps = append(caps, c)
		}
	}
	return caps, nil
}
func (m *mockRepo) UpdateCapability(_ context.Context, c *capability.Capability) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.capabilities[c.ID] = c
	return nil
}
func (m *mockRepo) UpdateSelfEvolveConfig(_ context.Context, id string, cfg capability.SelfEvolveConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.capabilities[id]
	if !ok {
		return errs.ErrorStoreNotFound
	}
	c.SelfEvolve = cfg
	m.capabilities[id] = c
	return nil
}
func (m *mockRepo) DeleteCapability(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.capabilities[id]; !ok {
		return sql.ErrNoRows
	}
	delete(m.capabilities, id)
	return nil
}
func (m *mockRepo) CreateVersion(_ context.Context, v *capability.Version) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.versions[v.ID] = v
	m.versionsByCap[v.CapabilityID] = append(m.versionsByCap[v.CapabilityID], v)
	return nil
}
func (m *mockRepo) GetVersion(_ context.Context, id string) (*capability.Version, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.versions[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return v, nil
}
func (m *mockRepo) ListVersions(_ context.Context, capabilityID string) ([]*capability.Version, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.versionsByCap[capabilityID], nil
}
func (m *mockRepo) GetVersionByNumber(_ context.Context, capabilityID string, version int) (*capability.Version, error) {
	if v, ok := m.versions[fmt.Sprintf("%s_%d", capabilityID, version)]; ok {
		return v, nil
	}
	return nil, errs.ErrorStoreNotFound
}

func (m *mockRepo) SetCapabilityContract(_ context.Context, capabilityID string, c *capability.CapabilityContract) error {
	if c == nil {
		delete(m.contracts, capabilityID)
		return nil
	}
	m.contracts[capabilityID] = c
	return nil
}

func (m *mockRepo) GetCapabilityContract(_ context.Context, capabilityID string) (*capability.CapabilityContract, error) {
	if c, ok := m.contracts[capabilityID]; ok {
		return c, nil
	}
	return nil, errs.ErrorStoreNotFound
}

func (m *mockRepo) GetCapabilityReputation(_ context.Context, capabilityID string) (capability.Reputation, error) {
	return capability.Reputation{CapabilityID: capabilityID, TrustScore: 0.5, EvalPassRate: 0.5, SLOAdherenceRate: 0.5, DecisionAdoptionRate: 0.5, SampleSize: 1}, nil
}

func (m *mockRepo) CatalogSearch(_ context.Context, _ string, query string, _ int) ([]*capability.Capability, error) {
	if query == "" {
		return nil, nil
	}
	return []*capability.Capability{{ID: "c1", Name: "matching"}}, nil
}

func (m *mockRepo) GetLatestVersion(_ context.Context, capabilityID string) (*capability.Version, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	versions := m.versionsByCap[capabilityID]
	if len(versions) == 0 {
		return nil, errs.ErrorStoreNotFound
	}
	latest := versions[0]
	for _, v := range versions[1:] {
		if v.Version > latest.Version {
			latest = v
		}
	}
	return latest, nil
}

func (m *mockRepo) CreateExecution(_ context.Context, e *capability.Execution) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executions[e.ID] = e
	return nil
}
func (m *mockRepo) GetExecution(_ context.Context, id string) (*capability.Execution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.executions[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return e, nil
}
func (m *mockRepo) ListExecutions(_ context.Context, filter capability.ExecutionFilter) ([]*capability.Execution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var execs []*capability.Execution
	for _, e := range m.executions {
		if e.CapabilityVersionID == filter.CapabilityVersionID {
			execs = append(execs, e)
		}
	}
	return execs, nil
}

// Releases
func (m *mockRepo) CreateRelease(_ context.Context, r *release.Release) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *r
	m.releases[r.ID] = &cp
	m.releasesByCap[r.CapabilityID] = append(m.releasesByCap[r.CapabilityID], &cp)
	return nil
}
func (m *mockRepo) GetRelease(_ context.Context, id string) (*release.Release, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.releases[id]
	if !ok {
		return nil, errs.ErrorReleaseNotFound
	}
	cp := *r
	return &cp, nil
}
func (m *mockRepo) ListReleasesForCapability(_ context.Context, capabilityID string) ([]*release.Release, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*release.Release, 0, len(m.releasesByCap[capabilityID]))
	for _, r := range m.releasesByCap[capabilityID] {
		cp := *r
		out = append(out, &cp)
	}
	return out, nil
}
func (m *mockRepo) ListActiveReleasesForEnvironment(_ context.Context, env release.Environment) ([]*release.Release, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*release.Release
	for _, r := range m.releases {
		if r.Environment == env && r.Status == release.StatusActive {
			cp := *r
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (m *mockRepo) UpdateRelease(_ context.Context, r *release.Release) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.releases[r.ID]; !ok {
		return errs.ErrorReleaseNotFound
	}
	cp := *r
	m.releases[r.ID] = &cp
	for i, existing := range m.releasesByCap[r.CapabilityID] {
		if existing.ID == r.ID {
			m.releasesByCap[r.CapabilityID][i] = &cp
			break
		}
	}
	return nil
}
func (m *mockRepo) DeleteRelease(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.releases, id)
	return nil
}
func (m *mockRepo) ActivateAtomic(_ context.Context, prior, next *release.Release) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if prior != nil {
		if _, ok := m.releases[prior.ID]; !ok {
			return errs.ErrorReleaseNotFound
		}
		cp := *prior
		m.releases[prior.ID] = &cp
	}
	if _, ok := m.releases[next.ID]; !ok {
		return errs.ErrorReleaseNotFound
	}
	cp := *next
	m.releases[next.ID] = &cp
	return nil
}

// Approvals
func (m *mockRepo) CreateApproval(_ context.Context, a *approval.Approval) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *a
	m.approvals[a.ReleaseID] = &cp
	return nil
}
func (m *mockRepo) GetApproval(_ context.Context, releaseID string) (*approval.Approval, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.approvals[releaseID]
	if !ok {
		return nil, errs.ErrorApprovalNotFound
	}
	cp := *a
	return &cp, nil
}
func (m *mockRepo) UpdateApproval(_ context.Context, a *approval.Approval) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.approvals[a.ReleaseID]; !ok {
		return errs.ErrorApprovalNotFound
	}
	cp := *a
	m.approvals[a.ReleaseID] = &cp
	return nil
}
func (m *mockRepo) DeleteApproval(_ context.Context, releaseID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.approvals, releaseID)
	return nil
}

// Datasets
func (m *mockRepo) CreateDataset(_ context.Context, d *harness.Dataset) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *d
	m.datasets[d.ID] = &cp
	return nil
}
func (m *mockRepo) GetDataset(_ context.Context, id string) (*harness.Dataset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.datasets[id]
	if !ok {
		return nil, errs.ErrorStoreNotFound
	}
	cp := *d
	return &cp, nil
}
func (m *mockRepo) ListDatasetsForCapability(_ context.Context, capabilityID string) ([]*harness.Dataset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*harness.Dataset
	for _, d := range m.datasets {
		if d.CapabilityID == capabilityID {
			cp := *d
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (m *mockRepo) DeleteDataset(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.datasets, id)
	return nil
}
func (m *mockRepo) UpsertDatasetCases(_ context.Context, datasetID string, cases []harness.DatasetCase) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cs := make([]harness.DatasetCase, len(cases))
	copy(cs, cases)
	m.datasetCases[datasetID] = cs
	return nil
}
func (m *mockRepo) ListDatasetCases(_ context.Context, datasetID string) ([]harness.DatasetCase, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cs := m.datasetCases[datasetID]
	out := make([]harness.DatasetCase, len(cs))
	copy(out, cs)
	return out, nil
}

// Preconditions
func (m *mockRepo) CreatePrecondition(_ context.Context, p *harness.Precondition) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *p
	m.preconditions[p.ID] = &cp
	return nil
}
func (m *mockRepo) ListPreconditionsForCapability(_ context.Context, capabilityID string) ([]*harness.Precondition, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*harness.Precondition
	for _, p := range m.preconditions {
		if p.CapabilityID == capabilityID {
			cp := *p
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (m *mockRepo) GetPrecondition(_ context.Context, id string) (*harness.Precondition, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.preconditions[id]
	if !ok {
		return nil, errs.ErrorStoreNotFound
	}
	cp := *p
	return &cp, nil
}
func (m *mockRepo) UpdatePrecondition(_ context.Context, p *harness.Precondition) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.preconditions[p.ID]; !ok {
		return errs.ErrorStoreNotFound
	}
	cp := *p
	m.preconditions[p.ID] = &cp
	return nil
}
func (m *mockRepo) DeletePrecondition(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.preconditions, id)
	return nil
}

// EvalRuns
func (m *mockRepo) CreateEvalRun(_ context.Context, r *harness.EvalRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *r
	m.evalRuns[r.ID] = &cp
	return nil
}
func (m *mockRepo) UpdateEvalRun(_ context.Context, r *harness.EvalRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *r
	m.evalRuns[r.ID] = &cp
	return nil
}
func (m *mockRepo) GetEvalRun(_ context.Context, id string) (*harness.EvalRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.evalRuns[id]
	if !ok {
		return nil, errs.ErrorStoreNotFound
	}
	cp := *r
	return &cp, nil
}
func (m *mockRepo) ListEvalRunsForRelease(_ context.Context, releaseID string) ([]*harness.EvalRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*harness.EvalRun
	for _, r := range m.evalRuns {
		if r.ReleaseID == releaseID {
			cp := *r
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (m *mockRepo) GetActiveReleaseID(_ context.Context, _ string) (string, error) {
	// Mock returns empty string: no active release; the
	// ContinuousEval loop is a no-op until production wiring
	// populates this.
	return "", nil
}
func (m *mockRepo) CreateEvalResults(_ context.Context, results []harness.EvalResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range results {
		cp := r
		m.evalResults = append(m.evalResults, cp)
	}
	return nil
}

func (m *mockRepo) CreateEvalResult(_ context.Context, r *harness.EvalResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *r
	m.evalResults = append(m.evalResults, cp)
	return nil
}

// GetChannelsForAlertRule is a no-op on the api test mock: the
// alert routing is exercised through the real alerting tests,
// not through the api surface. Returning an empty slice keeps
// the api routes that call TriggerAlert quiet.
func (m *mockRepo) GetChannelsForAlertRule(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

func (m *mockRepo) LinkRuleToGroup(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockRepo) UnlinkRuleFromGroup(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockRepo) GetVaultState(_ context.Context) (*models.VaultState, error) {
	return nil, nil
}

func (m *mockRepo) SaveVaultState(_ context.Context, _ *models.VaultState) error {
	return nil
}

func (m *mockRepo) GetWSNextID(_ context.Context) (int64, error) {
	return 0, nil
}

func (m *mockRepo) SetWSNextID(_ context.Context, _ int64) error {
	return nil
}

func (m *mockRepo) GetEnforcerBudget(_ context.Context, _ string) ([]byte, error) {
	return nil, nil
}

func (m *mockRepo) SetEnforcerBudget(_ context.Context, _ string, _ []byte) error {
	return nil
}

func (m *mockRepo) GetEnforcerQuota(_ context.Context, _ string) ([]byte, error) {
	return nil, nil
}

func (m *mockRepo) SetEnforcerQuota(_ context.Context, _ string, _ []byte) error {
	return nil
}

func (m *mockRepo) ListEvalResultsForRun(_ context.Context, runID string) ([]harness.EvalResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []harness.EvalResult
	for _, r := range m.evalResults {
		if r.RunID == runID {
			out = append(out, r)
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestServer(t *testing.T, opts ...Option) *Server {
	t.Helper()
	return newTestServerWithRepo(t, newMockRepo(), opts...)
}

func newTestServerWithRepo(t *testing.T, repo *mockRepo, opts ...Option) *Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError}))
	providers := llm.NewRegistry()
	providers.Configure("openai", llm.ProviderConfig{APIKey: "sk-test"})
	allOpts := make([]Option, 0, 2+len(opts))
	allOpts = append(allOpts, WithProviders(providers))
	allOpts = append(allOpts, opts...)
	return NewServer(newRepositories(repo), logger, allOpts...)
}

func newAuthTestServer(t *testing.T, repo *mockRepo, opts ...Option) *Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError}))
	allOpts := make([]Option, 0, 1+len(opts))
	allOpts = append(allOpts, WithAuth(repo))
	allOpts = append(allOpts, opts...)
	return NewServer(newRepositories(repo), logger, allOpts...)
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func readJSONBody(t *testing.T, body []byte, target any) {
	t.Helper()
	if err := json.Unmarshal(body, target); err != nil {
		t.Fatal(err)
	}
}

func newVault(t *testing.T) *vault.Vault {
	t.Helper()
	// 32 bytes = 64 hex chars
	v, err := vault.New("abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")
	if err != nil {
		t.Fatal(err)
	}
	return v
}
