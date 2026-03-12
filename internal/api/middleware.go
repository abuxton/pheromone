package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// contextKey is the unexported type for request context keys.
type contextKey int

const (
	ctxUser      contextKey = iota // authenticated user payload
	ctxRequestID                   // unique request identifier
)

// authMiddleware validates Bearer tokens on every request except the auth and
// health endpoints. On success the tokenPayload is stored in the request context.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow unauthenticated access to login, health, and k8s probe endpoints.
		if r.URL.Path == "/api/v1/auth/login" ||
			r.URL.Path == "/api/v1/health" ||
			r.URL.Path == "/healthz" ||
			r.URL.Path == "/readyz" ||
			!strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeError(w, http.StatusUnauthorized, "authorization header required")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			writeError(w, http.StatusUnauthorized, "bearer token required")
			return
		}

		payload, err := validateToken(parts[1], s.cfg.SecretKey)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}

		ctx := context.WithValue(r.Context(), ctxUser, payload)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// adminMiddleware rejects requests from non-admin users.
func adminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := userFromContext(r.Context())
		if user == nil || !hasRole(user.Role, "admin") {
			writeError(w, http.StatusForbidden, "admin role required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireRole returns middleware that rejects requests from users without the minimum role.
func requireRole(minRole string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := userFromContext(r.Context())
		if user == nil || !hasRole(user.Role, minRole) {
			writeError(w, http.StatusForbidden, minRole+" role required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware adds CORS headers based on the server's allowed origins list.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowOrigin := ""
		for _, o := range s.cfg.AllowedOrigins {
			if o == "*" {
				allowOrigin = "*"
				break
			}
			if o == origin {
				allowOrigin = origin
				break
			}
		}
		if allowOrigin == "" && len(s.cfg.AllowedOrigins) == 0 {
			allowOrigin = "*"
		}
		if allowOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware records request method, path, status, duration, and request ID.
func loggingMiddleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		log.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
			"request_id", requestIDFromContext(r.Context()),
			"component", "api",
		)
	})
}

// requestIDMiddleware generates a unique request ID for each HTTP request,
// stores it in the request context, and echoes it back via the X-Request-ID
// response header. If the incoming request carries a valid X-Request-ID header
// (ASCII printable, max 128 chars) its value is used as-is, enabling end-to-end
// correlation across services. Invalid or oversized header values are replaced
// with a freshly generated ID to prevent log injection and unbounded cardinality.
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if !isValidRequestID(id) {
			id = newRequestID()
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), ctxRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// maxRequestIDLen is the maximum permitted length for an incoming X-Request-ID header.
const maxRequestIDLen = 128

// isValidRequestID returns true when id is non-empty, at most maxRequestIDLen bytes,
// and every byte is an ASCII printable character (0x21–0x7E, i.e. no spaces,
// control characters, or multi-byte UTF-8 sequences). The byte-level check is
// intentional: it ensures IDs are safe for use in HTTP headers and log lines
// without any encoding ambiguity.
func isValidRequestID(id string) bool {
	if id == "" || len(id) > maxRequestIDLen {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if c < 0x21 || c > 0x7E {
			return false
		}
	}
	return true
}

// requestIDFromContext retrieves the request ID from ctx, or "" if not set.
func requestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxRequestID).(string)
	return v
}

// newRequestID generates a random 8-byte hex identifier.
// In the extremely unlikely event that crypto/rand fails, a fixed sentinel
// value beginning with "errgen-" is returned so that operators can identify
// entries where entropy was unavailable rather than mistaking them for real IDs.
func newRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "errgen-00000000"
	}
	return hex.EncodeToString(b)
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// writeJSON encodes v as JSON and writes it to w with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a standard JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{
		Error:   http.StatusText(status),
		Code:    status,
		Message: msg,
	})
}

// userFromContext retrieves the authenticated user from the request context.
func userFromContext(ctx context.Context) *tokenPayload {
	v := ctx.Value(ctxUser)
	if v == nil {
		return nil
	}
	p, _ := v.(*tokenPayload)
	return p
}

// decodeJSON decodes JSON from r.Body into v.
// If the body exceeds the limit set by requestSizeLimitMiddleware it returns
// 413; any other decode failure returns 400. Returns true on success.
func decodeJSON(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return false
		}
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

// defaultMaxBodyBytes is the default maximum request body size (1 MiB).
const defaultMaxBodyBytes = 1 << 20 // 1 MiB

// requestSizeLimitMiddleware rejects request bodies that exceed defaultMaxBodyBytes.
func requestSizeLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil && r.ContentLength > defaultMaxBodyBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, defaultMaxBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// ipRateLimiter is a simple per-IP token-bucket rate limiter backed by stdlib primitives.
// Tokens are continuously replenished at refillPerSec tokens/second up to a
// maximum of burstSize, calculated from elapsed wall-clock time on each call.
type ipRateLimiter struct {
	mu           sync.Mutex
	tokens       float64
	lastRefill   time.Time
	burstSize    float64
	refillPerSec float64
	lastAccess   time.Time
}

func newIPRateLimiter(requestsPerMinute float64, burst float64) *ipRateLimiter {
	return &ipRateLimiter{
		tokens:       burst,
		lastRefill:   time.Now(),
		lastAccess:   time.Now(),
		burstSize:    burst,
		refillPerSec: requestsPerMinute / 60.0,
	}
}

// allow reports whether a request should be permitted. It refills tokens based
// on elapsed time and consumes one token per call.
func (l *ipRateLimiter) allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(l.lastRefill).Seconds()
	l.tokens += elapsed * l.refillPerSec
	if l.tokens > l.burstSize {
		l.tokens = l.burstSize
	}
	l.lastRefill = now
	l.lastAccess = now

	if l.tokens >= 1 {
		l.tokens--
		return true
	}
	return false
}

// clientIP returns the originating client IP for rate-limiting purposes.
// When an X-Forwarded-For header is present, the leftmost (originating) address
// is used. This is only safe when the server runs behind a trusted reverse proxy
// that sets and controls the X-Forwarded-For header. In direct-connection
// deployments, or when the proxy is not trusted, X-Forwarded-For can be
// spoofed — a future enhancement should gate this on a configured trusted-proxy
// CIDR list (see ADR-017 follow-up actions).
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// XFF format: "client, proxy1, proxy2" — take the leftmost entry.
		if i := strings.Index(xff, ","); i != -1 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// rateLimitMiddleware applies a per-IP token-bucket rate limit (60 req/min, burst 20)
// using the per-server rate limiter map. Returns 429 Too Many Requests when exceeded.
func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)

		limiterI, _ := s.rateLimiters.LoadOrStore(ip, newIPRateLimiter(60, 20))
		limiter, ok := limiterI.(*ipRateLimiter)
		if !ok || limiter == nil {
			// This should never happen; all values stored in the map are *ipRateLimiter.
			next.ServeHTTP(w, r)
			return
		}

		if !limiter.allow() {
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// startRateLimitEviction launches the background goroutine that evicts stale
// per-IP rate limit buckets. It should be called exactly once per Server, from
// ListenAndServe. The goroutine exits when ctx is cancelled.
func (s *Server) startRateLimitEviction(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				expiry := time.Now().Add(-10 * time.Minute)
				s.rateLimiters.Range(func(key, value interface{}) bool {
					l, ok := value.(*ipRateLimiter)
					if !ok || l == nil {
						return true
					}
					l.mu.Lock()
					stale := l.lastAccess.Before(expiry)
					l.mu.Unlock()
					if stale {
						s.rateLimiters.Delete(key)
					}
					return true
				})
			}
		}
	}()
}
