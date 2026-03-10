package notification_test

import (
	"strings"
	"testing"

	"github.com/abuxton/pheromone/internal/notification"
)

// TestComputeHMACFormat verifies the "sha256=" prefix convention.
func TestComputeHMACFormat(t *testing.T) {
	sig := notification.ComputeHMAC([]byte("hello"), []byte("secret"))
	if !strings.HasPrefix(sig, "sha256=") {
		t.Fatalf("expected sig to start with \"sha256=\", got %q", sig)
	}
}

// TestVerifyHMACRoundtrip confirms that a freshly computed HMAC passes verification.
func TestVerifyHMACRoundtrip(t *testing.T) {
	payload := []byte(`{"action":"apply"}`)
	secret := []byte("my-signing-key")
	sig := notification.ComputeHMAC(payload, secret)
	if !notification.VerifyHMAC(payload, sig, secret) {
		t.Fatal("VerifyHMAC returned false for a valid roundtrip signature")
	}
}

// TestVerifyHMACTamperedPayload ensures a modified payload is rejected.
func TestVerifyHMACTamperedPayload(t *testing.T) {
	secret := []byte("my-signing-key")
	original := []byte(`{"action":"apply"}`)
	tampered := []byte(`{"action":"delete"}`)
	sig := notification.ComputeHMAC(original, secret)
	if notification.VerifyHMAC(tampered, sig, secret) {
		t.Fatal("VerifyHMAC returned true for a tampered payload")
	}
}

// TestVerifyHMACWrongSecret ensures a signature produced with a different secret
// is rejected.
func TestVerifyHMACWrongSecret(t *testing.T) {
	payload := []byte(`{"action":"apply"}`)
	sig := notification.ComputeHMAC(payload, []byte("correct-secret"))
	if notification.VerifyHMAC(payload, sig, []byte("wrong-secret")) {
		t.Fatal("VerifyHMAC returned true for a wrong secret")
	}
}

// TestVerifyHMACTamperedSig ensures a corrupted signature string is rejected.
func TestVerifyHMACTamperedSig(t *testing.T) {
	payload := []byte(`{"action":"apply"}`)
	secret := []byte("my-signing-key")
	sig := notification.ComputeHMAC(payload, secret)
	corrupted := sig[:len(sig)-4] + "XXXX"
	if notification.VerifyHMAC(payload, corrupted, secret) {
		t.Fatal("VerifyHMAC returned true for a corrupted signature")
	}
}
