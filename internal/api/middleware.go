package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// contextKey is the unexported type for request context keys.
type contextKey int

const ctxUser contextKey = iota

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
		if user == nil || user.Role != "admin" {
			writeError(w, http.StatusForbidden, "admin role required")
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
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware records request method, path, status, and duration.
func loggingMiddleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		log.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration", time.Since(start).String(),
			"remote", r.RemoteAddr,
		)
	})
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
// A token is replenished every refillInterval up to a burst capacity of burstSize.
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

// rateLimitMiddleware applies a per-IP token-bucket rate limit (60 req/min, burst 20)
// using the per-server rate limiter map. Returns 429 Too Many Requests when exceeded.
func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}

		limiterI, _ := s.rateLimiters.LoadOrStore(ip, newIPRateLimiter(60, 20))
		limiter := limiterI.(*ipRateLimiter)

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
					l := value.(*ipRateLimiter)
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
