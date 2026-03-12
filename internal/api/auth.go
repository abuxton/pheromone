package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// tokenPayload is the claims encoded inside a bearer token.
type tokenPayload struct {
	Sub  string `json:"sub"`
	Role string `json:"role"`
	Exp  int64  `json:"exp"`
	Iat  int64  `json:"iat"`
}

// generateToken creates a signed bearer token for the given user.
// The token format is: base64url(json_payload).<hmac-sha256-hex>
func generateToken(username, role, secretKey string, ttl time.Duration) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(ttl)

	payload := tokenPayload{
		Sub:  username,
		Role: role,
		Exp:  exp.Unix(),
		Iat:  now.Unix(),
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("marshal token payload: %w", err)
	}

	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadJSON)

	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(encodedPayload)) //nolint:errcheck
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return encodedPayload + "." + sig, exp, nil
}

// validateToken verifies the signature and expiry of a bearer token.
// It returns the decoded claims on success.
func validateToken(token, secretKey string) (*tokenPayload, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return nil, errors.New("invalid token format")
	}

	encodedPayload, sig := parts[0], parts[1]

	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(encodedPayload)) //nolint:errcheck
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return nil, errors.New("invalid token signature")
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return nil, fmt.Errorf("decode token payload: %w", err)
	}

	var p tokenPayload
	if err := json.Unmarshal(payloadJSON, &p); err != nil {
		return nil, fmt.Errorf("unmarshal token payload: %w", err)
	}

	if time.Now().Unix() > p.Exp {
		return nil, errors.New("token expired")
	}

	return &p, nil
}

// hashPassword creates a deterministic hash for (salt, password) using SHA-256.
// The stored format is "sha256:<salt>:<hex-sha256(salt:password)>".
// This is kept for backward compatibility with existing configurations.
// New passwords should use GeneratePasswordHash which produces bcrypt hashes.
func hashPassword(password, salt string) string {
	h := sha256.New()
	h.Write([]byte(salt + ":" + password))
	return fmt.Sprintf("sha256:%s:%x", salt, h.Sum(nil))
}

// checkPassword verifies password against a stored hash string.
// It supports both bcrypt hashes (starting with "$2a$" or "$2b$") and the
// legacy SHA-256 format "sha256:<salt>:<hex>".
func checkPassword(password, stored string) bool {
	if strings.HasPrefix(stored, "$2a$") || strings.HasPrefix(stored, "$2b$") || strings.HasPrefix(stored, "$2y$") {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(password)) == nil
	}
	// Legacy sha256 format: "sha256:<salt>:<hex>"
	parts := strings.SplitN(stored, ":", 3)
	if len(parts) != 3 || parts[0] != "sha256" {
		return false
	}
	salt := parts[1]
	return hashPassword(password, salt) == stored
}

// bcryptCost is the default bcrypt work factor (12 is the recommended minimum for 2024+).
const bcryptCost = 12

// GeneratePasswordHash creates a ready-to-store bcrypt hash for password.
// The resulting hash is self-contained and does not require a separate salt field.
func GeneratePasswordHash(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash: %w", err)
	}
	return string(b), nil
}

// roleLevel returns the numeric privilege level for a role.
// Higher numbers mean more privileges.
// viewer is accepted as a legacy alias for observer.
func roleLevel(role string) int {
	switch strings.ToLower(role) {
	case "agent":
		return 1
	case "observer", "viewer":
		return 2
	case "operator":
		return 3
	case "admin":
		return 4
	default:
		return 0
	}
}

// hasRole returns true when the authenticated user's role is at least as
// privileged as the required role.
func hasRole(userRole, requiredRole string) bool {
	return roleLevel(userRole) >= roleLevel(requiredRole)
}

// hashAPIKey returns the SHA-256 hex digest of key.
// API keys are high-entropy random tokens, so SHA-256 is appropriate
// (unlike passwords where bcrypt is required). bcrypt's computational cost
// is unnecessary for random tokens and would impact performance.
func hashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", h)
}

// randomHex returns n cryptographically random bytes encoded as a lowercase hex string.
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand.Read: %w", err)
	}
	return fmt.Sprintf("%x", b), nil
}
