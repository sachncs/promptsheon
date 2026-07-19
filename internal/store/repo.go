// Package store provides the repository interface for data access.
package store

import (
	"context"

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
	VerifyAuditChain(ctx context.Context) (bool, string, error)

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

	// Webhook Endpoints
	SaveWebhookEndpoint(ctx context.Context, ep *models.WebhookEndpointRecord) error
	GetWebhookEndpoint(ctx context.Context, id string) (*models.WebhookEndpointRecord, error)
	DeleteWebhookEndpoint(ctx context.Context, id string) error
	ListWebhookEndpoints(ctx context.Context) ([]*models.WebhookEndpointRecord, error)

	// Lifecycle
	Ping(ctx context.Context) error
	Close() error
}
