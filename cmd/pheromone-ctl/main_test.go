package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// newTestServer returns a minimal httptest.Server that mimics the pheromone
// management API. It seeds a fixed set of demo resources so the CLI commands
// can be exercised without a real server.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	registerTestRoutes(mux)
	return httptest.NewServer(mux)
}

// writeJSON is a small test helper – the production helpers live in api package.
func testWriteJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func registerTestRoutes(mux *http.ServeMux) {
	now := time.Now().UTC()

	// Auth
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Username != "admin" || req.Password != "admin" {
			testWriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
			return
		}
		testWriteJSON(w, http.StatusOK, map[string]interface{}{
			"token":      "test-token-abc123",
			"expires_at": now.Add(24 * time.Hour),
			"user":       map[string]string{"username": "admin", "role": "admin"},
		})
	})

	mux.HandleFunc("/api/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		testWriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/v1/auth/whoami", func(w http.ResponseWriter, r *http.Request) {
		testWriteJSON(w, http.StatusOK, userRow{Username: "admin", Role: "admin"})
	})

	// Health / Stats
	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		testWriteJSON(w, http.StatusOK, map[string]interface{}{
			"status":    "ok",
			"version":   "0.1.0",
			"uptime":    "1h0m0s",
			"timestamp": now,
		})
	})

	mux.HandleFunc("/api/v1/stats", func(w http.ResponseWriter, r *http.Request) {
		testWriteJSON(w, http.StatusOK, map[string]interface{}{
			"agent_count": 4, "twin_count": 4,
			"online_agents": 3, "active_twins": 3,
			"skill_count": 5, "group_count": 3,
			"pending_changes": 2, "connection_count": 4,
		})
	})

	// Agents
	agents := []agentRow{
		{ID: "agent-os-01", Name: "web-server-01", Type: "os", Status: "online",
			Hostname: "web-server-01.internal", LastSeen: now.Add(-5 * time.Second)},
		{ID: "agent-wl-01", Name: "nginx-agent-01", Type: "workload", Status: "online",
			Hostname: "web-server-01.internal", LastSeen: now.Add(-2 * time.Second)},
	}
	mux.HandleFunc("/api/v1/agents", func(w http.ResponseWriter, r *http.Request) {
		testWriteJSON(w, http.StatusOK, agents)
	})
	mux.HandleFunc("/api/v1/agents/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/api/v1/agents/"):]
		for _, a := range agents {
			if a.ID == id {
				testWriteJSON(w, http.StatusOK, a)
				return
			}
		}
		testWriteJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	})

	// Twins
	twins := []twinRow{
		{ID: "twin-os-01", Name: "web-server-01 OS", Type: "os", State: "active",
			AgentID: "agent-os-01", ConfigVersion: 5, LastHeartbeat: now.Add(-5 * time.Second)},
		{ID: "twin-wl-02", Name: "postgresql-service", Type: "workload", State: "stale",
			AgentID: "agent-wl-02", ConfigVersion: 7, LastHeartbeat: now.Add(-5 * time.Minute)},
	}
	mux.HandleFunc("/api/v1/twins", func(w http.ResponseWriter, r *http.Request) {
		testWriteJSON(w, http.StatusOK, twins)
	})
	mux.HandleFunc("/api/v1/twins/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path[len("/api/v1/twins/"):]
		if len(path) > 0 && path[len(path)-1] == '/' {
			path = path[:len(path)-1]
		}
		// /api/v1/twins/{id}/changesets
		if idx := len(path) - len("/changesets"); idx > 0 && path[idx:] == "/changesets" {
			testWriteJSON(w, http.StatusOK, []changesetRow{
				{ID: "cs-001", TwinID: "twin-os-02", AgentID: "agent-os-02",
					Type: "config", Status: "pending",
					Changes:   []fieldChange{{Field: "kernel", OldValue: "old", NewValue: "new"}},
					CreatedAt: now.Add(-10 * time.Minute)},
			})
			return
		}
		for _, t := range twins {
			if t.ID == path {
				testWriteJSON(w, http.StatusOK, t)
				return
			}
		}
		testWriteJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	})

	// Skills
	skills := []skillRow{
		{Name: "digital-twin", Version: "1.0.0", MinTwinLevel: "os", Description: "Twin state", AgentCount: 3},
		{Name: "metrics", Version: "1.0.0", MinTwinLevel: "workload", Description: "Metrics", AgentCount: 4},
	}
	mux.HandleFunc("/api/v1/skills", func(w http.ResponseWriter, r *http.Request) {
		testWriteJSON(w, http.StatusOK, skills)
	})

	// Groups
	groups := []groupRow{
		{ID: "grp-001", Name: "Production Servers", Type: "infrastructure",
			Description: "All prod nodes", Members: []string{"host1", "host2"},
			TwinNamespace: "prod", CreatedAt: now.Add(-72 * time.Hour), UpdatedAt: now.Add(-72 * time.Hour)},
	}
	mux.HandleFunc("/api/v1/groups", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			testWriteJSON(w, http.StatusOK, groups)
		case http.MethodPost:
			var req map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&req)
			g := groupRow{
				ID: "grp-new", Name: req["name"].(string),
				Type: req["type"].(string), CreatedAt: now, UpdatedAt: now,
			}
			testWriteJSON(w, http.StatusCreated, g)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/v1/groups/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/api/v1/groups/"):]
		switch r.Method {
		case http.MethodGet:
			for _, g := range groups {
				if g.ID == id {
					testWriteJSON(w, http.StatusOK, g)
					return
				}
			}
			testWriteJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		case http.MethodPut:
			testWriteJSON(w, http.StatusOK, groupRow{ID: id, Name: "updated", Type: "agents",
				CreatedAt: now, UpdatedAt: now})
		case http.MethodDelete:
			testWriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Changesets
	changesets := []changesetRow{
		{ID: "cs-001", TwinID: "twin-os-02", AgentID: "agent-os-02",
			Type: "config", Status: "pending",
			Changes:   []fieldChange{{Field: "kernel", OldValue: "old", NewValue: "new"}},
			CreatedAt: now.Add(-10 * time.Minute)},
		{ID: "cs-002", TwinID: "twin-wl-02", AgentID: "agent-wl-02",
			Type: "state", Status: "pending",
			Changes:   []fieldChange{{Field: "status", OldValue: "degraded", NewValue: "running"}},
			CreatedAt: now.Add(-4 * time.Minute)},
	}
	mux.HandleFunc("/api/v1/changesets", func(w http.ResponseWriter, r *http.Request) {
		testWriteJSON(w, http.StatusOK, changesets)
	})
	mux.HandleFunc("/api/v1/changesets/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/api/v1/changesets/"):]
		for _, cs := range changesets {
			if cs.ID == id {
				testWriteJSON(w, http.StatusOK, cs)
				return
			}
		}
		testWriteJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	})

	// Connections
	mux.HandleFunc("/api/v1/connections", func(w http.ResponseWriter, r *http.Request) {
		testWriteJSON(w, http.StatusOK, []connectionRow{
			{ID: "conn-001", AgentID: "agent-os-01", TwinID: "twin-os-01",
				Status: "connected", Since: now.Add(-72 * time.Hour), LastSync: now.Add(-5 * time.Second)},
			{ID: "conn-004", AgentID: "agent-wl-02", TwinID: "twin-wl-02",
				Status: "disconnected", Since: now.Add(-48 * time.Hour), LastSync: now.Add(-5 * time.Minute)},
		})
	})

	// Users (admin)
	users := []userRow{
		{Username: "admin", Role: "admin", DisplayName: "Administrator", Email: "admin@example.com"},
		{Username: "operator", Role: "operator", DisplayName: "Operator", Email: "op@example.com"},
	}
	mux.HandleFunc("/api/v1/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			testWriteJSON(w, http.StatusOK, users)
		case http.MethodPost:
			var req map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&req)
			u := userRow{
				Username: req["username"].(string),
				Role:     req["role"].(string),
			}
			testWriteJSON(w, http.StatusCreated, u)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/v1/users/", func(w http.ResponseWriter, r *http.Request) {
		// Determine sub-path: /api/v1/users/{username}[/api-keys[/{id}]]
		sub := r.URL.Path[len("/api/v1/users/"):]

		// api-keys sub-resource
		if idx := indexOf(sub, "/api-keys"); idx >= 0 {
			rest := sub[idx+len("/api-keys"):]

			// DELETE /api/v1/users/{username}/api-keys/{key-id}
			if rest != "" && rest != "/" {
				testWriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
				return
			}

			// GET or POST /api/v1/users/{username}/api-keys
			switch r.Method {
			case http.MethodGet:
				testWriteJSON(w, http.StatusOK, []apiKeyRow{
					{ID: "key-001", Name: "CI key", KeyPrefix: "ak_abc",
						CreatedAt: now.Add(-24 * time.Hour)},
				})
			case http.MethodPost:
				var req map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&req)
				testWriteJSON(w, http.StatusCreated, map[string]interface{}{
					"key":     "ak_full_secret_key",
					"api_key": apiKeyRow{ID: "key-new", Name: req["name"].(string), KeyPrefix: "ak_new", CreatedAt: now},
				})
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		// /api/v1/users/{username}
		username := sub
		switch r.Method {
		case http.MethodGet:
			for _, u := range users {
				if u.Username == username {
					testWriteJSON(w, http.StatusOK, u)
					return
				}
			}
			testWriteJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		case http.MethodPut:
			testWriteJSON(w, http.StatusOK, userRow{Username: username, Role: "operator"})
		case http.MethodDelete:
			testWriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Audit
	mux.HandleFunc("/api/v1/audit", func(w http.ResponseWriter, r *http.Request) {
		testWriteJSON(w, http.StatusOK, []auditRow{
			{ID: "ae-001", Timestamp: now.Add(-1 * time.Hour),
				UserID: "admin", Role: "admin", Operation: "create_user",
				ResourceID: "alice", Result: "success"},
		})
	})
}

// indexOf returns the index of substr in s using strings.Index, or -1 if not found.
func indexOf(s, substr string) int {
	return strings.Index(s, substr)
}

// -------------------------------------------------------------------------
// Helper: capture stdout during a test run.
// -------------------------------------------------------------------------

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf [4096]byte
	n, _ := r.Read(buf[:])
	r.Close()
	return string(buf[:n])
}

