// Package store provides the repository interface for data access.
package store

import (
	"context"
	"time"

	"github.com/sachncs/promptsheon/internal/approval"
	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/harness"
	"github.com/sachncs/promptsheon/internal/models"
	"github.com/sachncs/promptsheon/internal/release"
)

// Repository defines the data access interface for all persistence operations.
//
// It embeds capability.Repository (the consumer-defined interface)
// and adds aggregates that don't yet have per-package interfaces
// (Users, API Keys, Audit, Provider Keys, Alert Rules, Alerts,
// Notification Groups, Webhook Endpoints). Migration to per-aggregate
// interfaces is a follow-on; today's structure already follows the
// dependency direction: domain packages declare the interface,
// storage satisfies it.
type Repository interface {
	capability.Repository
	release.Repository
	approval.Repository
	harness.Repository

	// Users
	CreateUser(ctx context.Context, u *models.User) error
	GetUser(ctx context.Context, id string) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	ListUsers(ctx context.Context) ([]*models.User, error)
	UpdateUser(ctx context.Context, u *models.User) error
	DeleteUser(ctx context.Context, id string) error
	BootstrapAdmin(ctx context.Context, u *models.User, key *models.APIKey) error

	// API Keys
	CreateAPIKey(ctx context.Context, key *models.APIKey) error
	GetAPIKeyByHash(ctx context.Context, keyHash string) (*models.APIKey, error)
	GetAPIKeyByID(ctx context.Context, id string) (*models.APIKey, error)
	DeleteAPIKey(ctx context.Context, id string) error
	ListAPIKeysByUser(ctx context.Context, userID string) ([]*models.APIKey, error)
	UpdateAPIKeyLastUsed(ctx context.Context, id string) error

	// Audit
	AppendAudit(ctx context.Context, entry *models.AuditEntry) error
	ListAudit(ctx context.Context, filter *models.AuditFilter) ([]*models.AuditEntry, error)
	ExportAudit(ctx context.Context, filter *models.AuditFilter) ([]*models.AuditEntry, error)
	VerifyAuditChain(ctx context.Context) (*AuditVerifyResult, error)

	// Provider Keys (LLM API key vaulting)
	SaveProviderKey(ctx context.Context, pk *models.ProviderKey) error
	GetProviderKey(ctx context.Context, id string) (*models.ProviderKey, error)
	GetProviderKeyByName(ctx context.Context, providerName, keyName string) (*models.ProviderKey, error)
	DeleteProviderKey(ctx context.Context, id string) error
	ListProviderKeys(ctx context.Context) ([]*models.ProviderKey, error)

	// Alert Rules
	SaveAlertRule(ctx context.Context, r *models.AlertRuleRecord) error
	GetAlertRule(ctx context.Context, id string) (*models.AlertRuleRecord, error)
	DeleteAlertRule(ctx context.Context, id string) error
	ListAlertRules(ctx context.Context) ([]*models.AlertRuleRecord, error)

	// Alerts
	SaveAlert(ctx context.Context, a *models.AlertRecord) error
	GetAlert(ctx context.Context, id string) (*models.AlertRecord, error)
	UpdateAlert(ctx context.Context, a *models.AlertRecord) error
	ListAlerts(ctx context.Context, status string) ([]*models.AlertRecord, error)

	// Notification Groups
	SaveNotificationGroup(ctx context.Context, g *models.NotificationGroupRecord) error
	GetNotificationGroup(ctx context.Context, id string) (*models.NotificationGroupRecord, error)
	DeleteNotificationGroup(ctx context.Context, id string) error
	ListNotificationGroups(ctx context.Context) ([]*models.NotificationGroupRecord, error)

	// Alert rule / notification group M2M (migration 045).
	// Returns the union of channels across all groups wired to the
	// given rule. The order is unspecified; consumers do not rely
	// on it.
	GetChannelsForAlertRule(ctx context.Context, ruleID string) ([]string, error)
	// LinkRuleToGroup wires an alert rule to a notification group.
	// Idempotent. DB-11b.
	LinkRuleToGroup(ctx context.Context, ruleID, groupID string) error
	// UnlinkRuleFromGroup removes the M2M wire. Idempotent. DB-11b.
	UnlinkRuleFromGroup(ctx context.Context, ruleID, groupID string) error

	// Webhook Endpoints
	SaveWebhookEndpoint(ctx context.Context, ep *models.WebhookEndpointRecord) error
	GetWebhookEndpoint(ctx context.Context, id string) (*models.WebhookEndpointRecord, error)
	DeleteWebhookEndpoint(ctx context.Context, id string) error
	ListWebhookEndpoints(ctx context.Context) ([]*models.WebhookEndpointRecord, error)

	// Vault State (singleton). Persists the KMS-wrapped data key
	// so a process restart (or a fresh daemon process) can recover
	// the same data key by calling Decrypt on the persisted blob.
	// SEC-10a.
	GetVaultState(ctx context.Context) (*models.VaultState, error)
	SaveVaultState(ctx context.Context, vs *models.VaultState) error

	// WS State (singleton). Persists the SSE hub's nextID so
	// client IDs survive restarts. OBS-LOG-3.
	GetWSNextID(ctx context.Context) (int64, error)
	SetWSNextID(ctx context.Context, n int64) error

	// Enforcer state (budget + quota persistence). OBS-13.
	// Loads persisted SetBudget / SetQuota values on startup so
	// budget counters and quota counters survive a restart.
	GetEnforcerBudget(ctx context.Context, workspaceID string) ([]byte, error)
	SetEnforcerBudget(ctx context.Context, workspaceID string, payload []byte) error
	GetEnforcerQuota(ctx context.Context, workspaceID string) ([]byte, error)
	SetEnforcerQuota(ctx context.Context, workspaceID string, payload []byte) error

	// Lifecycle
	Ping(ctx context.Context) error
	Close() error

	// Settings (operator-tunable runtime config). SettingsMode
	// is enforced at the API layer; the store layer is mode-agnostic
	// so tests can drive GetSystemConfig / SetSystemConfig /
	// DeleteSystemConfig directly.
	GetSystemConfig(ctx context.Context, key string) (string, time.Time, error)
	SetSystemConfig(ctx context.Context, key, value, updatedBy string) error
	DeleteSystemConfig(ctx context.Context, key string) error
	ListSystemConfig(ctx context.Context) ([]models.SystemConfig, error)
}
