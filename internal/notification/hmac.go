// Package notification implements the Pheromone NotificationSkill and supporting
// utilities defined in ADR-011 (post-action hooks).
//
// It provides HMAC-SHA256 webhook signing, a fire-and-forget NotifyDirect path, and
// a stub NotifyServer path (gRPC delivery, Phase 2).
package notification

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// ComputeHMAC returns an "sha256=<hex>" HMAC-SHA256 signature of payload using secret.
// The prefix matches the GitHub webhook signature convention and is validated by
// VerifyHMAC.
func ComputeHMAC(payload, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// VerifyHMAC reports whether sig equals ComputeHMAC(payload, secret).
// The comparison is constant-time to prevent timing-based side-channel attacks.
func VerifyHMAC(payload []byte, sig string, secret []byte) bool {
	expected := ComputeHMAC(payload, secret)
	return hmac.Equal([]byte(sig), []byte(expected))
}