// -------------------------------------------------------------------------
// Tests
// -------------------------------------------------------------------------

func TestRunNoArgs(t *testing.T) {
	code := run([]string{})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	code := run([]string{"--server", "http://localhost:9999", "notacommand"})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

func TestRunHelp(t *testing.T) {
	code := run([]string{"--help"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

func TestVersionCommand(t *testing.T) {
	out := captureStdout(t, func() {
		code := run([]string{"version"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "pheromone-ctl") {
		t.Errorf("expected pheromone-ctl in output, got: %s", out)
	}
}

func TestLogin(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "login", "--username", "admin", "--password", "admin"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "test-token-abc123") {
		t.Errorf("expected token in output, got: %s", out)
	}
}

func TestLoginMissingArgs(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	code := run([]string{"--server", srv.URL, "login"})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

func TestLogout(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	code := run([]string{"--server", srv.URL, "--token", "tok", "logout"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

func TestWhoami(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--token", "tok", "whoami"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "admin") {
		t.Errorf("expected admin in output, got: %s", out)
	}
}

func TestWhoamiJSON(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--output", "json", "--token", "tok", "whoami"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	var u userRow
	if err := json.Unmarshal([]byte(out), &u); err != nil {
		t.Fatalf("expected valid JSON, got %s: %v", out, err)
	}
	if u.Username != "admin" {
		t.Errorf("expected admin, got %s", u.Username)
	}
}

func TestHealth(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "health"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "ok") {
		t.Errorf("expected ok in output, got: %s", out)
	}
}

func TestStats(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--token", "tok", "stats"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "4") {
		t.Errorf("expected agent count in output, got: %s", out)
	}
}

func TestAgentsList(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--token", "tok", "agents", "list"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "agent-os-01") {
		t.Errorf("expected agent-os-01 in output, got: %s", out)
	}
}

func TestAgentsListJSON(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--output", "json", "--token", "tok", "agents", "list"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	var agents []agentRow
	if err := json.Unmarshal([]byte(out), &agents); err != nil {
		t.Fatalf("expected valid JSON array, got %s: %v", out, err)
	}
	if len(agents) == 0 {
		t.Error("expected at least one agent")
	}
}

func TestAgentsListYAML(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--output", "yaml", "--token", "tok", "agents", "list"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "agent-os-01") {
		t.Errorf("expected agent-os-01 in yaml output, got: %s", out)
	}
}

func TestAgentsGet(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--token", "tok", "agents", "get", "agent-os-01"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "agent-os-01") {
		t.Errorf("expected agent in output, got: %s", out)
	}
}

func TestAgentsGetMissingID(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	code := run([]string{"--server", srv.URL, "--token", "tok", "agents", "get"})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

func TestAgentsMissingSubcmd(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	code := run([]string{"--server", srv.URL, "agents"})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

func TestTwinsList(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--token", "tok", "twins", "list"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "twin-os-01") {
		t.Errorf("expected twin-os-01 in output, got: %s", out)
	}
}

func TestTwinsGet(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--token", "tok", "twins", "get", "twin-os-01"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "twin-os-01") {
		t.Errorf("expected twin-os-01 in output, got: %s", out)
	}
}

func TestTwinsChangesets(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--token", "tok", "twins", "changesets", "twin-os-02"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "cs-001") {
		t.Errorf("expected cs-001 in output, got: %s", out)
	}
}

func TestSkillsList(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--token", "tok", "skills", "list"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "digital-twin") {
		t.Errorf("expected digital-twin in output, got: %s", out)
	}
}

func TestGroupsList(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--token", "tok", "groups", "list"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "grp-001") {
		t.Errorf("expected grp-001 in output, got: %s", out)
	}
}

func TestGroupsGet(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--token", "tok", "groups", "get", "grp-001"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "grp-001") {
		t.Errorf("expected grp-001 in output, got: %s", out)
	}
}

