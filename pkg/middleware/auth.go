package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/aegis-platform/aegis/pkg/apperrors"
	"github.com/aegis-platform/aegis/pkg/httputil"
)

// TokenValidator defines the interface for validating candidate session tokens.
type TokenValidator interface {
	ValidateToken(ctx context.Context, token string) (candidateID string, sessionID string, err error)
}

// SimpleTokenValidator is an in-memory/static token validator for dev and tests.
type SimpleTokenValidator struct{}

func (s *SimpleTokenValidator) ValidateToken(ctx context.Context, token string) (string, string, error) {
	if token == "" {
		return "", "", apperrors.NewUnauthorized("missing session token")
	}
	parts := strings.Split(token, ":")
	if len(parts) == 2 {
		return parts[0], parts[1], nil
	}
	return "cand-default", "sess-default", nil
}

// CandidateAuth verifies the Bearer token on incoming exam delivery requests (A7).
func CandidateAuth(validator TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				httputil.RespondError(w, r, apperrors.NewUnauthorized("missing Authorization header"))
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				httputil.RespondError(w, r, apperrors.NewUnauthorized("invalid Authorization header format, expected Bearer <token>"))
				return
			}

			token := parts[1]
			candID, sessID, err := validator.ValidateToken(r.Context(), token)
			if err != nil {
				httputil.RespondError(w, r, apperrors.NewUnauthorized("invalid or expired candidate token"))
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, candID)
			ctx = context.WithValue(ctx, RequestIDKey, sessID)
			ctx = context.WithValue(ctx, RolesKey, []string{"CANDIDATE"})

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole enforces role-based access control (RBAC) on API endpoints (R7).
func RequireRole(allowedRoles ...string) func(http.Handler) http.Handler {
	allowedSet := make(map[string]bool, len(allowedRoles))
	for _, r := range allowedRoles {
		allowedSet[r] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rolesVal := r.Context().Value(RolesKey)
			if rolesVal == nil {
				httputil.RespondError(w, r, apperrors.NewForbidden("access forbidden: no roles assigned"))
				return
			}

			roles, ok := rolesVal.([]string)
			if !ok {
				httputil.RespondError(w, r, apperrors.NewForbidden("access forbidden: invalid roles assignment"))
				return
			}

			for _, role := range roles {
				if allowedSet[role] {
					next.ServeHTTP(w, r)
					return
				}
			}

			httputil.RespondError(w, r, apperrors.NewForbidden("insufficient permissions for this operation"))
		})
	}
}
