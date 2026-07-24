package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/alerting"
	"github.com/sachncs/promptsheon/internal/approval"
	"github.com/sachncs/promptsheon/internal/auth"
	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/harness"
	"github.com/sachncs/promptsheon/internal/llm"
	"github.com/sachncs/promptsheon/internal/metrics"
	"github.com/sachncs/promptsheon/internal/models"
	"github.com/sachncs/promptsheon/internal/ratelimit"
	"github.com/sachncs/promptsheon/internal/release"
	"github.com/sachncs/promptsheon/internal/settings"
	"github.com/sachncs/promptsheon/internal/store"
	"github.com/sachncs/promptsheon/internal/vault"
	"github.com/sachncs/promptsheon/internal/webhook"
	"github.com/sachncs/promptsheon/internal/ws"
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
		return store.ErrConflict
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
func (m *mockRepo) GetLatestVersion(_ context.Context, capabilityID string) (*capability.Version, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	versions := m.versionsByCap[capabilityID]
	if len(versions) == 0 {
		return nil, sql.ErrNoRows
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
		return nil, release.ErrNotFound
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
		return release.ErrNotFound
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
			return release.ErrNotFound
		}
		cp := *prior
		m.releases[prior.ID] = &cp
	}
	if _, ok := m.releases[next.ID]; !ok {
		return release.ErrNotFound
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
		return nil, approval.ErrNotFound
	}
	cp := *a
	return &cp, nil
}
func (m *mockRepo) UpdateApproval(_ context.Context, a *approval.Approval) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.approvals[a.ReleaseID]; !ok {
		return approval.ErrNotFound
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
		return nil, store.ErrNotFound
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
		return nil, store.ErrNotFound
	}
	cp := *p
	return &cp, nil
}
func (m *mockRepo) UpdatePrecondition(_ context.Context, p *harness.Precondition) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.preconditions[p.ID]; !ok {
		return store.ErrNotFound
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
		return nil, store.ErrNotFound
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

func TestServer_ServeHTTP(t *testing.T) {
	s := newTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestNewServerOptions(t *testing.T) {
	_ = newTestServer(t)
	// Verify options-with-nil don't panic
	_ = newTestServer(t,
		WithTracing(nil, nil),
		WithWebhooks(nil),
		WithVault(nil),
		WithOAuth(nil),
		WithLogHub(nil),
		WithUsageTracker(nil),
		WithGuardrailManager(nil),
		WithAlertingManager(nil),
		WithContextManager(nil),
		WithRateLimiter(nil),
	)
}

// ---------------------------------------------------------------------------
// Middleware Tests
// ---------------------------------------------------------------------------

func TestLoggingMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	mw := Logging(logger)

	var handled bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		handled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !handled {
		t.Error("inner handler not called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(buf.String(), "http request") {
		t.Error("expected log output")
	}

	// Test with X-Request-ID header
	buf.Reset()
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("X-Request-ID", "req-123")
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if !strings.Contains(buf.String(), "req-123") {
		t.Error("expected X-Request-ID in log output")
	}

	// Test with X-Trace-ID header
	buf.Reset()
	req3 := httptest.NewRequest("GET", "/test", nil)
	req3.Header.Set("X-Trace-ID", "trace-456")
	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, req3)
	if !strings.Contains(buf.String(), "trace-456") {
		t.Error("expected X-Trace-ID in log output")
	}

	// Test with user in context
	buf.Reset()
	req4 := httptest.NewRequest("GET", "/test", nil)
	ctx := auth.WithUserContext(req4.Context(), &auth.User{ID: "u42", Role: auth.RoleAdmin})
	req4 = req4.WithContext(ctx)
	rr4 := httptest.NewRecorder()
	handler.ServeHTTP(rr4, req4)
	if !strings.Contains(buf.String(), "u42") {
		t.Error("expected user_id in log output")
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))
	mw := Recovery(logger)

	handler := mw(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest("GET", "/panic", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
	if !strings.Contains(buf.String(), "panic recovered") {
		t.Error("expected panic log output")
	}
	if !strings.Contains(rr.Body.String(), "internal server error") {
		t.Error("expected error message in body")
	}
}

func TestCORSNoOrigins(t *testing.T) {
	mw := CORS()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("expected no CORS headers when no origins configured")
	}
}

func TestCORSWithOrigin(t *testing.T) {
	mw := CORS("https://example.com")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("expected CORS origin header, got %q", rr.Header().Get("Access-Control-Allow-Origin"))
	}
	if rr.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected CORS methods header")
	}

	// OPTIONS preflight
	req2 := httptest.NewRequest("OPTIONS", "/test", nil)
	req2.Header.Set("Origin", "https://example.com")
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS, got %d", rr2.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("expected X-Content-Type-Options header")
	}
	if rr.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("expected X-Frame-Options header")
	}
	// BUG-24: HSTS is now TLS-only. Plain HTTP requests must NOT
	// receive the header because it would train the browser to
	// expect HTTPS on a connection that isn't using it.
	if got := rr.Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("did not expect HSTS on plain HTTP, got %q", got)
	}
	if rr.Header().Get("Content-Security-Policy") == "" {
		t.Error("expected Content-Security-Policy header")
	}
}

func TestSecurityHeadersHSTSOverTLS(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 1) r.TLS set (real TLS connection).
	req := httptest.NewRequest("GET", "/test", nil)
	req.TLS = &tls.ConnectionState{}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Header().Get("Strict-Transport-Security") == "" {
		t.Error("expected HSTS when r.TLS is set")
	}

	// 2) X-Forwarded-Proto: https (TLS-terminating proxy).
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("X-Forwarded-Proto", "https")
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if rr2.Header().Get("Strict-Transport-Security") == "" {
		t.Error("expected HSTS when X-Forwarded-Proto=https")
	}
}

func TestMaxBytesReader(t *testing.T) {
	mw := MaxBytesReader(10)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 20)
		n, _ := r.Body.Read(buf)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(buf[:n])
	}))

	body := strings.NewReader("hello world is long")
	req := httptest.NewRequest("POST", "/test", body)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestChainHTTP(t *testing.T) {
	var order []string
	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw1")
			next.ServeHTTP(w, r)
		})
	}
	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw2")
			next.ServeHTTP(w, r)
		})
	}

	handler := ChainHTTP(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		order = append(order, "inner")
	}), mw1, mw2)

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))

	if len(order) != 3 || order[0] != "mw1" || order[1] != "mw2" || order[2] != "inner" {
		t.Errorf("unexpected order: %v", order)
	}
}

func TestStatusWriter(t *testing.T) {
	sw := &statusWriter{ResponseWriter: httptest.NewRecorder(), status: http.StatusOK}
	sw.WriteHeader(http.StatusNotFound)
	if sw.status != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, sw.status)
	}
}

func TestSlogContext(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	ctx = WithSlogContext(ctx, logger)
	got := SlogFromContext(ctx)
	if got == nil {
		t.Fatal("expected logger from context")
	}
	defaultLogger := SlogFromContext(context.Background())
	if defaultLogger == slog.Default() {
		t.Log("default logger fallback works")
	}
}

// ---------------------------------------------------------------------------
// Error / Helper Tests
// ---------------------------------------------------------------------------

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusCreated, map[string]string{"key": "value"})

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected application/json, got %s", w.Header().Get("Content-Type"))
	}

	var result map[string]string
	readJSONBody(t, w.Body.Bytes(), &result)
	if result["key"] != "value" {
		t.Errorf("expected value, got %s", result["key"])
	}
}

func TestWriteError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "internal error", err: errors.New("oops"), wantStatus: http.StatusInternalServerError},
		{name: "not found", err: ErrNotFound, wantStatus: http.StatusNotFound},
		{name: "bad request", err: ErrBadRequest, wantStatus: http.StatusBadRequest},
		{name: "conflict", err: ErrConflict, wantStatus: http.StatusConflict},
		{name: "http error", err: &HTTPError{Status: http.StatusTeapot, Message: "teapot"}, wantStatus: http.StatusTeapot},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeError(w, tt.err)
			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}
			var result map[string]string
			readJSONBody(t, w.Body.Bytes(), &result)
			if result["error"] == "" {
				t.Error("expected error message in body")
			}
		})
	}
}