func TestGroupsCreate(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--token", "tok",
			"groups", "create", "--name", "Test Group", "--type", "agents"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "created group") {
		t.Errorf("expected created group in output, got: %s", out)
	}
}

func TestGroupsCreateMissingFlags(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	code := run([]string{"--server", srv.URL, "--token", "tok", "groups", "create", "--name", "x"})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

func TestGroupsUpdate(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--token", "tok",
			"groups", "update", "grp-001", "--name", "Renamed"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "updated group") {
		t.Errorf("expected updated group in output, got: %s", out)
	}
}

func TestGroupsDelete(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--token", "tok", "groups", "delete", "grp-001"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "deleted group") {
		t.Errorf("expected deleted group in output, got: %s", out)
	}
}

func TestChangesetsList(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--token", "tok", "changesets", "list"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "cs-001") {
		t.Errorf("expected cs-001 in output, got: %s", out)
	}
}

func TestChangesetsGet(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--token", "tok", "changesets", "get", "cs-001"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "cs-001") {
		t.Errorf("expected cs-001 in output, got: %s", out)
	}
}

func TestConnectionsList(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--token", "tok", "connections", "list"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "conn-001") {
		t.Errorf("expected conn-001 in output, got: %s", out)
	}
}

func TestUsersList(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--token", "tok", "users", "list"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "admin") {
		t.Errorf("expected admin in output, got: %s", out)
	}
}

