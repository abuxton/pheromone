package api

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/abuxton/pheromone/internal/config"
)

// newTestServer creates a Server with deterministic config and seeded data.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := config.UIConfig{
		Enabled:        true,
		Address:        "127.0.0.1",
		Port:           0,
		SecretKey:      "test-secret-key-for-unit-tests",
		TokenTTL:       time.Hour,
		AllowedOrigins: []string{"*"},
		Users: []config.UIUser{
			{Username: "admin", PasswordHash: hashPassword("admin", "testsalt"), Role: "admin"},
			{Username: "viewer", PasswordHash: hashPassword("viewer123", "testsalt2"), Role: "viewer"},
		},
	}
	srv := New(cfg, nil)
	srv.Seed()
	return srv
}

// newHandler returns the full handler chain for use in httptest.
func newHandler(srv *Server) http.Handler {
	mux := http.NewServeMux()
	srv.registerRoutes(mux)
	return srv.corsMiddleware(
		srv.rateLimitMiddleware(
			requestSizeLimitMiddleware(
				srv.authMiddleware(mux),
			),
		),
	)
}

func doJSON(t *testing.T, h http.Handler, method, path string, body interface{}, token string) *httptest.ResponseRecorder {
	t.Helper()
	var buf *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewBuffer(b)
	} else {
		buf = &bytes.Buffer{}
	}
	req := httptest.NewRequest(method, path, buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// login returns a valid Bearer token for the given credentials.
func login(t *testing.T, h http.Handler, username, password string) string {
	t.Helper()
	rr := doJSON(t, h, http.MethodPost, "/api/v1/auth/login",
		LoginRequest{Username: username, Password: password}, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("login failed: status %d, body: %s", rr.Code, rr.Body.String())
	}
	var resp LoginResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	return resp.Token
}

/* ── Health ── */

func TestHandleHealth(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	rr := doJSON(t, h, http.MethodGet, "/api/v1/health", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status ok, got %q", resp.Status)
	}
}

func TestHandleHealth_MethodNotAllowed(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	rr := doJSON(t, h, http.MethodPost, "/api/v1/health", nil, "")
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

/* ── Auth ── */

func TestHandleLogin_Success(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	rr := doJSON(t, h, http.MethodPost, "/api/v1/auth/login",
		LoginRequest{Username: "admin", Password: "admin"}, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp LoginResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Token == "" {
		t.Error("expected non-empty token")
	}
	if resp.User.Username != "admin" {
		t.Errorf("expected username admin, got %q", resp.User.Username)
	}
	if resp.User.Role != "admin" {
		t.Errorf("expected role admin, got %q", resp.User.Role)
	}
}

func TestHandleLogin_InvalidCredentials(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	rr := doJSON(t, h, http.MethodPost, "/api/v1/auth/login",
		LoginRequest{Username: "admin", Password: "wrongpassword"}, "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestHandleLogin_UnknownUser(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	rr := doJSON(t, h, http.MethodPost, "/api/v1/auth/login",
		LoginRequest{Username: "nobody", Password: "test"}, "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestHandleWhoami(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	tok := login(t, h, "admin", "admin")
	rr := doJSON(t, h, http.MethodGet, "/api/v1/auth/whoami", nil, tok)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var info UserInfo
	if err := json.NewDecoder(rr.Body).Decode(&info); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if info.Username != "admin" {
		t.Errorf("expected admin, got %q", info.Username)
	}
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	rr := doJSON(t, h, http.MethodGet, "/api/v1/agents", nil, "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	rr := doJSON(t, h, http.MethodGet, "/api/v1/agents", nil, "notavalidtoken")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

/* ── Stats ── */

func TestHandleStats(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	tok := login(t, h, "admin", "admin")
	rr := doJSON(t, h, http.MethodGet, "/api/v1/stats", nil, tok)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp StatsResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.AgentCount == 0 {
		t.Error("expected seeded agents, got 0")
	}
	if resp.TwinCount == 0 {
		t.Error("expected seeded twins, got 0")
	}
}

/* ── Agents ── */

func TestHandleAgents(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	tok := login(t, h, "admin", "admin")
	rr := doJSON(t, h, http.MethodGet, "/api/v1/agents", nil, tok)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var agents []*Agent
	if err := json.NewDecoder(rr.Body).Decode(&agents); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(agents) == 0 {
		t.Error("expected seeded agents")
	}
}

func TestHandleAgent_GetByID(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	tok := login(t, h, "admin", "admin")

	rr := doJSON(t, h, http.MethodGet, "/api/v1/agents/agent-os-01", nil, tok)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var a Agent
	if err := json.NewDecoder(rr.Body).Decode(&a); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if a.ID != "agent-os-01" {
		t.Errorf("expected agent-os-01, got %q", a.ID)
	}
}

func TestHandleAgent_NotFound(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	tok := login(t, h, "admin", "admin")
	rr := doJSON(t, h, http.MethodGet, "/api/v1/agents/nonexistent", nil, tok)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

/* ── Twins ── */

func TestHandleTwins(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	tok := login(t, h, "admin", "admin")
	rr := doJSON(t, h, http.MethodGet, "/api/v1/twins", nil, tok)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var twins []*Twin
	if err := json.NewDecoder(rr.Body).Decode(&twins); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(twins) == 0 {
		t.Error("expected seeded twins")
	}
}

func TestHandleTwin_GetByID(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	tok := login(t, h, "admin", "admin")
	rr := doJSON(t, h, http.MethodGet, "/api/v1/twins/twin-os-01", nil, tok)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var tw Twin
	if err := json.NewDecoder(rr.Body).Decode(&tw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if tw.ID != "twin-os-01" {
		t.Errorf("expected twin-os-01, got %q", tw.ID)
	}
}

func TestHandleTwinChangesets(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	tok := login(t, h, "admin", "admin")
	rr := doJSON(t, h, http.MethodGet, "/api/v1/twins/twin-os-02/changesets", nil, tok)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var cs []*Changeset
	if err := json.NewDecoder(rr.Body).Decode(&cs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(cs) == 0 {
		t.Error("expected changesets for twin-os-02")
	}
}

/* ── Skills ── */

func TestHandleSkills(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	tok := login(t, h, "admin", "admin")
	rr := doJSON(t, h, http.MethodGet, "/api/v1/skills", nil, tok)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var skills []*Skill
	if err := json.NewDecoder(rr.Body).Decode(&skills); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(skills) == 0 {
		t.Error("expected seeded skills")
	}
}

/* ── Groups ── */

func TestHandleGroups_List(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	tok := login(t, h, "admin", "admin")
	rr := doJSON(t, h, http.MethodGet, "/api/v1/groups", nil, tok)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var groups []*Group
	if err := json.NewDecoder(rr.Body).Decode(&groups); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(groups) == 0 {
		t.Error("expected seeded groups")
	}
}

func TestHandleGroups_Create(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	tok := login(t, h, "admin", "admin")
	rr := doJSON(t, h, http.MethodPost, "/api/v1/groups",
		CreateGroupRequest{Name: "Test Group", Type: "agents", Description: "A test group"}, tok)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var g Group
	if err := json.NewDecoder(rr.Body).Decode(&g); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if g.Name != "Test Group" {
		t.Errorf("expected name 'Test Group', got %q", g.Name)
	}
}

func TestHandleGroups_Create_InvalidType(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	tok := login(t, h, "admin", "admin")
	rr := doJSON(t, h, http.MethodPost, "/api/v1/groups",
		CreateGroupRequest{Name: "Bad", Type: "invalid"}, tok)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleGroup_Delete(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	tok := login(t, h, "admin", "admin")

	// Create then delete
	rr := doJSON(t, h, http.MethodPost, "/api/v1/groups",
		CreateGroupRequest{Name: "Temp", Type: "agents"}, tok)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create failed: %d", rr.Code)
	}
	var g Group
	json.NewDecoder(rr.Body).Decode(&g) //nolint:errcheck

	rr = doJSON(t, h, http.MethodDelete, "/api/v1/groups/"+g.ID, nil, tok)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify deleted
	rr = doJSON(t, h, http.MethodGet, "/api/v1/groups/"+g.ID, nil, tok)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after deletion, got %d", rr.Code)
	}
}

/* ── Changesets ── */

func TestHandleChangesets(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	tok := login(t, h, "admin", "admin")
	rr := doJSON(t, h, http.MethodGet, "/api/v1/changesets", nil, tok)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var cs []*Changeset
	if err := json.NewDecoder(rr.Body).Decode(&cs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(cs) == 0 {
		t.Error("expected seeded changesets")
	}
}

/* ── Connections ── */

func TestHandleConnections(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	tok := login(t, h, "admin", "admin")
	rr := doJSON(t, h, http.MethodGet, "/api/v1/connections", nil, tok)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var conns []*Connection
	if err := json.NewDecoder(rr.Body).Decode(&conns); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(conns) == 0 {
		t.Error("expected seeded connections")
	}
}

/* ── Users (admin only) ── */

func TestHandleUsers_AdminAccess(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	tok := login(t, h, "admin", "admin")
	rr := doJSON(t, h, http.MethodGet, "/api/v1/users", nil, tok)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleUsers_ViewerForbidden(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	tok := login(t, h, "viewer", "viewer123")
	rr := doJSON(t, h, http.MethodGet, "/api/v1/users", nil, tok)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

/* ── Auth helpers ── */

func TestGenerateAndValidateToken(t *testing.T) {
	tok, exp, err := generateToken("alice", "admin", "secret", time.Hour)
	if err != nil {
		t.Fatalf("generateToken: %v", err)
	}
	if tok == "" {
		t.Fatal("expected non-empty token")
	}
	if exp.Before(time.Now()) {
		t.Error("expected future expiry")
	}

	payload, err := validateToken(tok, "secret")
	if err != nil {
		t.Fatalf("validateToken: %v", err)
	}
	if payload.Sub != "alice" {
		t.Errorf("expected sub alice, got %q", payload.Sub)
	}
	if payload.Role != "admin" {
		t.Errorf("expected role admin, got %q", payload.Role)
	}
}

func TestValidateToken_WrongSecret(t *testing.T) {
	tok, _, _ := generateToken("bob", "viewer", "correct-secret", time.Hour)
	_, err := validateToken(tok, "wrong-secret")
	if err == nil {
		t.Error("expected error for wrong secret")
	}
}

func TestCheckPassword(t *testing.T) {
	hash := hashPassword("hunter2", "abc123")
	if !checkPassword("hunter2", hash) {
		t.Error("expected checkPassword to return true for correct password")
	}
	if checkPassword("wrong", hash) {
		t.Error("expected checkPassword to return false for wrong password")
	}
}

func TestGeneratePasswordHash(t *testing.T) {
	hash, err := GeneratePasswordHash("testpassword")
	if err != nil {
		t.Fatalf("GeneratePasswordHash: %v", err)
	}
	if !checkPassword("testpassword", hash) {
		t.Error("checkPassword returned false for generated hash")
	}
	if checkPassword("wrongpassword", hash) {
		t.Error("checkPassword returned true for wrong password")
	}
}

/* ── CORS ── */

func TestCORSPreflightReturns204(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/agents", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for OPTIONS, got %d", rr.Code)
	}
}

func TestCORSReflectsSpecificOrigin(t *testing.T) {
	cfg := config.UIConfig{
		Enabled:        true,
		Address:        "127.0.0.1",
		Port:           0,
		SecretKey:      "test-secret",
		TokenTTL:       time.Hour,
		AllowedOrigins: []string{"https://console.example.com"},
		Users:          []config.UIUser{{Username: "admin", PasswordHash: hashPassword("admin", "s"), Role: "admin"}},
	}
	srv := New(cfg, nil)
	h := newHandler(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Origin", "https://console.example.com")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	got := rr.Header().Get("Access-Control-Allow-Origin")
	if got != "https://console.example.com" {
		t.Errorf("expected specific origin reflection, got %q", got)
	}
}

func TestCORSBlocksUnknownOrigin(t *testing.T) {
	cfg := config.UIConfig{
		Enabled:        true,
		Address:        "127.0.0.1",
		Port:           0,
		SecretKey:      "test-secret",
		TokenTTL:       time.Hour,
		AllowedOrigins: []string{"https://console.example.com"},
		Users:          []config.UIUser{{Username: "admin", PasswordHash: hashPassword("admin", "s"), Role: "admin"}},
	}
	srv := New(cfg, nil)
	h := newHandler(srv)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Origin", "https://attacker.example.com")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	got := rr.Header().Get("Access-Control-Allow-Origin")
	if got == "https://attacker.example.com" || got == "*" {
		t.Errorf("unexpected CORS header for unknown origin: %q", got)
	}
}

/* ── TLS ── */

// generateSelfSignedCert writes a self-signed certificate and private key to dir.
// Returns the cert path, key path, and the DER-encoded certificate bytes.
func generateSelfSignedCert(t *testing.T, dir string) (certPath, keyPath string, certDER []byte) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "pheromone-test"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certPath = filepath.Join(dir, "server.crt")
	f, err := os.Create(certPath)
	if err != nil {
		t.Fatalf("create cert file: %v", err)
	}
	if err := pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		t.Fatalf("encode cert: %v", err)
	}
	f.Close()

	keyPath = filepath.Join(dir, "server.key")
	fk, err := os.Create(keyPath)
	if err != nil {
		t.Fatalf("create key file: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	if err := pem.Encode(fk, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}); err != nil {
		t.Fatalf("encode key: %v", err)
	}
	fk.Close()

	return certPath, keyPath, der
}

// TestListenAndServe_TLS starts the server with a self-signed cert and verifies
// the health endpoint is reachable over HTTPS.
func TestListenAndServe_TLS(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath, certDER := generateSelfSignedCert(t, dir)

	cfg := config.UIConfig{
		Enabled:        true,
		Address:        "127.0.0.1",
		Port:           0, // assigned by OS
		SecretKey:      "test-secret-tls",
		TokenTTL:       time.Hour,
		AllowedOrigins: []string{"*"},
		Users: []config.UIUser{
			{Username: "admin", PasswordHash: hashPassword("admin", "salt1"), Role: "admin"},
		},
		TLS: config.UITLSConfig{
			Enabled:  true,
			CertFile: certPath,
			KeyFile:  keyPath,
		},
	}

	srv := New(cfg, nil)

	// Find a free port by binding temporarily.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	cfg.Port = port
	srv = New(cfg, nil)
	srv.Seed()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe(ctx)
	}()

	// Give the server a moment to start.
	time.Sleep(100 * time.Millisecond)

	// Build an HTTP client that trusts our self-signed cert.
	certPool := x509.NewCertPool()
	certPool.AddCert(func() *x509.Certificate {
		c, err := x509.ParseCertificate(certDER)
		if err != nil {
			t.Fatalf("parse cert: %v", err)
		}
		return c
	}())
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: certPool},
		},
	}

	url := "https://127.0.0.1:" + itoa(port) + "/api/v1/health"
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("HTTPS GET health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// Shut down gracefully.
	cancel()
	if err := <-errCh; err != nil {
		t.Logf("server stopped with: %v", err)
	}
}

// TestBuildTLSConfig_InvalidCert verifies that buildTLSConfig returns an error
// when the cert file does not exist.
func TestBuildTLSConfig_InvalidCert(t *testing.T) {
	cfg := config.UIConfig{
		Enabled:   true,
		Port:      8081,
		SecretKey: "secret",
		TLS: config.UITLSConfig{
			Enabled:  true,
			CertFile: "/nonexistent/server.crt",
			KeyFile:  "/nonexistent/server.key",
		},
	}
	srv := New(cfg, nil)
	if _, err := srv.buildTLSConfig(); err == nil {
		t.Fatal("expected error for nonexistent cert/key files")
	}
}

// TestBuildTLSConfig_Disabled verifies that buildTLSConfig returns nil when TLS is off.
func TestBuildTLSConfig_Disabled(t *testing.T) {
	cfg := config.UIConfig{
		Enabled:   true,
		Port:      8081,
		SecretKey: "secret",
		TLS:       config.UITLSConfig{Enabled: false},
	}
	srv := New(cfg, nil)
	tc, err := srv.buildTLSConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tc != nil {
		t.Error("expected nil tls.Config when TLS is disabled")
	}
}

// itoa converts an int to string for use in test URLs.
func itoa(n int) string {
	return strconv.Itoa(n)
}

/* ── k8s health probes ── */

func TestHandleHealthz(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	rr := doJSON(t, h, http.MethodGet, "/healthz", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp ProbeResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status ok, got %q", resp.Status)
	}
}

func TestHandleHealthz_MethodNotAllowed(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	rr := doJSON(t, h, http.MethodPost, "/healthz", nil, "")
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestHandleReadyz_Ready(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	rr := doJSON(t, h, http.MethodGet, "/readyz", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp ProbeResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status ok, got %q", resp.Status)
	}
}

func TestHandleReadyz_NotReady(t *testing.T) {
	srv := newTestServer(t)
	srv.ready.Store(false) // simulate pre-startup state
	h := newHandler(srv)
	rr := doJSON(t, h, http.MethodGet, "/readyz", nil, "")
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleReadyz_MethodNotAllowed(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	rr := doJSON(t, h, http.MethodPost, "/readyz", nil, "")
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

// TestHandleHealthz_NoAuthRequired verifies probes are accessible without a token.
func TestHandleHealthz_NoAuthRequired(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	rr := doJSON(t, h, http.MethodGet, "/healthz", nil, "") // no token
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 without auth, got %d", rr.Code)
	}
}

func TestHandleReadyz_NoAuthRequired(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	rr := doJSON(t, h, http.MethodGet, "/readyz", nil, "") // no token
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 without auth, got %d", rr.Code)
	}
}

/* ── SSE events endpoint ── */

func TestHandleEvents_MethodNotAllowed(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	tok := login(t, h, "admin", "admin")
	rr := doJSON(t, h, http.MethodPost, "/api/v1/events", nil, tok)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestHandleEvents_RequiresAuth(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	rr := doJSON(t, h, http.MethodGet, "/api/v1/events", nil, "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rr.Code)
	}
}

// TestHandleEvents_SSEHeaders verifies that the SSE endpoint sets the correct
// Content-Type and that the initial ": connected" comment is written.
func TestHandleEvents_SSEHeaders(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)
	tok := login(t, h, "admin", "admin")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	ctx, cancel := context.WithTimeout(req.Context(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	ct := rr.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %q", ct)
	}
	body := rr.Body.String()
	if len(body) == 0 {
		t.Error("expected non-empty SSE body (at least the connected comment)")
	}
}

/* ── Request size limit middleware ── */

func TestRequestSizeLimit_Rejected(t *testing.T) {
	srv := newTestServer(t)
	h := newHandler(srv)

	// Build a body larger than defaultMaxBodyBytes by setting Content-Length.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		bytes.NewReader(make([]byte, defaultMaxBodyBytes+1)))
	req.ContentLength = defaultMaxBodyBytes + 1
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rr.Code)
	}
}

// TestDecodeJSON_MaxBytesError verifies that decodeJSON maps *http.MaxBytesError
// to 413 (not 400) when the body exceeds the limit applied by MaxBytesReader.
func TestDecodeJSON_MaxBytesError(t *testing.T) {
	// Build a body that starts as a valid JSON object but exceeds the size
	// limit while the decoder is still reading a string field value.
	// A valid JSON prefix ensures the decoder doesn't fail with a syntax error
	// before it reads past the MaxBytesReader limit.
	prefix := []byte(`{"username":"`)
	padding := bytes.Repeat([]byte("a"), defaultMaxBodyBytes) // enough to exceed limit
	body := append(prefix, padding...)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	r.ContentLength = -1 // unknown / chunked length
	r.Body = http.MaxBytesReader(w, r.Body, defaultMaxBodyBytes)

	var req LoginRequest
	if decodeJSON(w, r, &req) {
		t.Fatal("expected decodeJSON to return false for oversized body")
	}
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", w.Code, w.Body.String())
	}
}

/* ── Rate limit middleware ── */

func TestRateLimitMiddleware_AllowsNormalTraffic(t *testing.T) {
	// A fresh limiter allows up to burst requests immediately.
	l := newIPRateLimiter(60, 20)
	for i := 0; i < 20; i++ {
		if !l.allow() {
			t.Fatalf("request %d was unexpectedly rate-limited", i+1)
		}
	}
}

func TestRateLimitMiddleware_BlocksWhenExhausted(t *testing.T) {
	l := newIPRateLimiter(60, 5)
	for i := 0; i < 5; i++ {
		l.allow()
	}
	if l.allow() {
		t.Error("expected rate limit to block after burst exhausted")
	}
}

/* ── clientIP helper ── */

func TestClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.1:12345"
	if got := clientIP(req); got != "192.0.2.1" {
		t.Errorf("expected 192.0.2.1, got %q", got)
	}
}

func TestClientIP_XForwardedFor_Single(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.5")
	if got := clientIP(req); got != "203.0.113.5" {
		t.Errorf("expected 203.0.113.5, got %q", got)
	}
}

func TestClientIP_XForwardedFor_Chain(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.1, 10.0.0.2")
	// leftmost entry is the originating client
	if got := clientIP(req); got != "203.0.113.5" {
		t.Errorf("expected 203.0.113.5, got %q", got)
	}
}

// TestBroadcast verifies that events published via broadcast reach SSE subscribers.
func TestBroadcast(t *testing.T) {
	srv := newTestServer(t)

	ch := make(chan []byte, 4)
	srv.eventMu.Lock()
	srv.eventClients[ch] = struct{}{}
	srv.eventMu.Unlock()
	defer func() {
		srv.eventMu.Lock()
		delete(srv.eventClients, ch)
		srv.eventMu.Unlock()
	}()

	srv.broadcast(Event{Type: "agent.status_changed", Payload: map[string]string{"id": "agent-os-01"}})

	select {
	case frame := <-ch:
		if len(frame) == 0 {
			t.Error("expected non-empty SSE frame")
		}
		// Frame must start with "data: "
		s := string(frame)
		if len(s) < 8 || s[:6] != "data: " {
			t.Errorf("unexpected SSE frame prefix: %q", s[:min(len(s), 20)])
		}
	case <-time.After(time.Second):
		t.Fatal("broadcast event not received within 1s")
	}
}