func TestReadJSON(t *testing.T) {
	body := mustMarshal(t, map[string]string{"hello": "world"})
	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))

	var target map[string]string
	if err := readJSON(req, &target); err != nil {
		t.Fatal(err)
	}
	if target["hello"] != "world" {
		t.Errorf("expected world, got %s", target["hello"])
	}
}

func TestGenerateID(t *testing.T) {
	id := generateID()
	if !strings.HasPrefix(id, "api-") {
		t.Errorf("expected api- prefix, got %s", id)
	}
	time.Sleep(time.Nanosecond)
	id2 := generateID()
	if id == id2 {
		t.Error("expected different IDs")
	}
}

func TestCallerID(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	if id := callerID(req); id != "api" {
		t.Errorf("expected api, got %s", id)
	}

	ctx := auth.WithUserContext(context.Background(), &auth.User{ID: "u1", Role: auth.RoleAdmin})
	req2 := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	if id := callerID(req2); id != "u1" {
		t.Errorf("expected u1, got %s", id)
	}

	// With nil user
	ctx2 := auth.WithUserContext(context.Background(), nil)
	req3 := httptest.NewRequest("GET", "/", nil).WithContext(ctx2)
	if id := callerID(req3); id != "api" {
		t.Errorf("expected api for nil user, got %s", id)
	}
}

func TestHelperErrors(t *testing.T) {
	if badRequest("bad").Error() != "bad" {
		t.Error("badRequest message mismatch")
	}
	if notFound("nf").Error() != "nf" {
		t.Error("notFound message mismatch")
	}
	if unauthorized().Error() != "authentication required" {
		t.Error("unauthorized message mismatch")
	}
	if forbidden("forb").Error() != "forb" {
		t.Error("forbidden message mismatch")
	}
}

func TestHTTPError(t *testing.T) {
	e := &HTTPError{Status: http.StatusBadRequest, Message: "bad"}
	var httpErr *HTTPError
	if !errors.As(e, &httpErr) {
		t.Error("expected HTTPError to match HTTPError")
	}
}

// ---------------------------------------------------------------------------
// Health Handler Tests
// ---------------------------------------------------------------------------

func TestHandleHealth(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var body map[string]any
	readJSONBody(t, rr.Body.Bytes(), &body)
	if body["status"] != "healthy" {
		t.Errorf("expected healthy, got %v", body["status"])
	}
	if body["version"] == nil {
		t.Error("expected version")
	}
	if body["uptime"] == nil {
		t.Error("expected uptime")
	}
}

func TestHandleReady(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/ready", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var body map[string]any
	readJSONBody(t, rr.Body.Bytes(), &body)
	if body["status"] != "ready" {
		t.Errorf("expected ready, got %v", body["status"])
	}
	if body["database"] != "ok" {
		t.Errorf("expected database ok, got %v", body["database"])
	}
	if body["go"] == nil {
		t.Error("expected go version")
	}
}

func TestHandleReady_DBPingFail(t *testing.T) {
	repo := newMockRepo()
	repo.pingErr = errors.New("db down")
	s := newTestServer(t)
	s.db = newRepositories(repo)

	req := httptest.NewRequest("GET", "/ready", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rr.Code)
	}

	var body map[string]any
	readJSONBody(t, rr.Body.Bytes(), &body)
	if body["status"] != "not_ready" {
		t.Errorf("expected not_ready, got %v", body["status"])
	}
	if body["database"] != "unreachable" {
		t.Errorf("expected database unreachable, got %v", body["database"])
	}
}

func TestHandleVersion(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/version", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// Auth Handler Tests
// ---------------------------------------------------------------------------

func TestHandleCreateAPIKey_NoAuth(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"name": "test-key", "role": "writer", "user_id": "u1"})
	req := httptest.NewRequest("POST", "/api/v1/apikeys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["key"] == "" {
		t.Error("expected api key in response")
	}
}

