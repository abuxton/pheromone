package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"gopkg.in/yaml.v3"
)

// client wraps an HTTP client with server URL and auth credentials.
type client struct {
	serverURL  string
	token      string
	apiKey     string
	httpClient *http.Client
}

// newClient creates a new API client.
func newClient(serverURL, token, apiKey string, insecure bool) *client {
	transport := http.DefaultTransport
	if insecure {
		fmt.Fprintln(os.Stderr, "warning: TLS certificate verification is disabled (--insecure)")

		// Clone the default transport to preserve proxy, dialer, keep-alive, and other defaults,
		// and only override TLS verification behavior.
		if base, ok := http.DefaultTransport.(*http.Transport); ok {
			cloned := base.Clone()
			if cloned.TLSClientConfig == nil {
				cloned.TLSClientConfig = &tls.Config{}
			}
			cloned.TLSClientConfig.InsecureSkipVerify = true //nolint:gosec // operator-controlled flag
			transport = cloned
		} else {
			// Fallback (highly unlikely): retain previous behavior with a minimal transport.
			transport = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // operator-controlled flag
			}
		}
	}
	return &client{
		serverURL: strings.TrimRight(serverURL, "/"),
		token:     token,
		apiKey:    apiKey,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

// do performs an HTTP request and decodes the JSON response into out.
// If out is nil, the response body is discarded.
func (c *client) do(method, path string, body interface{}, out interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.serverURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	switch {
	case c.token != "":
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		if jsonErr := json.Unmarshal(respBody, &errResp); jsonErr == nil && errResp.Error != "" {
			msg := errResp.Error
			if errResp.Message != "" {
				msg += ": " + errResp.Message
			}
			return fmt.Errorf("server error %d: %s", resp.StatusCode, msg)
		}
		return fmt.Errorf("server error %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// get performs a GET request.
func (c *client) get(path string, out interface{}) error {
	return c.do(http.MethodGet, path, nil, out)
}

// post performs a POST request.
func (c *client) post(path string, body, out interface{}) error {
	return c.do(http.MethodPost, path, body, out)
}

// put performs a PUT request.
func (c *client) put(path string, body, out interface{}) error {
	return c.do(http.MethodPut, path, body, out)
}

// delete performs a DELETE request.
func (c *client) delete(path string, out interface{}) error {
	return c.do(http.MethodDelete, path, nil, out)
}

// printOutput prints v to stdout in the requested format (json, yaml, or table).
// When format is "table", tableFn is called to render a human-friendly table.
// If tableFn is nil, table falls back to json.
func printOutput(v interface{}, format string, tableFn func(interface{})) {
	switch format {
	case "yaml":
		enc := yaml.NewEncoder(os.Stdout)
		enc.SetIndent(2)
		if err := enc.Encode(v); err != nil {
			fmt.Fprintf(os.Stderr, "error: yaml encode: %v\n", err)
		}
	case "table":
		if tableFn != nil {
			tableFn(v)
			return
		}
		// Fallback to JSON when no table renderer provided.
		fallthrough
	default: // json
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(v); err != nil {
			fmt.Fprintf(os.Stderr, "error: json encode: %v\n", err)
		}
	}
}

// newTabWriter returns a *tabwriter.Writer suitable for table output.
func newTabWriter() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
}

// fmtTime formats a time.Time as a short human-readable string.
func fmtTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	ago := time.Since(t).Round(time.Second)
	if ago < time.Minute {
		return fmt.Sprintf("%ds ago", int(ago.Seconds()))
	}
	if ago < time.Hour {
		return fmt.Sprintf("%dm ago", int(ago.Minutes()))
	}
	if ago < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(ago.Hours()))
	}
	return t.Format("2006-01-02")
}

// printAgentsTable renders a slice of agents as a table.
func printAgentsTable(v interface{}) {
	agents, _ := v.([]agentRow)
	tw := newTabWriter()
	fmt.Fprintln(tw, "ID\tNAME\tTYPE\tSTATUS\tHOSTNAME\tLAST SEEN")
	for _, a := range agents {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			a.ID, a.Name, a.Type, a.Status, a.Hostname, fmtTime(a.LastSeen))
	}
	tw.Flush()
}

