package auth

import (
	"net/http"
)

// Authorizer checks whether an authenticated user has the required permission.
type Authorizer struct{}

// NewAuthorizer creates a new Authorizer.
func NewAuthorizer() *Authorizer {
	return &Authorizer{}
}

// Require returns middleware that rejects requests if the user does not have
// the specified permission.
func (az *Authorizer) Require(perm Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok {
				http.Error(w, `{"error":"unauthorized","message":"no user in context"}`, http.StatusUnauthorized)
				return
			}
			if !HasPermission(user.Role, perm) {
				http.Error(w, `{"error":"forbidden","message":"insufficient permissions"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