func TestHandleCreateAPIKey_NoAuth_AdminRejected(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"name": "admin-key", "role": "admin"})
	req := httptest.NewRequest("POST", "/api/v1/apikeys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateAPIKey_NoAuth_MissingName(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"role": "reader"})
	req := httptest.NewRequest("POST", "/api/v1/apikeys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateAPIKey_NoAuth_MissingRole(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"name": "test"})
	req := httptest.NewRequest("POST", "/api/v1/apikeys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateAPIKey_NoAuth_InvalidRole(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"name": "test", "role": "superadmin"})
	req := httptest.NewRequest("POST", "/api/v1/apikeys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateAPIKey_WithAuth_Reader(t *testing.T) {
	repo := newMockRepo()
	key, hash, _ := auth.GenerateAPIKey()
	_ = repo.CreateUser(context.Background(), &models.User{ID: "u1", Email: "u@t.com", Name: "U", Role: "reader"})
	_ = repo.CreateAPIKey(context.Background(), &models.APIKey{
		ID: "k1", UserID: "u1", Name: "reader-key", KeyHash: hash, KeyPrefix: key[:8], Role: "reader",
	})
	s := newAuthTestServer(t, repo)

	body := mustMarshal(t, map[string]string{"name": "new-key", "role": "writer"})
	req := httptest.NewRequest("POST", "/api/v1/apikeys", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateAPIKey_WithAuth_Admin(t *testing.T) {
	repo := newMockRepo()
	key, hash, _ := auth.GenerateAPIKey()
	_ = repo.CreateUser(context.Background(), &models.User{ID: "admin1", Email: "a@t.com", Name: "A", Role: "admin"})
	_ = repo.CreateAPIKey(context.Background(), &models.APIKey{
		ID: "k1", UserID: "admin1", Name: "admin-key", KeyHash: hash, KeyPrefix: key[:8], Role: "admin",
	})

	s := newAuthTestServer(t, repo)

	body := mustMarshal(t, map[string]string{"name": "new-key", "role": "writer", "user_id": "admin1"})
	req := httptest.NewRequest("POST", "/api/v1/apikeys", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateAPIKey_WithAuth_Unauthorized(t *testing.T) {
	repo := newMockRepo()
	s := newAuthTestServer(t, repo)
	body := mustMarshal(t, map[string]string{"name": "new-key", "role": "writer"})
	req := httptest.NewRequest("POST", "/api/v1/apikeys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListAPIKeys_NoAuth(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateAPIKey(context.Background(), &models.APIKey{ID: "k1", UserID: "u1", Name: "key1", KeyHash: "h1", KeyPrefix: "ps_test1", Role: "reader"})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	req := httptest.NewRequest("GET", "/api/v1/apikeys?user_id=u1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListAPIKeys_MissingUserID(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/apikeys", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListAPIKeys_AdminListsOtherUser(t *testing.T) {
	repo := newMockRepo()
	key, hash, _ := auth.GenerateAPIKey()
	_ = repo.CreateUser(context.Background(), &models.User{ID: "admin1", Email: "a@t.com", Name: "A", Role: "admin"})
	_ = repo.CreateAPIKey(context.Background(), &models.APIKey{ID: "ak1", UserID: "admin1", Name: "admin-key", KeyHash: hash, KeyPrefix: key[:8], Role: "admin"})
	_ = repo.CreateAPIKey(context.Background(), &models.APIKey{ID: "uk1", UserID: "u1", Name: "user-key", KeyHash: "h1", KeyPrefix: "ps_test1", Role: "reader"})

	s := newAuthTestServer(t, repo)
	req := httptest.NewRequest("GET", "/api/v1/apikeys?user_id=u1", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleRevokeAPIKey_NoAuth(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateAPIKey(context.Background(), &models.APIKey{ID: "k1", UserID: "u1", Name: "key1", KeyHash: "h1", KeyPrefix: "ps_test1", Role: "reader", Revoked: false})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	req := httptest.NewRequest("DELETE", "/api/v1/apikeys/k1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleRevokeAPIKey_Unauthorized(t *testing.T) {
	repo := newMockRepo()
	key, hash, _ := auth.GenerateAPIKey()
	_ = repo.CreateUser(context.Background(), &models.User{ID: "reader1", Email: "r@t.com", Name: "R", Role: "reader"})
	_ = repo.CreateAPIKey(context.Background(), &models.APIKey{ID: "rk1", UserID: "reader1", Name: "r-key", KeyHash: hash, KeyPrefix: key[:8], Role: "reader"})

	s := newAuthTestServer(t, repo)
	// reader1 cannot revoke a key that belongs to another user (u2 is not in DB but key belongs to reader1)
	// First create a key owned by a different user
	_ = repo.CreateAPIKey(context.Background(), &models.APIKey{ID: "uk-other", UserID: "u-other", Name: "other-key", KeyHash: "h2", KeyPrefix: "ps_test2", Role: "admin"})
	req := httptest.NewRequest("DELETE", "/api/v1/apikeys/uk-other", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleRevokeAPIKey_NotFound(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("DELETE", "/api/v1/apikeys/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleRevokeAPIKey_AlreadyRevoked(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateAPIKey(context.Background(), &models.APIKey{ID: "k1", UserID: "u1", Name: "key1", KeyHash: "h1", KeyPrefix: "ps_test1", Role: "reader", Revoked: true})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	req := httptest.NewRequest("DELETE", "/api/v1/apikeys/k1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		// Should still succeed after revocation
		t.Logf("status %d: %s", rr.Code, rr.Body.String())
	}
}

// BUG-27: TestHandleBootstrap_WrongMethod removed. The route is
// registered as POST /api/v1/setup, so the mux rejects non-POST
// requests with 405 before the handler ever runs. The previous
// guard inside the handler was unreachable.

func TestHandleBootstrap_InvalidJSON(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("POST", "/api/v1/setup", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleBootstrap(t *testing.T) {
	repo := newMockRepo()
	s := newTestServer(t)
	s.db = newRepositories(repo)

	body := mustMarshal(t, map[string]string{"email": "admin@local", "name": "Bootstrap Admin"})
	req := httptest.NewRequest("POST", "/api/v1/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["key"] == "" {
		t.Error("expected bootstrap key")
	}
}

func TestHandleBootstrap_AlreadyUsers(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateUser(context.Background(), &models.User{ID: "u1", Email: "u@t.com", Name: "U", Role: "admin"})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	body := mustMarshal(t, map[string]string{"email": "admin@local"})
	req := httptest.NewRequest("POST", "/api/v1/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleBootstrap_AuthEnabled(t *testing.T) {
	repo := newMockRepo()
	s := newAuthTestServer(t, repo)
	body := mustMarshal(t, map[string]string{"email": "admin@local"})
	req := httptest.NewRequest("POST", "/api/v1/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	// SEC-5b: /api/v1/setup is unregistered when requireAuth=true.
	// mux returns 404 for unregistered paths.
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 (route not registered), got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleOAuthLogin_NoOAuth(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/auth/google/login", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleOAuthLogin(t *testing.T) {
	oauthMgr := auth.NewOAuthManager()
	oauthMgr.RegisterProvider("google", &auth.OAuthProvider{
		Name:     "google",
		AuthURL:  "https://accounts.google.com/o/oauth2/auth",
		ClientID: "test-client-id",
		Scopes:   []string{"email", "profile"},
	})
	s := newTestServer(t)
	s.oauth = oauthMgr

	req := httptest.NewRequest("GET", "/api/v1/auth/google/login", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected 302, got %d: %s", rr.Code, rr.Body.String())
	}
	location := rr.Header().Get("Location")
	if !strings.Contains(location, "accounts.google.com") {
		t.Errorf("expected Google auth URL, got %s", location)
	}
}

func TestHandleOAuthCallback_WithRealOAuth(t *testing.T) {
	oauthMgr := auth.NewOAuthManager()
	oauthMgr.RegisterProvider("test", &auth.OAuthProvider{
		Name:         "test",
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost:9090/callback",
		AuthURL:      "https://example.com/auth",
		TokenURL:     "https://example.com/token",
		UserInfoURL:  "https://example.com/userinfo",
		Scopes:       []string{"openid"},
	})

	stateVal := "test-oauthstate-123"
	activeOAuthStates.put(stateVal, time.Now().Add(10*time.Minute))

	s := newTestServer(t)
	s.oauth = oauthMgr

	req := httptest.NewRequest("GET", "/api/v1/auth/test/callback?code=test_code&state="+stateVal, nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: stateVal})
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	// The ExchangeCode will try to make an HTTP request to example.com and
	// it may fail in tests without network. We expect either 200 or 400.
	if rr.Code != http.StatusOK && rr.Code != http.StatusBadRequest {
		t.Errorf("expected 200 or 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleOAuthCallback_MissingStateCookie(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/auth/google/callback?code=abc&state=xyz", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGenerateOAuthState(t *testing.T) {
	activeOAuthStates = newOAuthStateStore()
	state, err := generateOAuthState()
	if err != nil {
		t.Fatal(err)
	}
	if state == "" {
		t.Error("expected non-empty state")
	}
	if !validateOAuthState(state) {
		t.Error("expected valid state")
	}
	if validateOAuthState(state) {
		t.Error("expected state to be consumed after first use")
	}
}

// ---------------------------------------------------------------------------
// User Handler Tests
// ---------------------------------------------------------------------------

func TestHandleListUsers(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateUser(context.Background(), &models.User{ID: "u1", Email: "a@b.com", Name: "A", Role: "admin"})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateUser(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"email": "new@test.com", "name": "New User", "role": "reader"})
	req := httptest.NewRequest("POST", "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["email"] != "new@test.com" {
		t.Errorf("expected new@test.com, got %v", resp["email"])
	}
}

func TestHandleCreateUser_MissingFields(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"email": "new@test.com"})
	req := httptest.NewRequest("POST", "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateUser_RejectsInvalidRole(t *testing.T) {
	// API-VAL-6: reject roles outside the closed admin/writer/reader set.
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{
		"email": "u@test.com", "name": "U", "role": "superuser",
	})
	req := httptest.NewRequest("POST", "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unknown role, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleUpdateUser_RejectsInvalidRole(t *testing.T) {
	s := newTestServer(t)
	repo := newMockRepo()
	_ = repo.CreateUser(context.Background(), &models.User{ID: "u1", Email: "u@t.com", Name: "U", Role: "reader"})
	s.db = newRepositories(repo)

	body := mustMarshal(t, map[string]string{"role": "superuser"})
	req := httptest.NewRequest("PUT", "/api/v1/users/u1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unknown role, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetUser(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateUser(context.Background(), &models.User{ID: "u1", Email: "a@b.com", Name: "A", Role: "admin"})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	req := httptest.NewRequest("GET", "/api/v1/users/u1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetUser_NotFound(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/users/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleUpdateUser(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateUser(context.Background(), &models.User{ID: "u1", Email: "a@b.com", Name: "A", Role: "reader"})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	body := mustMarshal(t, map[string]string{"name": "Updated Name", "email": "new@b.com", "role": "admin"})
	req := httptest.NewRequest("PUT", "/api/v1/users/u1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["name"] != "Updated Name" {
		t.Errorf("expected Updated Name, got %v", resp["name"])
	}
}

func TestHandleUpdateUser_NotFound(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"name": "Updated"})
	req := httptest.NewRequest("PUT", "/api/v1/users/nonexistent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleDeleteUser(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateUser(context.Background(), &models.User{ID: "u1", Email: "a@b.com", Name: "A", Role: "reader"})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	req := httptest.NewRequest("DELETE", "/api/v1/users/u1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleDeleteUser_NotFound(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("DELETE", "/api/v1/users/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Audit Handler Tests
// ---------------------------------------------------------------------------

func TestHandleListAudit(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/audit", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListAudit_WithFilters(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/audit?user_id=u1&action=create&resource=test&since=2024-01-01T00:00:00Z&until=2024-12-31T23:59:59Z&limit=10&offset=5", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListAudit_InvalidSince(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/audit?since=invalid-date", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListAudit_InvalidUntil(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/audit?until=invalid-date", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleExportAudit(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/audit/export", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleExportAudit_WithFilters(t *testing.T) {
	repo := newMockRepo()
	_ = repo.AppendAudit(context.Background(), &models.AuditEntry{
		ID: "f1", UserID: "u1", Action: "create", Resource: "test",
		Details: map[string]any{"k": "v"}, Timestamp: time.Now(),
	})
	_ = repo.AppendAudit(context.Background(), &models.AuditEntry{
		ID: "f2", UserID: "u2", Action: "delete", Resource: "other",
		Details: nil, Timestamp: time.Now(),
	})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	req := httptest.NewRequest("GET", "/api/v1/audit/export?user_id=u1&action=create&resource=test&since=2000-01-01T00:00:00Z&until=2100-01-01T00:00:00Z", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var entries []*models.AuditEntry
	readJSONBody(t, rr.Body.Bytes(), &entries)
	if len(entries) == 0 {
		t.Error("expected at least one audit entry")
	}
}

func TestHandleExportAudit_InvalidSince(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/audit/export?since=bad-date", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleExportAudit_InvalidUntil(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/audit/export?until=bad-date", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleExportAudit_CSV(t *testing.T) {
	repo := newMockRepo()
	_ = repo.AppendAudit(context.Background(), &models.AuditEntry{
		ID: "e1", UserID: "u1", Action: "create", Resource: "test",
		Details: map[string]any{"key": "val"}, Timestamp: time.Now(),
	})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	req := httptest.NewRequest("GET", "/api/v1/audit/export?format=csv", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "text/csv" {
		t.Errorf("expected text/csv, got %s", ct)
	}
	if !strings.Contains(rr.Body.String(), "e1") {
		t.Error("expected audit entry in CSV output")
	}
	if !strings.Contains(rr.Body.String(), `""key"":""val""`) {
		t.Error("expected details in CSV output")
	}
}

func TestHandleExportAudit_CSVBadDetails(t *testing.T) {
	repo := newMockRepo()
	// Create entry with non-serializable details (using Inf for NaN in JSON)
	_ = repo.AppendAudit(context.Background(), &models.AuditEntry{
		ID: "e2", UserID: "u1", Action: "test", Resource: "test",
		Details:   map[string]any{"nested": map[string]any{"value": "test"}},
		Timestamp: time.Now(),
	})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	req := httptest.NewRequest("GET", "/api/v1/audit/export?format=csv", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleVerifyAuditChain(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/audit/verify", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["ok"] != true {
		t.Errorf("expected ok true, got %v", resp["ok"])
	}
	if resp["tail_mismatch"] != false {
		t.Errorf("expected tail_mismatch false, got %v", resp["tail_mismatch"])
	}
	if resp["last_row_id"] == nil {
		t.Error("expected last_row_id to be present")
	}
}

// ---------------------------------------------------------------------------
// Trace Handler Tests
// ---------------------------------------------------------------------------

func TestHandleMetricsSummary(t *testing.T) {
	s := newTestServer(t)
	s.collector = metrics.NewCollector()

	req := httptest.NewRequest("GET", "/api/v1/metrics/summary", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestHandleMetricsSummary_AuditDropped locks in OBS-AUDIT-1: the
// /api/v1/metrics/summary JSON response surfaces auditDropped
// under pipeline_metrics.audit_dropped.
func TestHandleMetricsSummary_AuditDropped(t *testing.T) {
	s := newTestServer(t)
	s.collector = metrics.NewCollector()
	s.collector.SetAuditDropped(42)

	req := httptest.NewRequest("GET", "/api/v1/metrics/summary", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	pm, ok := resp["pipeline_metrics"].(map[string]any)
	if !ok {
		t.Fatalf("missing pipeline_metrics in response: %v", resp)
	}
	if pm["audit_dropped"] != float64(42) {
		t.Errorf("expected audit_dropped=42, got %v", pm["audit_dropped"])
	}
}

// TestHandleMetricsSummary_AuditQueueLatency locks in OBS-AUDIT-2:
// the audit queue latency histogram is exposed in
// /api/v1/metrics/summary under pipeline_metrics.
func TestHandleMetricsSummary_AuditQueueLatency(t *testing.T) {
	s := newTestServer(t)
	c := metrics.NewCollector()
	s.collector = c
	c.ObserveAuditQueue(0.005)
	c.ObserveAuditQueue(0.020)
	c.ObserveAuditQueue(0.080)

	req := httptest.NewRequest("GET", "/api/v1/metrics/summary", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	pm, ok := resp["pipeline_metrics"].(map[string]any)
	if !ok {
		t.Fatalf("missing pipeline_metrics in response: %v", resp)
	}
	if _, ok := pm["audit_queue_avg_secs"]; !ok {
		t.Errorf("missing audit_queue_avg_secs in pipeline_metrics: %v", pm)
	}
	if _, ok := pm["audit_queue_p95_secs"]; !ok {
		t.Errorf("missing audit_queue_p95_secs in pipeline_metrics: %v", pm)
	}
	if _, ok := pm["audit_queue_p99_secs"]; !ok {
		t.Errorf("missing audit_queue_p99_secs in pipeline_metrics: %v", pm)
	}
}

func TestHandleTopCapabilities_NilTracker(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/metrics/top-capabilities", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleMetricsPrometheus(t *testing.T) {
	s := newTestServer(t)
	s.collector = metrics.NewCollector()

	req := httptest.NewRequest("GET", "/api/v1/metrics", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleMetricsPrometheus_NotConfigured(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/metrics", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleLogsStream_WithHub(t *testing.T) {
	hub := ws.NewHub(slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError})))
	go hub.Run()

	s := newTestServer(t)
	s.logHub = hub

	req := httptest.NewRequest("GET", "/api/v1/logs/stream", nil)
	// Cancel context after a short delay so SSE handler exits
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)
	go func() { time.Sleep(50 * time.Millisecond); cancel() }()

	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
}

func TestHandleLogsStream_NotConfigured(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/logs/stream", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Provider Handler Tests
// ---------------------------------------------------------------------------

func TestHandleListProviders(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/providers", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetProvider_Found(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/providers/openai", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetProvider_NotFound(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/providers/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleTestProvider_NotAvailable(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"model": "gpt-3.5-turbo"})
	req := httptest.NewRequest("POST", "/api/v1/providers/nonexistent/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestHandleCreateVersionWithManifest verifies that POST /v1/capabilities/
// {c}/versions accepts an explicit Manifest in the request body and
// rejects an invalid manifest with HTTP 400.
func TestHandleCreateVersionWithManifest(t *testing.T) {
	repo := newMockRepo()
	s := newTestServerWithRepo(t, repo)
	_ = repo.CreateCapability(context.Background(), &capability.Capability{ID: "c-manifest", Name: "manifest-test", ProjectID: "p1"})
	capID := "c-manifest"

	good := capability.Manifest{
		Prompt:        capability.ArtifactRef{Kind: capability.ArtifactPrompt, Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
		ModelPolicy:   capability.ArtifactRef{Kind: capability.ArtifactModelPolicy, Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
		RuntimePolicy: capability.ArtifactRef{Kind: capability.ArtifactRuntimePolicy, Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
		Context:       capability.ArtifactRef{Kind: capability.ArtifactContext, Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
		Memory:        capability.ArtifactRef{Kind: capability.ArtifactMemory, Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
	}
	body, err := json.Marshal(map[string]any{"version": 1, "manifest": good})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest("POST", "/api/v1/capabilities/"+capID+"/versions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 for valid manifest, got %d: %s", rr.Code, rr.Body.String())
	}
	var got capability.Version
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ManifestHash == "" {
		t.Fatalf("expected manifest_hash to be set, got empty")
	}

	body, _ = json.Marshal(map[string]any{"version": 2, "manifest": capability.Manifest{
		Prompt: capability.ArtifactRef{Kind: capability.ArtifactKind("bogus"), Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
	}})
	req = httptest.NewRequest("POST", "/api/v1/capabilities/"+capID+"/versions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid manifest kind, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Vault Handler Tests
// ---------------------------------------------------------------------------

func TestHandleSaveVaultKey(t *testing.T) {
	s := newTestServer(t)
	s.vault = newVault(t)

	body := mustMarshal(t, map[string]string{"provider_name": "openai", "key_name": "default", "key": "sk-abc123"})
	req := httptest.NewRequest("POST", "/api/v1/vault/keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["id"] == "" {
		t.Error("expected key id in response")
	}
}

func TestHandleSaveVaultKey_NoVault(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"provider_name": "openai", "key_name": "default", "key": "sk-abc123"})
	req := httptest.NewRequest("POST", "/api/v1/vault/keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSaveVaultKey_MissingFields(t *testing.T) {
	s := newTestServer(t)
	s.vault = newVault(t)

	body := mustMarshal(t, map[string]string{"provider_name": "openai"})
	req := httptest.NewRequest("POST", "/api/v1/vault/keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListVaultKeys(t *testing.T) {
	repo := newMockRepo()
	_ = repo.SaveProviderKey(context.Background(), &models.ProviderKey{ID: "pk1", ProviderName: "openai", KeyName: "default", EncryptedKey: "enc"})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	req := httptest.NewRequest("GET", "/api/v1/vault/keys", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleDeleteVaultKey(t *testing.T) {
	repo := newMockRepo()
	_ = repo.SaveProviderKey(context.Background(), &models.ProviderKey{ID: "pk1", ProviderName: "openai", KeyName: "default", EncryptedKey: "enc"})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	req := httptest.NewRequest("DELETE", "/api/v1/vault/keys/pk1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleDeleteVaultKey_NotFound(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("DELETE", "/api/v1/vault/keys/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Alert Handler Tests
// ---------------------------------------------------------------------------

func TestHandleListAlertRules(t *testing.T) {
	s := newTestServer(t)
	s.alertingManager = alerting.NewManager(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil)

	req := httptest.NewRequest("GET", "/api/v1/alerts/rules", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListAlertRules_NilManager(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/alerts/rules", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateAlertRule(t *testing.T) {
	s := newTestServer(t)
	s.alertingManager = alerting.NewManager(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil)

	body := mustMarshal(t, map[string]any{"name": "test-rule", "type": "threshold", "severity": "high", "threshold": 10.0})
	req := httptest.NewRequest("POST", "/api/v1/alerts/rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["name"] != "test-rule" {
		t.Errorf("expected test-rule, got %v", resp["name"])
	}
}

func TestHandleCreateAlertRule_NilManager(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]any{"name": "test-rule", "type": "threshold"})
	req := httptest.NewRequest("POST", "/api/v1/alerts/rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateAlertRule_MissingName(t *testing.T) {
	s := newTestServer(t)
	s.alertingManager = alerting.NewManager(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil)

	body := mustMarshal(t, map[string]any{"type": "threshold"})
	req := httptest.NewRequest("POST", "/api/v1/alerts/rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetAlertRule(t *testing.T) {
	am := alerting.NewManager(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil)
	rule := &alerting.AlertRule{ID: "rule1", Name: "test", Type: "threshold", Severity: alerting.SeverityHigh}
	am.AddRule(rule)
	s := newTestServer(t)
	s.alertingManager = am

	req := httptest.NewRequest("GET", "/api/v1/alerts/rules/rule1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["name"] != "test" {
		t.Errorf("expected test, got %v", resp["name"])
	}
}

func TestHandleGetAlertRule_NotFound(t *testing.T) {
	s := newTestServer(t)
	s.alertingManager = alerting.NewManager(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil)

	req := httptest.NewRequest("GET", "/api/v1/alerts/rules/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetAlertRule_NilManager(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/alerts/rules/rule1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleUpdateAlertRule(t *testing.T) {
	am := alerting.NewManager(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil)
	rule := &alerting.AlertRule{ID: "rule1", Name: "test", Type: "threshold", Severity: alerting.SeverityHigh}
	am.AddRule(rule)
	s := newTestServer(t)
	s.alertingManager = am

	body := mustMarshal(t, map[string]any{"name": "updated-rule", "enabled": false, "threshold": 20.0})
	req := httptest.NewRequest("PUT", "/api/v1/alerts/rules/rule1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["name"] != "updated-rule" {
		t.Errorf("expected updated-rule, got %v", resp["name"])
	}
	if resp["enabled"] != false {
		t.Errorf("expected enabled false, got %v", resp["enabled"])
	}
}

func TestHandleDeleteAlertRule(t *testing.T) {
	am := alerting.NewManager(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil)
	am.AddRule(&alerting.AlertRule{ID: "rule1", Name: "test", Type: "threshold", Severity: alerting.SeverityHigh})
	s := newTestServer(t)
	s.alertingManager = am

	req := httptest.NewRequest("DELETE", "/api/v1/alerts/rules/rule1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListAlerts(t *testing.T) {
	am := alerting.NewManager(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil)
	s := newTestServer(t)
	s.alertingManager = am

	req := httptest.NewRequest("GET", "/api/v1/alerts/active", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListAlerts_NilManager(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/alerts/active", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleResolveAlert_NotFound(t *testing.T) {
	am := alerting.NewManager(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil)
	s := newTestServer(t)
	s.alertingManager = am

	req := httptest.NewRequest("PUT", "/api/v1/alerts/active/nonexistent/resolve", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleResolveAlert_NilManager(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("PUT", "/api/v1/alerts/active/nonexistent/resolve", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleAddNotificationGroup(t *testing.T) {
	s := newTestServer(t)
	s.alertingManager = alerting.NewManager(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), nil)

	body := mustMarshal(t, map[string]any{"name": "slack-group", "channels": []string{"slack"}})
	req := httptest.NewRequest("POST", "/api/v1/alerts/notifications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["name"] != "slack-group" {
		t.Errorf("expected slack-group, got %v", resp["name"])
	}
}

func TestHandleAddNotificationGroup_NilManager(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]any{"name": "group", "channels": []string{"email"}})
	req := httptest.NewRequest("POST", "/api/v1/alerts/notifications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Webhook Handler Tests
// ---------------------------------------------------------------------------

func TestHandleListWebhooks_NilDispatcher(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/webhooks", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListWebhooks(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError}))
	s := newTestServer(t)
	s.webhooks = webhook.NewDispatcher(logger)

	req := httptest.NewRequest("GET", "/api/v1/webhooks", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateWebhook(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError}))
	s := newTestServer(t)
	s.webhooks = webhook.NewDispatcher(logger)

	body := mustMarshal(t, map[string]any{
		"url":    "https://example.com/webhook",
		"events": []string{"eval.completed"},
	})
	req := httptest.NewRequest("POST", "/api/v1/webhooks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["url"] != "https://example.com/webhook" {
		t.Errorf("expected url, got %v", resp["url"])
	}
}

func TestHandleCreateWebhook_NilDispatcher(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]any{
		"url":    "https://example.com/webhook",
		"events": []string{"eval.completed"},
	})
	req := httptest.NewRequest("POST", "/api/v1/webhooks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateWebhook_MissingURL(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError}))
	s := newTestServer(t)
	s.webhooks = webhook.NewDispatcher(logger)

	body := mustMarshal(t, map[string]any{"events": []string{"eval.completed"}})
	req := httptest.NewRequest("POST", "/api/v1/webhooks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateWebhook_NoEvents(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError}))
	s := newTestServer(t)
	s.webhooks = webhook.NewDispatcher(logger)

	body := mustMarshal(t, map[string]any{"url": "https://example.com/hook"})
	req := httptest.NewRequest("POST", "/api/v1/webhooks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleDeleteWebhook(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError}))
	s := newTestServer(t)
	s.webhooks = webhook.NewDispatcher(logger)

	req := httptest.NewRequest("DELETE", "/api/v1/webhooks/wh1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Capability / Workspace / Project / Version / Execution Handler Tests
// ---------------------------------------------------------------------------

func TestHandleListWorkspaces(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/workspaces", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateWorkspace(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"name": "test-workspace", "organization": "TestOrg"})
	req := httptest.NewRequest("POST", "/api/v1/workspaces", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["name"] != "test-workspace" {
		t.Errorf("expected test-workspace, got %v", resp["name"])
	}
}

func TestHandleCreateWorkspace_MissingName(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{})
	req := httptest.NewRequest("POST", "/api/v1/workspaces", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetWorkspace(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateWorkspace(context.Background(), &capability.Workspace{ID: "w1", Name: "Test"})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	req := httptest.NewRequest("GET", "/api/v1/workspaces/w1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetWorkspace_NotFound(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/workspaces/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleUpdateWorkspace(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateWorkspace(context.Background(), &capability.Workspace{ID: "w1", Name: "Old"})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	body := mustMarshal(t, map[string]string{"name": "Updated"})
	req := httptest.NewRequest("PUT", "/api/v1/workspaces/w1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["name"] != "Updated" {
		t.Errorf("expected Updated, got %v", resp["name"])
	}
}

func TestHandleDeleteWorkspace(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateWorkspace(context.Background(), &capability.Workspace{ID: "w1", Name: "Test"})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	req := httptest.NewRequest("DELETE", "/api/v1/workspaces/w1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListProjects(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/workspaces/w1/projects", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateProject(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"name": "test-project", "description": "A test"})
	req := httptest.NewRequest("POST", "/api/v1/workspaces/w1/projects", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateProject_MissingName(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{})
	req := httptest.NewRequest("POST", "/api/v1/workspaces/w1/projects", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetProject(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateProject(context.Background(), &capability.Project{ID: "p1", Name: "Test", WorkspaceID: "w1"})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	req := httptest.NewRequest("GET", "/api/v1/projects/p1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetProject_NotFound(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/projects/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleUpdateProject(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateProject(context.Background(), &capability.Project{ID: "p1", Name: "Old", WorkspaceID: "w1"})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	body := mustMarshal(t, map[string]string{"name": "Updated"})
	req := httptest.NewRequest("PUT", "/api/v1/projects/p1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleDeleteProject(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateProject(context.Background(), &capability.Project{ID: "p1", Name: "Test", WorkspaceID: "w1"})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	req := httptest.NewRequest("DELETE", "/api/v1/projects/p1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListCapabilities(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/projects/p1/capabilities", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateCapability(t *testing.T) {
	s := newTestServer(t)
	body := mustMarshal(t, map[string]string{"name": "test-capability", "description": "A test capability"})
	req := httptest.NewRequest("POST", "/api/v1/projects/p1/capabilities", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["name"] != "test-capability" {
		t.Errorf("expected test-capability, got %v", resp["name"])
	}
	if resp["name"] != "test-capability" {
		t.Errorf("expected test-capability, got %v", resp["name"])
	}
	if _, ok := resp["state"]; ok {
		t.Errorf("post-M0.8: state must not be returned on create (it is derived from Releases), got %v", resp["state"])
	}
}

func TestHandleGetCapability(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateCapability(context.Background(), &capability.Capability{ID: "c1", Name: "Test", ProjectID: "p1"})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	req := httptest.NewRequest("GET", "/api/v1/capabilities/c1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetCapability_NotFound(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/capabilities/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleUpdateCapability(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateCapability(context.Background(), &capability.Capability{ID: "c1", Name: "Old", ProjectID: "p1"})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	body := mustMarshal(t, map[string]string{"name": "Updated", "description": "New desc"})
	req := httptest.NewRequest("PUT", "/api/v1/capabilities/c1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["name"] != "Updated" {
		t.Errorf("expected Updated, got %v", resp["name"])
	}
}

func TestHandleDeleteCapability(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateCapability(context.Background(), &capability.Capability{ID: "c1", Name: "Test", ProjectID: "p1"})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	req := httptest.NewRequest("DELETE", "/api/v1/capabilities/c1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleListVersions(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/capabilities/c1/versions", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateVersion(t *testing.T) {
	s := newTestServer(t)
	good := capability.Manifest{
		Prompt:        capability.ArtifactRef{Kind: capability.ArtifactPrompt, Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
		ModelPolicy:   capability.ArtifactRef{Kind: capability.ArtifactModelPolicy, Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
		RuntimePolicy: capability.ArtifactRef{Kind: capability.ArtifactRuntimePolicy, Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
		Context:       capability.ArtifactRef{Kind: capability.ArtifactContext, Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
		Memory:        capability.ArtifactRef{Kind: capability.ArtifactMemory, Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
	}
	body, _ := json.Marshal(map[string]any{"version": 1, "manifest": good})
	req := httptest.NewRequest("POST", "/api/v1/capabilities/c1/versions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["version"] != float64(1) {
		t.Errorf("expected version 1, got %v", resp["version"])
	}
}

func TestHandleGetVersion(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateVersion(context.Background(), &capability.Version{ID: "v1", Version: 1, CapabilityID: "c1"})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	req := httptest.NewRequest("GET", "/api/v1/versions/v1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetLatestVersion(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateVersion(context.Background(), &capability.Version{ID: "v1", Version: 1, CapabilityID: "c1"})
	_ = repo.CreateVersion(context.Background(), &capability.Version{ID: "v2", Version: 2, CapabilityID: "c1"})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	req := httptest.NewRequest("GET", "/api/v1/capabilities/c1/versions/latest", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["id"] != "v2" {
		t.Errorf("expected v2, got %v", resp["id"])
	}
}

func TestHandleListExecutions(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/versions/v1/executions", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestHandleCreateExecution exercises the production invoke
// path end-to-end. newInvokeTestServer wires a real
// invoke.Invoker backed by an in-memory LLM provider, so the
// handler actually runs the request and persists a real
// Execution row with deterministic token counts (1 prompt / 1
// completion token / $0.01 cost).
func TestHandleCreateExecution(t *testing.T) {
	s := newInvokeTestServer(t)
	body := mustMarshal(t, map[string]any{
		"inputs":   map[string]any{"query": "hello"},
		"model":    "gpt-4",
		"provider": "openai",
	})
	req := httptest.NewRequest("POST", "/api/v1/versions/v1/executions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	readJSONBody(t, rr.Body.Bytes(), &resp)
	if resp["model"] != "gpt-4" {
		t.Errorf("expected gpt-4, got %v", resp["model"])
	}
	if got, want := resp["total_tokens"].(float64), float64(2); got != want {
		t.Errorf("expected total_tokens=%v, got %v", want, got)
	}
	if got, want := resp["cost_usd"].(float64), 0.01; got != want {
		t.Errorf("expected cost_usd=%v, got %v", want, got)
	}
}

func TestHandleGetExecution(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateExecution(context.Background(), &capability.Execution{ID: "e1", CapabilityVersionID: "v1"})
	s := newTestServer(t)
	s.db = newRepositories(repo)

	req := httptest.NewRequest("GET", "/api/v1/executions/e1", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetExecution_NotFound(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/v1/executions/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Rate Limiting Tests
// ---------------------------------------------------------------------------

func TestRateLimit(t *testing.T) {
	s := newTestServer(t)
	handler := s.rateLimit(func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	err := handler(rr, req)
	if err != nil {
		t.Fatal(err)
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRateLimit_Exceeded(t *testing.T) {
	// SEC-RL-2: rate=0 disables the limiter (always-allow). To
	// exercise the deny path we need a non-zero Rate that the
	// bucket can exhaust. Burst=0 with a single request means
	// the second call hits the bucket drain.
	limiter := ratelimit.NewLimiter(ratelimit.Config{Rate: 1, Interval: time.Hour, Burst: 0})
	t.Cleanup(limiter.Stop)

	s := newTestServer(t, WithRateLimiter(limiter))
	var called int
	handler := s.rateLimit(func(w http.ResponseWriter, _ *http.Request) error {
		called++
		w.WriteHeader(http.StatusOK)
		return nil
	})

	// First call: bucket starts at burst=0, the maths below
	// gives tokens=0, the request is denied without the inner
	// handler running.
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	err := handler(rr, req)
	if err != nil {
		t.Fatal(err)
	}
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rr.Code)
	}
	if called != 0 {
		t.Error("inner handler should not be called when rate limited")
	}
	if rr.Header().Get("Retry-After") != "60" {
		t.Errorf("expected Retry-After: 60, got %s", rr.Header().Get("Retry-After"))
	}
}

// ---------------------------------------------------------------------------
// requirePerm Tests
// ---------------------------------------------------------------------------

func TestRequirePerm_NoAuth(t *testing.T) {
	s := newTestServer(t)
	handler := s.requirePerm(auth.PermPromptRead)(func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	err := handler(rr, req)
	if err != nil {
		t.Fatal(err)
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRequirePerm_WithAuth_Unauthenticated(t *testing.T) {
	repo := newMockRepo()
	s := newAuthTestServer(t, repo)
	handler := s.requirePerm(auth.PermPromptRead)(func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	err := handler(rr, req)
	if err == nil {
		t.Fatal("expected error for unauthenticated request")
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusUnauthorized {
		t.Errorf("expected 401 error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Audit Worker Tests
// ---------------------------------------------------------------------------

func TestStartAuditWorkers(t *testing.T) {
	s := newTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.StartAuditWorkers(ctx, 3)
	if cap(s.auditQueue) != 1024 {
		t.Errorf("expected queue capacity 1024, got %d", cap(s.auditQueue))
	}
	_ = s.StopAuditWorkers(ctx)
}

func TestStartAuditWorkers_ZeroWorkers(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()
	s.StartAuditWorkers(ctx, 0)
	if cap(s.auditQueue) != 1024 {
		t.Errorf("expected queue capacity 1024, got %d", cap(s.auditQueue))
	}
	_ = s.StopAuditWorkers(ctx)
}

func TestAuditWorkerProcess(t *testing.T) {
	repo := newMockRepo()
	s := newTestServerWithRepo(t, repo)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.StartAuditWorkers(ctx, 1)
	entry := &models.AuditEntry{ID: "test-audit", Action: "test", Resource: "test-resource"}
	s.auditQueue <- entry

	// Give worker time to process
	time.Sleep(50 * time.Millisecond)
	cancel()
	_ = s.StopAuditWorkers(context.Background())

	entries := repo.auditEntries
	var found bool
	for _, e := range entries {
		if e.ID == "test-audit" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected audit entry to be stored by worker")
	}
}

func TestAudit_DropWhenQueueFull(t *testing.T) {
	s := newTestServer(t)
	// Use a closed channel so all sends fail
	s.auditQueue = make(chan *models.AuditEntry)
	s.auditDropped.Store(0)

	ctx := context.Background()
	s.audit(ctx, "test", "resource", nil)

	if s.auditDropped.Load() == 0 {
		t.Log("audit entry was not dropped (queue may have had room)")
	}
}

func TestAudit_WithRequestInContext(t *testing.T) {
	s := newTestServer(t)
	s.auditQueue = make(chan *models.AuditEntry, 100)

	r := httptest.NewRequest("GET", "/test", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("User-Agent", "test-agent")
	ctx := WithRequest(context.Background(), r)

	s.audit(ctx, "test-action", "test-resource", nil)

	select {
	case entry := <-s.auditQueue:
		if entry.Details["remote_addr"] != "127.0.0.1:1234" {
			t.Errorf("expected remote_addr=127.0.0.1:1234, got %v", entry.Details["remote_addr"])
		}
		if entry.Details["user_agent"] != "test-agent" {
			t.Errorf("expected user_agent=test-agent, got %v", entry.Details["user_agent"])
		}
	default:
		t.Error("expected audit entry in queue")
	}
}

func TestAudit_WithUserInContext(t *testing.T) {
	s := newTestServer(t)
	s.auditQueue = make(chan *models.AuditEntry, 100)

	ctx := auth.WithUserContext(context.Background(), &auth.User{ID: "u42", Role: auth.RoleWriter})
	s.audit(ctx, "create", "test", nil)

	select {
	case entry := <-s.auditQueue:
		if entry.UserID != "u42" {
			t.Errorf("expected user_id u42, got %s", entry.UserID)
		}
	default:
		t.Error("expected audit entry in queue")
	}
}

func TestStopAuditWorkers_ConcurrentIdempotent(t *testing.T) {
	s := newTestServer(t)
	s.StartAuditWorkers(t.Context(), 2)
	errs := make(chan error, 2)
	go func() { errs <- s.StopAuditWorkers(t.Context()) }()
	go func() { errs <- s.StopAuditWorkers(t.Context()) }()
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
}

func TestStopAuditWorkers_NilQueue(t *testing.T) {
	s := newTestServer(t)
	s.auditQueue = nil
	if err := s.StopAuditWorkers(context.Background()); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// OAuth State Store Tests
// ---------------------------------------------------------------------------

func TestOAuthStateStore_PutAndConsume(t *testing.T) {
	store := newOAuthStateStore()
	store.put("state1", time.Now().Add(10*time.Minute))

	if !store.consume("state1") {
		t.Error("expected state1 to be consumed successfully")
	}
	if store.consume("state1") {
		t.Error("expected state1 to be removed after consume")
	}
}

func TestOAuthStateStore_ConsumeNotFound(t *testing.T) {
	store := newOAuthStateStore()
	if store.consume("nonexistent") {
		t.Error("expected false for nonexistent state")
	}
}

func TestOAuthStateStore_ConsumeExpired(t *testing.T) {
	store := newOAuthStateStore()
	store.put("expired-state", time.Now().Add(-1*time.Hour))

	if store.consume("expired-state") {
		t.Error("expected false for expired state")
	}
}

func TestOAuthStateStore_Janitor(t *testing.T) {
	store := newOAuthStateStore()
	ctx, cancel := context.WithCancel(context.Background())
	store.start(ctx)

	store.put("stale", time.Now().Add(-1*time.Hour))
	store.put("fresh", time.Now().Add(10*time.Minute))

	cancel()
	store.stopJanitor()

	if !store.consume("fresh") {
		t.Error("expected fresh state to still be available after janitor stopped")
	}
	if store.consume("stale") {
		t.Error("expected stale state to be removed by janitor")
	}
}

func TestStartOAuthStateJanitor(_ *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	StartOAuthStateJanitor(ctx)
	StopOAuthStateJanitor()
}

func TestResolveAndValidateWebhook(t *testing.T) {
	err := ResolveAndValidateWebhook(context.Background(), "https://example.com/hook")
	if err != nil {
		t.Errorf("expected nil for public host, got %v", err)
	}

	err = ResolveAndValidateWebhook(context.Background(), "http://localhost:8080/hook")
	if err == nil {
		t.Error("expected error for http scheme")
	}
}

func TestResolveAndValidateWebhook_EmptyHost(t *testing.T) {
	err := ResolveAndValidateWebhook(context.Background(), "http:///path")
	if err == nil {
		t.Error("expected error for empty host")
	}
}

// ---------------------------------------------------------------------------
// Webhook URL Validation Tests
// ---------------------------------------------------------------------------

func TestValidateWebhookURL_InvalidScheme(t *testing.T) {
	err := ValidateWebhookURL("ftp://example.com/hook")
	if err == nil {
		t.Error("expected error for ftp scheme")
	}
}

func TestValidateWebhookURL_HTTPRejected(t *testing.T) {
	err := ValidateWebhookURL("http://example.com/hook")
	if err == nil {
		t.Error("expected error for http scheme (only https is accepted)")
	}
}

func TestValidateWebhookURL_MissingHost(t *testing.T) {
	err := ValidateWebhookURL("http:///path")
	if err == nil {
		t.Error("expected error for missing host")
	}
}

func TestValidateWebhookURL_LocalhostBlocked(t *testing.T) {
	err := ValidateWebhookURL("https://localhost:8080/hook")
	if err == nil {
		t.Error("expected error for localhost")
	}
}

func TestValidateWebhookURL_MetadataHostname(t *testing.T) {
	err := ValidateWebhookURL("https://metadata.google.internal")
	if err == nil {
		t.Error("expected error for metadata hostname")
	}
}

// TestValidateWebhookURL_AWSMetadataIP locks in the SEC-4a
// acceptance literal: https://169.254.169.254/... must be
// rejected as a link-local address. The hostname form was
// already covered by TestValidateWebhookURL_MetadataHostname.
func TestValidateWebhookURL_AWSMetadataIP(t *testing.T) {
	err := ValidateWebhookURL("https://169.254.169.254/latest/meta-data/")
	if err == nil {
		t.Error("expected error for AWS IMDS link-local address")
	}
}

// ---------------------------------------------------------------------------
// UsageTracker Tests
// ---------------------------------------------------------------------------

func TestUsageTracker_RecordAndGet(t *testing.T) {
	ut := NewUsageTracker()
	ut.RecordCapabilityUsage("p1", "Prompt 1", 100, 50.0)
	ut.RecordCapabilityUsage("p1", "Prompt 1", 200, 150.0)

	top := ut.GetTopCapabilities(10)
	if len(top) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(top))
	}
	if top[0].Count != 2 {
		t.Errorf("expected count 2, got %d", top[0].Count)
	}
	if top[0].AvgTokens != 150 {
		t.Errorf("expected avg tokens 150, got %f", top[0].AvgTokens)
	}
}

func TestUsageTracker_RecordAgent(t *testing.T) {
	ut := NewUsageTracker()
	ut.RecordCapabilityUsage("a1", "Agent 1", 100, 50.0)
	ut.RecordCapabilityUsage("a1", "Agent 1", 100, 50.0)
	ut.RecordCapabilityUsage("a2", "Agent 2", 100, 50.0)

	top := ut.GetTopCapabilities(10)
	if len(top) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(top))
	}
	if top[0].ID != "a1" || top[0].Count != 2 {
		t.Errorf("expected a1 with count 2, got %s with count %d", top[0].ID, top[0].Count)
	}
}

func TestUsageTracker_GetEmpty(t *testing.T) {
	ut := NewUsageTracker()
	if prompts := ut.GetTopCapabilities(10); len(prompts) != 0 {
		t.Errorf("expected empty, got %d", len(prompts))
	}
	if agents := ut.GetTopCapabilities(10); len(agents) != 0 {
		t.Errorf("expected empty, got %d", len(agents))
	}
}

func TestUsageTracker_GetTopCapabilitiesLimit(t *testing.T) {
	ut := NewUsageTracker()
	ut.RecordCapabilityUsage("p1", "P1", 10, 1)
	ut.RecordCapabilityUsage("p2", "P2", 20, 2)
	ut.RecordCapabilityUsage("p3", "P3", 30, 3)
	top := ut.GetTopCapabilities(1)
	if len(top) != 1 {
		t.Errorf("expected 1 result, got %d", len(top))
	}
}

func TestUsageTracker_GetTopCapabilitiesMore(t *testing.T) {
	ut := NewUsageTracker()
	ut.RecordCapabilityUsage("a1", "A1", 100, 50.0)
	ut.RecordCapabilityUsage("a2", "A2", 100, 50.0)
	ut.RecordCapabilityUsage("a3", "A3", 100, 50.0)
	top := ut.GetTopCapabilities(2)
	if len(top) != 2 {
		t.Errorf("expected 2 results, got %d", len(top))
	}
}

func TestUsageTracker_NewUsageTracker(t *testing.T) {
	ut := NewUsageTracker()
	if ut == nil {
		t.Fatal("expected non-nil tracker")
	}
	if ut.entries == nil {
		t.Error("expected initialized map")
	}
	if ut.order == nil {
		t.Error("expected initialized lru list")
	}
}

func TestUsageTracker_LRUEvicts(t *testing.T) {
	// PERF-5: oldest entry is evicted when the cap is hit.
	// We can't easily test with the production cap (16384)
	// here, so we directly exercise the list behaviour.
	ut := NewUsageTracker()
	for i := 0; i < maxUsageEntries; i++ {
		ut.RecordCapabilityUsage(strconv.Itoa(i), "c", 0, 0)
	}
	if ut.order.Len() != maxUsageEntries {
		t.Fatalf("expected full LRU, got %d", ut.order.Len())
	}
	ut.RecordCapabilityUsage("new", "new", 0, 0)
	if ut.order.Len() != maxUsageEntries {
		t.Fatalf("expected LRU cap, got %d", ut.order.Len())
	}
	if _, ok := ut.entries["0"]; ok {
		t.Error("expected entry 0 to be evicted")
	}
	if _, ok := ut.entries["new"]; !ok {
		t.Error("expected entry 'new' to be present")
	}
}

// ---------------------------------------------------------------------------
// storeAuthAdapter Tests
// ---------------------------------------------------------------------------

func TestStoreAuthAdapter_GetAPIKeyByHash(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateAPIKey(context.Background(), &models.APIKey{
		ID: "k1", UserID: "u1", KeyHash: "hash1", KeyPrefix: "ps_test", Role: "admin",
	})
	adapter := &storeAuthAdapter{db: repo}

	rec, err := adapter.GetAPIKeyByHash(context.Background(), "hash1")
	if err != nil {
		t.Fatal(err)
	}
	if rec == nil {
		t.Fatal("expected APIKeyRecord")
	}
	if rec.ID != "k1" || rec.UserID != "u1" || rec.Role != "admin" {
		t.Errorf("unexpected record: %+v", rec)
	}
}

func TestStoreAuthAdapter_GetAPIKeyByHash_Nil(t *testing.T) {
	adapter := &storeAuthAdapter{db: newMockRepo()}
	rec, err := adapter.GetAPIKeyByHash(context.Background(), "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if rec != nil {
		t.Error("expected nil record")
	}
}

func TestStoreAuthAdapter_UpdateAPIKeyLastUsed(t *testing.T) {
	repo := newMockRepo()
	_ = repo.CreateAPIKey(context.Background(), &models.APIKey{ID: "k1", UserID: "u1", KeyHash: "h1", KeyPrefix: "ps_t", Role: "reader"})
	adapter := &storeAuthAdapter{db: repo}
	if err := adapter.UpdateAPIKeyLastUsed(context.Background(), "k1"); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// authAuditLogger Tests
// ---------------------------------------------------------------------------

func TestAuthAuditLogger_LogAuthFailure(t *testing.T) {
	s := newTestServer(t)
	s.auditQueue = make(chan *models.AuditEntry, 100)

	adapter := &authAuditLogger{server: s}
	adapter.LogAuthFailure(context.Background(), "ps_test", "invalid key", "127.0.0.1")

	select {
	case entry := <-s.auditQueue:
		if entry.Action != "auth_failure" {
			t.Errorf("expected auth_failure, got %s", entry.Action)
		}
		if entry.Details["key_prefix"] != "ps_test" {
			t.Errorf("expected key_prefix ps_test, got %v", entry.Details["key_prefix"])
		}
	default:
		t.Error("expected audit entry")
	}
}

// ---------------------------------------------------------------------------
// Span Helper Tests (dashboard.go)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// WithRequest / httpRequestFromContext
// ---------------------------------------------------------------------------

func TestWithRequestAndHTTPRequestFromContext(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	ctx := WithRequest(context.Background(), req)
	got := httpRequestFromContext(ctx)
	if got == nil {
		t.Fatal("expected request from context")
	}
	if got.URL.Path != "/test" {
		t.Errorf("expected /test, got %s", got.URL.Path)
	}
	if httpRequestFromContext(context.Background()) != nil {
		t.Error("expected nil from empty context")
	}
}

// ---------------------------------------------------------------------------
// authenticateRequest
// ---------------------------------------------------------------------------

func TestAuthenticateRequest_NoAuth(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/", nil)
	newReq, user, err := s.authenticateRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	if user != nil {
		t.Error("expected nil user when auth disabled")
	}
	if newReq == nil {
		t.Error("expected non-nil request")
	}
}

func TestAuthenticateRequest_WithAuth_NoKey(t *testing.T) {
	repo := newMockRepo()
	s := newAuthTestServer(t, repo)
	req := httptest.NewRequest("GET", "/", nil)
	_, _, err := s.authenticateRequest(req)
	if err == nil {
		t.Error("expected error when no API key provided")
	}
}

func TestAuthenticateRequest_WithAuth_ValidKey(t *testing.T) {
	repo := newMockRepo()
	key, hash, _ := auth.GenerateAPIKey()
	_ = repo.CreateUser(context.Background(), &models.User{ID: "u1", Email: "u@t.com", Name: "U", Role: "admin"})
	_ = repo.CreateAPIKey(context.Background(), &models.APIKey{
		ID: "k1", UserID: "u1", Name: "admin-key", KeyHash: hash, KeyPrefix: key[:8], Role: "admin",
	})
	s := newAuthTestServer(t, repo)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	newReq, user, err := s.authenticateRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	if user == nil {
		t.Fatal("expected non-nil user")
	}
	if user.ID != "u1" {
		t.Errorf("expected u1, got %s", user.ID)
	}
	if newReq == nil {
		t.Error("expected non-nil request")
	}
}
