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

// hashPassword creates a deterministic hash for (salt, password).
// The stored format is "sha256:<salt>:<hex-sha256(salt:password)>".
//
// Note: SHA-256 is used here for simplicity in the MVP without external
// dependencies. For production deployments with high-security requirements,
// consider migrating to bcrypt or argon2 via golang.org/x/crypto.
func hashPassword(password, salt string) string {
	h := sha256.New()
	h.Write([]byte(salt + ":" + password))
	return fmt.Sprintf("sha256:%s:%x", salt, h.Sum(nil))
}

// checkPassword verifies password against a stored hash string.
// The stored format must be "sha256:<salt>:<hex>".
func checkPassword(password, stored string) bool {
	parts := strings.SplitN(stored, ":", 3)
	if len(parts) != 3 || parts[0] != "sha256" {
		return false
	}
	salt := parts[1]
	return hashPassword(password, salt) == stored
}

// GeneratePasswordHash creates a ready-to-store hash for password using a random salt.
func GeneratePasswordHash(password string) (string, error) {
	salt, err := randomHex(16)
	if err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	return hashPassword(password, salt), nil
}

// randomHex returns n cryptographically random bytes encoded as a lowercase hex string.
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}