// printTwinsTable renders a slice of twins as a table.
func printTwinsTable(v interface{}) {
	twins, _ := v.([]twinRow)
	tw := newTabWriter()
	fmt.Fprintln(tw, "ID\tNAME\tTYPE\tSTATE\tAGENT\tVERSION\tLAST HEARTBEAT")
	for _, t := range twins {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\t%s\n",
			t.ID, t.Name, t.Type, t.State, t.AgentID, t.ConfigVersion, fmtTime(t.LastHeartbeat))
	}
	tw.Flush()
}

// printSkillsTable renders a slice of skills as a table.
func printSkillsTable(v interface{}) {
	skills, _ := v.([]skillRow)
	tw := newTabWriter()
	fmt.Fprintln(tw, "NAME\tVERSION\tMIN TWIN LEVEL\tAGENT COUNT\tDESCRIPTION")
	for _, s := range skills {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\n",
			s.Name, s.Version, s.MinTwinLevel, s.AgentCount, s.Description)
	}
	tw.Flush()
}

// printGroupsTable renders a slice of groups as a table.
func printGroupsTable(v interface{}) {
	groups, _ := v.([]groupRow)
	tw := newTabWriter()
	fmt.Fprintln(tw, "ID\tNAME\tTYPE\tMEMBERS\tNAMESPACE\tDESCRIPTION")
	for _, g := range groups {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\t%s\n",
			g.ID, g.Name, g.Type, len(g.Members), g.TwinNamespace, g.Description)
	}
	tw.Flush()
}

// printChangesetsTable renders a slice of changesets as a table.
func printChangesetsTable(v interface{}) {
	cs, _ := v.([]changesetRow)
	tw := newTabWriter()
	fmt.Fprintln(tw, "ID\tTWIN\tAGENT\tTYPE\tSTATUS\tCHANGES\tCREATED")
	for _, c := range cs {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\t%s\n",
			c.ID, c.TwinID, c.AgentID, c.Type, c.Status, len(c.Changes), fmtTime(c.CreatedAt))
	}
	tw.Flush()
}

// printConnectionsTable renders a slice of connections as a table.
func printConnectionsTable(v interface{}) {
	conns, _ := v.([]connectionRow)
	tw := newTabWriter()
	fmt.Fprintln(tw, "ID\tAGENT\tTWIN\tSTATUS\tSINCE\tLAST SYNC")
	for _, c := range conns {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			c.ID, c.AgentID, c.TwinID, c.Status, fmtTime(c.Since), fmtTime(c.LastSync))
	}
	tw.Flush()
}

// printUsersTable renders a slice of users as a table.
func printUsersTable(v interface{}) {
	users, _ := v.([]userRow)
	tw := newTabWriter()
	fmt.Fprintln(tw, "USERNAME\tROLE\tDISPLAY NAME\tEMAIL\tDISABLED")
	for _, u := range users {
		disabled := "no"
		if u.Disabled {
			disabled = "yes"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			u.Username, u.Role, u.DisplayName, u.Email, disabled)
	}
	tw.Flush()
}

// printAPIKeysTable renders a slice of API keys as a table.
func printAPIKeysTable(v interface{}) {
	keys, _ := v.([]apiKeyRow)
	tw := newTabWriter()
	fmt.Fprintln(tw, "ID\tNAME\tPREFIX\tCREATED\tLAST USED\tEXPIRES")
	for _, k := range keys {
		lastUsed := "-"
		if k.LastUsedAt != nil {
			lastUsed = fmtTime(*k.LastUsedAt)
		}
		expires := "-"
		if k.ExpiresAt != nil {
			expires = k.ExpiresAt.Format("2006-01-02")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			k.ID, k.Name, k.KeyPrefix, fmtTime(k.CreatedAt), lastUsed, expires)
	}
	tw.Flush()
}

// printAuditTable renders a slice of audit entries as a table.
func printAuditTable(v interface{}) {
	entries, _ := v.([]auditRow)
	tw := newTabWriter()
	fmt.Fprintln(tw, "TIME\tUSER\tROLE\tOPERATION\tRESOURCE\tRESULT")
	for _, e := range entries {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			e.Timestamp.Format("15:04:05"), e.UserID, e.Role,
			e.Operation, e.ResourceID, e.Result)
	}
	tw.Flush()
}

