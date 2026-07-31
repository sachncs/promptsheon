//go:build promptsheon

package promptsheon

import "github.com/sachncs/promptsheon/backend/auth"

// Re-exported role + permission constants. Consumers reference
// these when calling role-checking helpers in the SDK's auth
// adapters.
const (
	RoleAdmin  = auth.RoleAdmin
	RoleWriter = auth.RoleWriter
	RoleReader = auth.RoleReader
)

// Permission enumerates the auth.Permission values used by
// Promptsheon's RBAC system. The naming mirrors the
// auth.PermXxx constants; both are interchangeable.
type Permission = auth.Permission

const (
	PermPromptCreate = auth.PermPromptCreate
	PermPromptRead   = auth.PermPromptRead
	PermPromptUpdate = auth.PermPromptUpdate
	PermPromptDelete = auth.PermPromptDelete
	PermAgentCreate  = auth.PermAgentCreate
	PermAgentRead    = auth.PermAgentRead
	PermAgentUpdate  = auth.PermAgentUpdate
	PermAgentDelete  = auth.PermAgentDelete
	PermDatasetCreate = auth.PermDatasetCreate
	PermDatasetRead   = auth.PermDatasetRead
	PermEvalRun        = auth.PermEvalRun
	PermEvalRead       = auth.PermEvalRead
	PermReviewCreate   = auth.PermReviewCreate
	PermReviewApprove  = auth.PermReviewApprove
	PermAuditRead       = auth.PermAuditRead
	PermAPIKeyManage    = auth.PermAPIKeyManage
	PermWebhookAdmin    = auth.PermWebhookAdmin
	PermUserManage      = auth.PermUserManage
	PermSettingsRead    = auth.PermSettingsRead
	PermSettingsWrite   = auth.PermSettingsWrite
)

// DefaultAdminEmail is the email of the bootstrap admin user
// created on first daemon start. Re-exported for SDK consumers
// who want to construct the initial admin role.
const DefaultAdminEmail = auth.DefaultAdminEmail