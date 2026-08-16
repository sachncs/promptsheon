package promptsheon

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"

	"github.com/sachncs/promptsheon/promptsheon/approval"
	"github.com/sachncs/promptsheon/promptsheon/capability"
	"github.com/sachncs/promptsheon/promptsheon/harness"
	"github.com/sachncs/promptsheon/promptsheon/llm"
	"github.com/sachncs/promptsheon/promptsheon/models"
	"github.com/sachncs/promptsheon/promptsheon/release"
	"github.com/sachncs/promptsheon/promptsheon/store"
	"github.com/sachncs/promptsheon/promptsheon/vault"
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

func newDB(repo *mockRepo) *store.DB {
	return &store.DB{
		Users:                repo,
		APIKeys:              repo,
		Audit:                repo,
		ProviderKeys:         repo,
		Alerting:             repo,
		Webhooks:             repo,
		VaultState:           repo,
		WSState:              repo,
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

// Method implementations live in per-resource files:
//   handlers_test_support_settings_test.go
//   handlers_test_support_users_test.go
//   ... (one file per resource, see git log for the split)
//
// This file holds only the type, the constructor, and the test
// helpers used across the api handler tests.

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
	return NewServer(newDB(repo), logger, allOpts...)
}

func newAuthTestServer(t *testing.T, repo *mockRepo, opts ...Option) *Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError}))
	allOpts := make([]Option, 0, 1+len(opts))
	allOpts = append(allOpts, WithAuth(repo))
	allOpts = append(allOpts, opts...)
	return NewServer(newDB(repo), logger, allOpts...)
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
