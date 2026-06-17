// Package auth provides authentication and authorization for the Promptsheon
// API server. It implements API key authentication and role-based access
// control (RBAC).
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// Role represents a user role with specific permissions.
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleWriter Role = "writer"
	RoleReader Role = "reader"
)

// Permission represents a specific action that can be performed.
type Permission string

const (
	PermPromptCreate   Permission = "prompt:create"
	PermPromptRead     Permission = "prompt:read"
	PermPromptUpdate   Permission = "prompt:update"
	PermPromptDelete   Permission = "prompt:delete"
	PermAgentCreate    Permission = "agent:create"
	PermAgentRead      Permission = "agent:read"
	PermAgentUpdate    Permission = "agent:update"
	PermAgentDelete    Permission = "agent:delete"
	PermDatasetCreate  Permission = "dataset:create"
	PermDatasetRead    Permission = "dataset:read"
	PermDatasetUpdate  Permission = "dataset:update"
	PermDatasetDelete  Permission = "dataset:delete"
	PermEvalRun        Permission = "eval:run"
	PermEvalRead       Permission = "eval:read"
	PermReviewCreate   Permission = "review:create"
	PermReviewApprove  Permission = "review:approve"
	PermAuditRead      Permission = "audit:read"
	PermAPIKeyManage   Permission = "apikey:manage"
	PermUserManage     Permission = "user:manage"
)

// rolePermissions maps roles to their allowed permissions.
var rolePermissions = map[Role][]Permission{
	RoleAdmin: {
		PermPromptCreate, PermPromptRead, PermPromptUpdate, PermPromptDelete,
		PermAgentCreate, PermAgentRead, PermAgentUpdate, PermAgentDelete,
		PermDatasetCreate, PermDatasetRead, PermDatasetUpdate, PermDatasetDelete,
		PermEvalRun, PermEvalRead,
		PermReviewCreate, PermReviewApprove,
		PermAuditRead,
		PermAPIKeyManage, PermUserManage,
	},
	RoleWriter: {
		PermPromptCreate, PermPromptRead, PermPromptUpdate,
		PermAgentCreate, PermAgentRead, PermAgentUpdate,
		PermDatasetCreate, PermDatasetRead, PermDatasetUpdate,
		PermEvalRun, PermEvalRead,
		PermReviewCreate,
	},
	RoleReader: {
		PermPromptRead,
		PermAgentRead,
		PermDatasetRead,
		PermEvalRead,
	},
}

// HasPermission checks if a role has a specific permission.
func HasPermission(role Role, perm Permission) bool {
	perms, ok := rolePermissions[role]
	if !ok {
		return false
	}
	for _, p := range perms {
		if p == perm {
			return true
		}
	}
	return false
}

// GenerateAPIKey generates a random API key and returns the key and its
// SHA-256 hash. The key is returned in the format "ps_" followed by hex-encoded
// random bytes. The hash is used for storage.
func GenerateAPIKey() (key string, hash string, err error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", "", fmt.Errorf("generate api key: %w", err)
	}
	key = "ps_" + hex.EncodeToString(b)
	hash = HashAPIKey(key)
	return key, hash, nil
}

// HashAPIKey returns the SHA-256 hex-encoded hash of an API key.
func HashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// ValidateAPIKeyFormat checks if an API key has the expected prefix and format.
func ValidateAPIKeyFormat(key string) bool {
	return strings.HasPrefix(key, "ps_") && len(key) == 67
}