// Row types for table rendering — they mirror the JSON types but use only the
// fields we display in the table. Decoding into these avoids importing internal/api.

type agentRow struct {
	ID       string    `json:"id" yaml:"id"`
	Name     string    `json:"name" yaml:"name"`
	Type     string    `json:"type" yaml:"type"`
	Status   string    `json:"status" yaml:"status"`
	Hostname string    `json:"hostname" yaml:"hostname"`
	LastSeen time.Time `json:"last_seen" yaml:"last_seen"`
}

type twinRow struct {
	ID            string    `json:"id" yaml:"id"`
	Name          string    `json:"name" yaml:"name"`
	Type          string    `json:"type" yaml:"type"`
	State         string    `json:"state" yaml:"state"`
	AgentID       string    `json:"agent_id" yaml:"agent_id"`
	ConfigVersion int       `json:"config_version" yaml:"config_version"`
	LastHeartbeat time.Time `json:"last_heartbeat" yaml:"last_heartbeat"`
}

type skillRow struct {
	Name         string `json:"name" yaml:"name"`
	Version      string `json:"version" yaml:"version"`
	MinTwinLevel string `json:"min_twin_level" yaml:"min_twin_level"`
	Description  string `json:"description" yaml:"description"`
	AgentCount   int    `json:"agent_count" yaml:"agent_count"`
}

type groupRow struct {
	ID            string    `json:"id" yaml:"id"`
	Name          string    `json:"name" yaml:"name"`
	Type          string    `json:"type" yaml:"type"`
	Description   string    `json:"description" yaml:"description"`
	Members       []string  `json:"members" yaml:"members"`
	TwinNamespace string    `json:"twin_namespace" yaml:"twin_namespace"`
	CreatedAt     time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" yaml:"updated_at"`
}

type changesetRow struct {
	ID        string        `json:"id" yaml:"id"`
	TwinID    string        `json:"twin_id" yaml:"twin_id"`
	AgentID   string        `json:"agent_id" yaml:"agent_id"`
	Type      string        `json:"type" yaml:"type"`
	Status    string        `json:"status" yaml:"status"`
	Changes   []fieldChange `json:"changes" yaml:"changes"`
	CreatedAt time.Time     `json:"created_at" yaml:"created_at"`
}

type fieldChange struct {
	Field    string `json:"field" yaml:"field"`
	OldValue string `json:"old_value" yaml:"old_value"`
	NewValue string `json:"new_value" yaml:"new_value"`
}

type connectionRow struct {
	ID       string    `json:"id" yaml:"id"`
	AgentID  string    `json:"agent_id" yaml:"agent_id"`
	TwinID   string    `json:"twin_id" yaml:"twin_id"`
	Status   string    `json:"status" yaml:"status"`
	Since    time.Time `json:"since" yaml:"since"`
	LastSync time.Time `json:"last_sync" yaml:"last_sync"`
}

type userRow struct {
	Username    string `json:"username" yaml:"username"`
	Role        string `json:"role" yaml:"role"`
	DisplayName string `json:"display_name" yaml:"display_name"`
	Email       string `json:"email" yaml:"email"`
	Disabled    bool   `json:"disabled" yaml:"disabled"`
}

type apiKeyRow struct {
	ID         string     `json:"id" yaml:"id"`
	Name       string     `json:"name" yaml:"name"`
	KeyPrefix  string     `json:"key_prefix" yaml:"key_prefix"`
	CreatedAt  time.Time  `json:"created_at" yaml:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at" yaml:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at" yaml:"expires_at,omitempty"`
}

type auditRow struct {
	ID         string    `json:"id" yaml:"id"`
	Timestamp  time.Time `json:"timestamp" yaml:"timestamp"`
	UserID     string    `json:"user_id" yaml:"user_id"`
	Role       string    `json:"role" yaml:"role"`
	Operation  string    `json:"operation" yaml:"operation"`
	ResourceID string    `json:"resource_id" yaml:"resource_id"`
	RemoteAddr string    `json:"remote_addr" yaml:"remote_addr"`
	Result     string    `json:"result" yaml:"result"`
	Message    string    `json:"message" yaml:"message"`
}
