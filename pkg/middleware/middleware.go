// Package middleware provides HTTP middleware for the AEGIS API gateway.
//
// Middleware is applied in the following order (outermost first):
//  1. Recovery (panic protection)
//  2. RequestID (correlation)
//  3. Logger (request/response logging)
//  4. CORS (cross-origin)
//  5. Timeout (request deadline)
//  6. RateLimit (throttling)
//  7. Auth (authentication)
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegis-platform/aegis/pkg/apperrors"
	"github.com/aegis-platform/aegis/pkg/httputil"
	"github.com/aegis-platform/aegis/pkg/logging"
)

// ──────────────────────────────────────────────
//  Context Keys
// ──────────────────────────────────────────────

type contextKey string

const (
	// RequestIDKey is the context key for the request ID.
	RequestIDKey contextKey = "request_id"
	// UserIDKey is the context key for the authenticated user ID.
	UserIDKey contextKey = "user_id"
	// OrganizationIDKey is the context key for the user's organization.
	OrganizationIDKey contextKey = "organization_id"
	// RolesKey is the context key for the user's roles.
	RolesKey contextKey = "roles"
)

// GetRequestID extracts the request ID from the context.
func GetRequestID(ctx context.Context) string {
	if v, ok := ctx.Value(RequestIDKey).(string); ok {
		return v
	}
	return ""
}

// GetUserID extracts the authenticated user ID from the context.
func GetUserID(ctx context.Context) string {
	if v, ok := ctx.Value(UserIDKey).(string); ok {
		return v
	}
	return ""
}

// GetOrganizationID extracts the organization ID from the context.
func GetOrganizationID(ctx context.Context) string {
	if v, ok := ctx.Value(OrganizationIDKey).(string); ok {
		return v
	}
	return ""
}

// ──────────────────────────────────────────────
//  Request ID Middleware
// ──────────────────────────────────────────────

// RequestID generates or propagates a unique request ID for correlation.
// If the incoming request has an X-Request-ID header, it is reused.
// Otherwise, a new UUID is generated.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Set on response headers for client correlation
		w.Header().Set("X-Request-ID", requestID)

		// Inject into context
		ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ──────────────────────────────────────────────
//  Recovery Middleware
// ──────────────────────────────────────────────

// Recovery catches panics in downstream handlers and returns a 500 error.
// Stack traces are logged but never exposed to clients.
func Recovery(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					requestID := GetRequestID(r.Context())
					stack := string(debug.Stack())

					logger.Error("panic recovered",
						zap.String("request_id", requestID),
						zap.String("method", r.Method),
						zap.String("path", r.URL.Path),
						zap.Any("panic", rec),
						zap.String("stack", stack),
					)

					err := apperrors.NewInternal("an unexpected error occurred", fmt.Errorf("panic: %v", rec))
					httputil.RespondError(w, r, err)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// ──────────────────────────────────────────────
//  Logger Middleware
// ──────────────────────────────────────────────

// responseWriter wraps http.ResponseWriter to capture status code and size.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

// Logger logs each HTTP request with duration, status, and size.
// It injects a request-scoped logger into the context.
func Logger(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			requestID := GetRequestID(r.Context())

			// Create request-scoped logger
			reqLogger := logger.With(
				zap.String("request_id", requestID),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("user_agent", r.UserAgent()),
			)

			// Inject logger into context
			ctx := logging.WithContext(r.Context(), reqLogger)

			// Wrap response writer to capture status
			rw := newResponseWriter(w)

			// Process request
			next.ServeHTTP(rw, r.WithContext(ctx))

			// Log completion
			duration := time.Since(start)
			fields := []zap.Field{
				zap.Int("status", rw.statusCode),
				zap.Int64("response_size", rw.written),
				zap.Duration("duration", duration),
			}

			if rw.statusCode >= 500 {
				reqLogger.Error("request completed with server error", fields...)
			} else if rw.statusCode >= 400 {
				reqLogger.Warn("request completed with client error", fields...)
			} else {
				reqLogger.Info("request completed", fields...)
			}
		})
	}
}

// ──────────────────────────────────────────────
//  CORS Middleware
// ──────────────────────────────────────────────

// CORSConfig holds CORS configuration.
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int // Preflight cache duration in seconds
}

// DefaultCORSConfig returns a restrictive default CORS config.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins:   []string{}, // Must be explicitly configured
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Request-ID", "Idempotency-Key"},
		ExposedHeaders:   []string{"X-Request-ID", "X-RateLimit-Remaining"},
		AllowCredentials: true,
		MaxAge:           3600,
	}
}

// CORS handles cross-origin resource sharing.
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	allowedOriginsSet := make(map[string]bool, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		allowedOriginsSet[o] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if origin != "" && allowedOriginsSet[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
				w.Header().Set("Access-Control-Expose-Headers", strings.Join(cfg.ExposedHeaders, ", "))
				if cfg.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", cfg.MaxAge))
			}

			// Handle preflight
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ──────────────────────────────────────────────
//  Timeout Middleware
// ──────────────────────────────────────────────

// Timeout sets a deadline on the request context.
func Timeout(duration time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), duration)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ──────────────────────────────────────────────
//  Security Headers Middleware
// ──────────────────────────────────────────────

// SecurityHeaders adds security-related HTTP headers to every response.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "0") // Disabled per modern best practice
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
		w.Header().Set("Pragma", "no-cache")

		next.ServeHTTP(w, r)
	})
}

// ──────────────────────────────────────────────
//  Chain Helper
// ──────────────────────────────────────────────

// Chain composes multiple middleware into a single middleware.
// Middleware is applied left-to-right (outermost first).
func Chain(middlewares ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(final http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}
