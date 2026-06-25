// Package store defines the persistence interface and implementations for
// Promptsheon metadata. Content is stored in the CAS engine; metadata is
// stored here.
package store

import (
	"context"

	"github.com/sachn-cs/promptsheon/internal/models"
)

// Repository defines the persistence contract for all domain types.
type Repository interface {
	// Prompts
	CreatePrompt(ctx context.Context, p *models.Prompt) error
	GetPrompt(ctx context.Context, id string) (*models.Prompt, error)
	ListPrompts(ctx context.Context, filter models.PromptFilter) ([]*models.Prompt, error)
	UpdatePrompt(ctx context.Context, p *models.Prompt) error
	DeletePrompt(ctx context.Context, id string) error

	// Agents
	CreateAgent(ctx context.Context, a *models.Agent) error
	GetAgent(ctx context.Context, id string) (*models.Agent, error)
	ListAgents(ctx context.Context) ([]*models.Agent, error)
	UpdateAgent(ctx context.Context, a *models.Agent) error
	DeleteAgent(ctx context.Context, id string) error

	// Test Datasets
	CreateDataset(ctx context.Context, d *models.TestDataset) error
	GetDataset(ctx context.Context, id string) (*models.TestDataset, error)
	ListDatasets(ctx context.Context) ([]*models.TestDataset, error)
	UpdateDataset(ctx context.Context, d *models.TestDataset) error
	DeleteDataset(ctx context.Context, id string) error

	// Evaluations
	SaveEvalResults(ctx context.Context, results []*models.EvalResult) error
	GetEvalResults(ctx context.Context, promptHash string) ([]*models.EvalResult, error)
	GetEvalResultsByDataset(ctx context.Context, datasetID string) ([]*models.EvalResult, error)

	// Eval Runs
	SaveEvalRun(ctx context.Context, run *models.EvalRun) error
	GetEvalRun(ctx context.Context, id string) (*models.EvalRun, error)
	ListEvalRuns(ctx context.Context, filter models.EvalRunFilter) ([]*models.EvalRun, error)

	// Workflows
	SaveWorkflow(ctx context.Context, w *models.Workflow) error
	GetWorkflow(ctx context.Context, id string) (*models.Workflow, error)
	ListWorkflows(ctx context.Context, filter models.WorkflowFilter) ([]*models.Workflow, error)
	SaveWorkflowStep(ctx context.Context, s *models.WorkflowStep) error
	GetWorkflowSteps(ctx context.Context, workflowID string) ([]*models.WorkflowStep, error)

	// Audit
	AppendAudit(ctx context.Context, entry *models.AuditEntry) error
	ListAudit(ctx context.Context, filter models.AuditFilter) ([]*models.AuditEntry, error)
	ExportAudit(ctx context.Context, filter models.AuditFilter) ([]*models.AuditEntry, error)
	VerifyAuditChain(ctx context.Context) (bool, string, error)

	// Reviews
	CreateReview(ctx context.Context, r *models.Review) error
	GetReview(ctx context.Context, id string) (*models.Review, error)
	UpdateReview(ctx context.Context, r *models.Review) error
	ListPendingReviews(ctx context.Context) ([]*models.Review, error)
	ListReviewsByResource(ctx context.Context, resourceID, resourceType string) ([]*models.Review, error)

	// API Keys
	CreateAPIKey(ctx context.Context, key *models.APIKey) error
	GetAPIKeyByHash(ctx context.Context, keyHash string) (*models.APIKey, error)
	GetAPIKeyByID(ctx context.Context, id string) (*models.APIKey, error)
	DeleteAPIKey(ctx context.Context, id string) error
	ListAPIKeysByUser(ctx context.Context, userID string) ([]*models.APIKey, error)
	UpdateAPIKeyLastUsed(ctx context.Context, id string) error

	// Users
	CreateUser(ctx context.Context, u *models.User) error
	GetUser(ctx context.Context, id string) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	ListUsers(ctx context.Context) ([]*models.User, error)
	UpdateUser(ctx context.Context, u *models.User) error
	DeleteUser(ctx context.Context, id string) error

	// Provider Keys (LLM API key vaulting)
	SaveProviderKey(ctx context.Context, pk *models.ProviderKey) error
	GetProviderKey(ctx context.Context, id string) (*models.ProviderKey, error)
	GetProviderKeyByName(ctx context.Context, providerName, keyName string) (*models.ProviderKey, error)
	DeleteProviderKey(ctx context.Context, id string) error
	ListProviderKeys(ctx context.Context) ([]*models.ProviderKey, error)

	// Execution Logs
	SaveExecutionLog(ctx context.Context, el *models.ExecutionLog) error
	GetExecutionLog(ctx context.Context, id string) (*models.ExecutionLog, error)
	ListExecutionLogs(ctx context.Context, filter models.ExecutionLogFilter) ([]*models.ExecutionLog, error)

	// Contexts
	CreateContext(ctx context.Context, c *models.Context) error
	GetContext(ctx context.Context, id string) (*models.Context, error)
	ListContexts(ctx context.Context, filter models.ContextFilter) ([]*models.Context, error)
	UpdateContext(ctx context.Context, c *models.Context) error
	DeleteContext(ctx context.Context, id string) error

	// Guardrail Rules
	SaveGuardrailRule(ctx context.Context, r *models.GuardrailRule) error
	GetGuardrailRule(ctx context.Context, id string) (*models.GuardrailRule, error)
	DeleteGuardrailRule(ctx context.Context, id string) error
	ListGuardrailRules(ctx context.Context) ([]*models.GuardrailRule, error)

	// Guardrail Violations
	SaveGuardrailViolation(ctx context.Context, v *models.GuardrailViolationRecord) error
	ListGuardrailViolations(ctx context.Context, resolved bool) ([]*models.GuardrailViolationRecord, error)
	UpdateGuardrailViolation(ctx context.Context, v *models.GuardrailViolationRecord) error

	// Agent Guardrail Configs
	SaveAgentGuardrailConfig(ctx context.Context, c *models.AgentGuardrailConfig) error
	GetAgentGuardrailConfig(ctx context.Context, id string) (*models.AgentGuardrailConfig, error)
	GetAgentGuardrailConfigByAgent(ctx context.Context, agentID string) (*models.AgentGuardrailConfig, error)
	DeleteAgentGuardrailConfig(ctx context.Context, id string) error

	// Agent Executions
	SaveAgentExecution(ctx context.Context, e *models.AgentExecution) error
	GetAgentExecution(ctx context.Context, id string) (*models.AgentExecution, error)
	ListAgentExecutions(ctx context.Context, agentID string, limit, offset int) ([]*models.AgentExecution, error)

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

	// Webhook Endpoints
	SaveWebhookEndpoint(ctx context.Context, ep *models.WebhookEndpointRecord) error
	GetWebhookEndpoint(ctx context.Context, id string) (*models.WebhookEndpointRecord, error)
	DeleteWebhookEndpoint(ctx context.Context, id string) error
	ListWebhookEndpoints(ctx context.Context) ([]*models.WebhookEndpointRecord, error)

	// Lifecycle
	Ping(ctx context.Context) error
	Close() error
}
