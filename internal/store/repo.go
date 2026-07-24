package store

import (
	"context"

	"github.com/sachncs/promptsheon/internal/approval"
	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/harness"
	"github.com/sachncs/promptsheon/internal/models"
	"github.com/sachncs/promptsheon/internal/release"
	"github.com/sachncs/promptsheon/internal/settings"
)

type Users interface {
	CreateUser(context.Context, *models.User) error
	GetUser(context.Context, string) (*models.User, error)
	GetUserByEmail(context.Context, string) (*models.User, error)
	ListUsers(context.Context) ([]*models.User, error)
	UpdateUser(context.Context, *models.User) error
	DeleteUser(context.Context, string) error
	BootstrapAdmin(context.Context, *models.User, *models.APIKey) error
}
type APIKeys interface {
	CreateAPIKey(context.Context, *models.APIKey) error
	GetAPIKeyByHash(context.Context, string) (*models.APIKey, error)
	GetAPIKeyByID(context.Context, string) (*models.APIKey, error)
	DeleteAPIKey(context.Context, string) error
	ListAPIKeysByUser(context.Context, string) ([]*models.APIKey, error)
	UpdateAPIKeyLastUsed(context.Context, string) error
}
type Audit interface {
	AppendAudit(context.Context, *models.AuditEntry) error
	ListAudit(context.Context, *models.AuditFilter) ([]*models.AuditEntry, error)
	ExportAudit(context.Context, *models.AuditFilter) ([]*models.AuditEntry, error)
	VerifyAuditChain(context.Context) (*AuditVerifyResult, error)
}
type ProviderKeys interface {
	SaveProviderKey(context.Context, *models.ProviderKey) error
	GetProviderKey(context.Context, string) (*models.ProviderKey, error)
	GetProviderKeyByName(context.Context, string, string) (*models.ProviderKey, error)
	DeleteProviderKey(context.Context, string) error
	ListProviderKeys(context.Context) ([]*models.ProviderKey, error)
}
type Alerting interface {
	SaveAlertRule(context.Context, *models.AlertRuleRecord) error
	GetAlertRule(context.Context, string) (*models.AlertRuleRecord, error)
	DeleteAlertRule(context.Context, string) error
	ListAlertRules(context.Context) ([]*models.AlertRuleRecord, error)
	SaveAlert(context.Context, *models.AlertRecord) error
	GetAlert(context.Context, string) (*models.AlertRecord, error)
	UpdateAlert(context.Context, *models.AlertRecord) error
	ListAlerts(context.Context, string) ([]*models.AlertRecord, error)
	SaveNotificationGroup(context.Context, *models.NotificationGroupRecord) error
	GetNotificationGroup(context.Context, string) (*models.NotificationGroupRecord, error)
	DeleteNotificationGroup(context.Context, string) error
	ListNotificationGroups(context.Context) ([]*models.NotificationGroupRecord, error)
	GetChannelsForAlertRule(context.Context, string) ([]string, error)
	LinkRuleToGroup(context.Context, string, string) error
	UnlinkRuleFromGroup(context.Context, string, string) error
}
type Webhooks interface {
	SaveWebhookEndpoint(context.Context, *models.WebhookEndpointRecord) error
	GetWebhookEndpoint(context.Context, string) (*models.WebhookEndpointRecord, error)
	DeleteWebhookEndpoint(context.Context, string) error
	ListWebhookEndpoints(context.Context) ([]*models.WebhookEndpointRecord, error)
}
type VaultState interface {
	GetVaultState(context.Context) (*models.VaultState, error)
	SaveVaultState(context.Context, *models.VaultState) error
}
type WSState interface {
	GetWSNextID(context.Context) (int64, error)
	SetWSNextID(context.Context, int64) error
}
type EnforcerState interface {
	GetEnforcerBudget(context.Context, string) ([]byte, error)
	SetEnforcerBudget(context.Context, string, []byte) error
	GetEnforcerQuota(context.Context, string) ([]byte, error)
	SetEnforcerQuota(context.Context, string, []byte) error
}
type Lifecycle interface {
	Ping(context.Context) error
	Close() error
}

// Settings is the SQLite-backed contract that the settings
// resolver consumes. The previous (value, updatedAt)-only
// shape was lossy under concurrent writes; the current CRDT
// shape carries a version vector, replica id, tombstone, and
// monotonic timestamp so the resolver can implement a
// last-write-wins register per key with conflict-safe merge.
type Settings interface {
	GetSystemConfig(ctx context.Context, key string) (settings.CRDTRecord, error)
	SetSystemConfig(ctx context.Context, rec settings.CRDTRecord) error
	ListSystemConfig(ctx context.Context) ([]settings.CRDTRecord, error)
	MergeSystemConfig(ctx context.Context, replicaID string, records []settings.CRDTRecord) error
}

type CapabilityRepository = capability.Repository
type ReleaseRepository = release.Repository
type ApprovalRepository = approval.Repository
type HarnessRepository = harness.Repository

type Repositories struct {
	Users
	APIKeys
	Audit
	ProviderKeys
	Alerting
	Webhooks
	VaultState
	WSState
	EnforcerState
	Settings
	Lifecycle
	CapabilityRepository
	ReleaseRepository
	ApprovalRepository
	HarnessRepository
}

func NewRepositories(db *SQLite) *Repositories {
	return &Repositories{
		Users:                db,
		APIKeys:              db,
		Audit:                db,
		ProviderKeys:         db,
		Alerting:             db,
		Webhooks:             db,
		VaultState:           db,
		WSState:              db,
		EnforcerState:        db,
		Settings:             db,
		Lifecycle:            db,
		CapabilityRepository: db,
		ReleaseRepository:    db,
		ApprovalRepository:   db,
		HarnessRepository:    db,
	}
}