func TestUsersGet(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--token", "tok", "users", "get", "admin"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "admin") {
		t.Errorf("expected admin in output, got: %s", out)
	}
}

func TestUsersCreate(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--token", "tok",
			"users", "create", "--username", "alice", "--password", "secret123", "--role", "operator"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "created user") {
		t.Errorf("expected created user in output, got: %s", out)
	}
}

func TestUsersCreateMissingFlags(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	code := run([]string{"--server", srv.URL, "--token", "tok",
		"users", "create", "--username", "alice"})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

func TestUsersUpdate(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--token", "tok",
			"users", "update", "alice", "--role", "observer"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "updated user") {
		t.Errorf("expected updated user in output, got: %s", out)
	}
}

func TestUsersUpdateDisableEnable(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	// Flags must come before the positional username arg with Go's flag package.
	code := run([]string{"--server", srv.URL, "--token", "tok",
		"users", "update", "--disable", "--enable", "alice"})
	if code != 1 {
		t.Fatalf("expected exit code 1 for conflicting flags, got %d", code)
	}
}

func TestUsersDelete(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--token", "tok", "users", "delete", "alice"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "deleted user") {
		t.Errorf("expected deleted user in output, got: %s", out)
	}
}

func TestAPIKeysList(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--token", "tok",
			"users", "api-keys", "list", "admin"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "key-001") {
		t.Errorf("expected key-001 in output, got: %s", out)
	}
}

