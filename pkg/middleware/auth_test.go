package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aegis-platform/aegis/pkg/middleware"
)

func TestCandidateAuthMiddleware(t *testing.T) {
	validator := &middleware.SimpleTokenValidator{}
	authMiddleware := middleware.CandidateAuth(validator)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok:" + userID))
	})

	chained := authMiddleware(testHandler)

	t.Run("Missing Authorization Header returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/sessions/123/questions", nil)
		rec := httptest.NewRecorder()

		chained.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Invalid Bearer Format returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/sessions/123/questions", nil)
		req.Header.Set("Authorization", "Basic 123456")
		rec := httptest.NewRecorder()

		chained.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Valid Token injects candidate ID and returns 200", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/sessions/123/questions", nil)
		req.Header.Set("Authorization", "Bearer cand-42:sess-99")
		rec := httptest.NewRecorder()

		chained.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "ok:cand-42", rec.Body.String())
	})
}

func TestRequireRoleMiddleware(t *testing.T) {
	roleCheck := middleware.RequireRole("EXAM_ADMIN")

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("admin_allowed"))
	})

	chained := roleCheck(testHandler)

	t.Run("User without required role is forbidden (403)", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/admin/generate", nil)
		ctx := context.WithValue(req.Context(), middleware.RolesKey, []string{"CANDIDATE"})
		rec := httptest.NewRecorder()

		chained.ServeHTTP(rec, req.WithContext(ctx))
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("User with required role succeeds (200)", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/admin/generate", nil)
		ctx := context.WithValue(req.Context(), middleware.RolesKey, []string{"EXAM_ADMIN", "REVIEWER"})
		rec := httptest.NewRecorder()

		chained.ServeHTTP(rec, req.WithContext(ctx))
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "admin_allowed", rec.Body.String())
	})
}
