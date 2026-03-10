package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	return srv.corsMiddleware(srv.authMiddleware(mux))
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