func TestAPIKeysCreate(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		// --name flag must come before the positional username arg with Go's flag package.
		code := run([]string{"--server", srv.URL, "--token", "tok",
			"users", "api-keys", "create", "--name", "CI key", "admin"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "ak_full_secret_key") {
		t.Errorf("expected key in output, got: %s", out)
	}
}

func TestAPIKeysCreateInvalidExpiry(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	code := run([]string{"--server", srv.URL, "--token", "tok",
		"users", "api-keys", "create", "admin", "--name", "x", "--expires", "not-a-date"})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

func TestAPIKeysDelete(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--token", "tok",
			"users", "api-keys", "delete", "admin", "key-001"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "deleted API key") {
		t.Errorf("expected deleted API key in output, got: %s", out)
	}
}

func TestAudit(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	out := captureStdout(t, func() {
		code := run([]string{"--server", srv.URL, "--token", "tok", "audit"})
		if code != 0 {
			t.Errorf("expected exit code 0, got %d", code)
		}
	})
	if !containsStr(out, "create_user") {
		t.Errorf("expected create_user in output, got: %s", out)
	}
}

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{"a, b , c", []string{"a", "b", "c"}},
		{",", nil},
	}
	for _, tt := range tests {
		got := splitCSV(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitCSV(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitCSV(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestFmtTime(t *testing.T) {
	zero := time.Time{}
	if fmtTime(zero) != "-" {
		t.Errorf("expected - for zero time")
	}

	recent := time.Now().Add(-10 * time.Second)
	out := fmtTime(recent)
	if !containsStr(out, "ago") {
		t.Errorf("expected 'ago' in output, got: %s", out)
	}
}

func TestServerFromEnv(t *testing.T) {
	old := os.Getenv("PHEROMONE_CTL_SERVER")
	defer os.Setenv("PHEROMONE_CTL_SERVER", old)

	os.Setenv("PHEROMONE_CTL_SERVER", "http://custom:9000")
	got := serverFromEnv()
	if got != "http://custom:9000" {
		t.Errorf("expected http://custom:9000, got %s", got)
	}

	os.Unsetenv("PHEROMONE_CTL_SERVER")
	got = serverFromEnv()
	if got != "http://localhost:8081" {
		t.Errorf("expected default, got %s", got)
	}
}

// containsStr returns true if s contains substr.
func containsStr(s, substr string) bool {
	return strings.Contains(s, substr)
}
