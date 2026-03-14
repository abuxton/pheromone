// pheromone-ctl is the Pheromone server CLI client.
//
// Usage:
//
//	pheromone-ctl [global flags] <command> [subcommand] [flags]
//
// Commands:
//
//	login            Authenticate and obtain a session token
//	logout           Discard the current session token
//	whoami           Show the current authenticated user
//	health           Show server health
//	stats            Show platform statistics
//	agents           Manage agents
//	twins            Manage digital twins
//	skills           List available skills
//	groups           Manage agent/twin groups
//	changesets       Inspect twin changesets
//	connections      List agent-twin connections
//	users            Manage users (admin only)
//	audit            View audit log (admin only)
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

const usageText = `pheromone-ctl — Pheromone server CLI client

Usage:
  pheromone-ctl [global flags] <command> [subcommand] [flags]

Commands:
  login            Authenticate and print a session token
  logout           Discard the current session token
  whoami           Show the current authenticated user
  health           Show server health and uptime
  stats            Show platform statistics
  agents list      List registered agents
  agents get       Get details for a specific agent
  twins list       List digital twins
  twins get        Get details for a specific twin
  twins changesets List changesets for a specific twin
  skills list      List available skills
  groups list      List groups
  groups get       Get details for a specific group
  groups create    Create a new group
  groups update    Update an existing group
  groups delete    Delete a group
  changesets list  List all changesets
  changesets get   Get details for a specific changeset
  connections list List agent-twin connections
  users list       List users (admin only)
  users get        Get a user (admin only)
  users create     Create a user (admin only)
  users update     Update a user (admin only)
  users delete     Delete a user (admin only)
  users api-keys list    List API keys for a user (admin only)
  users api-keys create  Create an API key for a user (admin only)
  users api-keys delete  Delete an API key for a user (admin only)
  audit            View the audit log (admin only)

Global Flags:
  --server   <url>    Pheromone server URL
                      (default: http://localhost:8081, env: PHEROMONE_CTL_SERVER)
  --token    <token>  Bearer token for authentication
                      (env: PHEROMONE_CTL_TOKEN)
  --api-key  <key>    API key for authentication (alternative to --token)
                      (env: PHEROMONE_CTL_API_KEY)
  --output   <fmt>    Output format: json, yaml, or table (default: table)
  --insecure          Skip TLS certificate verification

Examples:
  # Authenticate and print the token
  pheromone-ctl login --username admin --password admin

  # List all agents in table format
  pheromone-ctl --token <token> agents list

  # Get a specific twin as JSON
  pheromone-ctl --token <token> --output json twins get twin-os-01

  # Create a new group
  pheromone-ctl --token <token> groups create --name "My Group" --type agents

  # Create a user (admin only)
  pheromone-ctl --token <token> users create --username alice --password secret --role operator

  # View the audit log as YAML
  pheromone-ctl --token <token> --output yaml audit
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usageText)
		return 1
	}

	// Global flags
	globalFlags := flag.NewFlagSet("pheromone-ctl", flag.ContinueOnError)
	server := globalFlags.String("server", serverFromEnv(), "Pheromone server URL")
	token := globalFlags.String("token", os.Getenv("PHEROMONE_CTL_TOKEN"), "bearer token for authentication")
	apiKey := globalFlags.String("api-key", os.Getenv("PHEROMONE_CTL_API_KEY"), "API key for authentication")
	output := globalFlags.String("output", "table", "output format: json, yaml, or table")
	insecure := globalFlags.Bool("insecure", false, "skip TLS certificate verification")

	if err := globalFlags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	remaining := globalFlags.Args()
	if len(remaining) == 0 {
		fmt.Fprint(os.Stderr, usageText)
		return 1
	}

	c := newClient(*server, *token, *apiKey, *insecure)

	switch remaining[0] {
	case "login":
		return runLogin(c, remaining[1:])
	case "logout":
		return runLogout(c, remaining[1:])
	case "whoami":
		return runWhoami(c, remaining[1:], *output)
	case "health":
		return runHealth(c, remaining[1:], *output)
	case "stats":
		return runStats(c, remaining[1:], *output)
	case "agents":
		return runAgents(c, remaining[1:], *output)
	case "twins":
		return runTwins(c, remaining[1:], *output)
	case "skills":
		return runSkills(c, remaining[1:], *output)
	case "groups":
		return runGroups(c, remaining[1:], *output)
	case "changesets":
		return runChangesets(c, remaining[1:], *output)
	case "connections":
		return runConnections(c, remaining[1:], *output)
	case "users":
		return runUsers(c, remaining[1:], *output)
	case "audit":
		return runAudit(c, remaining[1:], *output)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", remaining[0], usageText)
		return 1
	}
}

// serverFromEnv returns the server URL from the environment or the default.
func serverFromEnv() string {
	if v := os.Getenv("PHEROMONE_CTL_SERVER"); v != "" {
		return v
	}
	return "http://localhost:8081"
}

// -------------------------------------------------------------------------
// login / logout / whoami
// -------------------------------------------------------------------------

func runLogin(c *client, args []string) int {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	username := fs.String("username", "", "username")
	password := fs.String("password", "", "password")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if *username == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "error: --username and --password are required")
		return 1
	}

	var resp struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
		User      userRow   `json:"user"`
	}
	if err := c.post("/api/v1/auth/login", map[string]string{
		"username": *username,
		"password": *password,
	}, &resp); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Printf("token:      %s\n", resp.Token)
	fmt.Printf("expires_at: %s\n", resp.ExpiresAt.Format(time.RFC3339))
	fmt.Printf("user:       %s (%s)\n", resp.User.Username, resp.User.Role)
	fmt.Printf("\nTip: export PHEROMONE_CTL_TOKEN=%s\n", resp.Token)
	return 0
}

func runLogout(c *client, args []string) int {
	fs := flag.NewFlagSet("logout", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if err := c.delete("/api/v1/auth/logout", nil); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Println("logged out")
	return 0
}

func runWhoami(c *client, args []string, output string) int {
	fs := flag.NewFlagSet("whoami", flag.ContinueOnError)
	outputFlag := fs.String("output", output, "output format: json, yaml, or table")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	var u userRow
	if err := c.get("/api/v1/auth/whoami", &u); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	printOutput(u, *outputFlag, func(v interface{}) {
		u := v.(userRow)
		fmt.Printf("Username:     %s\n", u.Username)
		fmt.Printf("Role:         %s\n", u.Role)
		if u.DisplayName != "" {
			fmt.Printf("Display Name: %s\n", u.DisplayName)
		}
		if u.Email != "" {
			fmt.Printf("Email:        %s\n", u.Email)
		}
	})
	return 0
}

// -------------------------------------------------------------------------
// health / stats
// -------------------------------------------------------------------------

func runHealth(c *client, args []string, output string) int {
	fs := flag.NewFlagSet("health", flag.ContinueOnError)
	outputFlag := fs.String("output", output, "output format: json, yaml, or table")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	var h struct {
		Status    string    `json:"status" yaml:"status"`
		Version   string    `json:"version" yaml:"version"`
		Uptime    string    `json:"uptime" yaml:"uptime"`
		Timestamp time.Time `json:"timestamp" yaml:"timestamp"`
	}
	if err := c.get("/api/v1/health", &h); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	printOutput(h, *outputFlag, func(v interface{}) {
		fmt.Printf("Status:    %s\n", h.Status)
		fmt.Printf("Version:   %s\n", h.Version)
		fmt.Printf("Uptime:    %s\n", h.Uptime)
		fmt.Printf("Timestamp: %s\n", h.Timestamp.Format(time.RFC3339))
	})
	return 0
}

func runStats(c *client, args []string, output string) int {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	outputFlag := fs.String("output", output, "output format: json, yaml, or table")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	var s struct {
		AgentCount      int `json:"agent_count" yaml:"agent_count"`
		TwinCount       int `json:"twin_count" yaml:"twin_count"`
		OnlineAgents    int `json:"online_agents" yaml:"online_agents"`
		ActiveTwins     int `json:"active_twins" yaml:"active_twins"`
		SkillCount      int `json:"skill_count" yaml:"skill_count"`
		GroupCount      int `json:"group_count" yaml:"group_count"`
		PendingChanges  int `json:"pending_changes" yaml:"pending_changes"`
		ConnectionCount int `json:"connection_count" yaml:"connection_count"`
	}
	if err := c.get("/api/v1/stats", &s); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	printOutput(s, *outputFlag, func(v interface{}) {
		fmt.Printf("Agents:          %d (%d online)\n", s.AgentCount, s.OnlineAgents)
		fmt.Printf("Twins:           %d (%d active)\n", s.TwinCount, s.ActiveTwins)
		fmt.Printf("Skills:          %d\n", s.SkillCount)
		fmt.Printf("Groups:          %d\n", s.GroupCount)
		fmt.Printf("Connections:     %d\n", s.ConnectionCount)
		fmt.Printf("Pending Changes: %d\n", s.PendingChanges)
	})
	return 0
}

// -------------------------------------------------------------------------
// agents
// -------------------------------------------------------------------------

func runAgents(c *client, args []string, output string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "agents requires a subcommand: list, get")
		return 1
	}
	switch args[0] {
	case "list":
		return runAgentsList(c, args[1:], output)
	case "get":
		return runAgentsGet(c, args[1:], output)
	default:
		fmt.Fprintf(os.Stderr, "unknown agents subcommand %q\n", args[0])
		return 1
	}
}

func runAgentsList(c *client, args []string, output string) int {
	fs := flag.NewFlagSet("agents list", flag.ContinueOnError)
	outputFlag := fs.String("output", output, "output format: json, yaml, or table")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	var agents []agentRow
	if err := c.get("/api/v1/agents", &agents); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	printOutput(agents, *outputFlag, func(v interface{}) {
		printAgentsTable(v)
	})
	return 0
}

func runAgentsGet(c *client, args []string, output string) int {
	fs := flag.NewFlagSet("agents get", flag.ContinueOnError)
	outputFlag := fs.String("output", output, "output format: json, yaml, or table")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: pheromone-ctl agents get <id>")
		return 1
	}
	id := fs.Arg(0)

	var agent json.RawMessage
	if err := c.get("/api/v1/agents/"+id, &agent); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	var a agentRow
	_ = json.Unmarshal(agent, &a)

	printOutput(agent, *outputFlag, func(v interface{}) {
		printAgentsTable([]agentRow{a})
	})
	return 0
}

// -------------------------------------------------------------------------
// twins
// -------------------------------------------------------------------------

func runTwins(c *client, args []string, output string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "twins requires a subcommand: list, get, changesets")
		return 1
	}
	switch args[0] {
	case "list":
		return runTwinsList(c, args[1:], output)
	case "get":
		return runTwinsGet(c, args[1:], output)
	case "changesets":
		return runTwinsChangesets(c, args[1:], output)
	default:
		fmt.Fprintf(os.Stderr, "unknown twins subcommand %q\n", args[0])
		return 1
	}
}

func runTwinsList(c *client, args []string, output string) int {
	fs := flag.NewFlagSet("twins list", flag.ContinueOnError)
	outputFlag := fs.String("output", output, "output format: json, yaml, or table")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	var twins []twinRow
	if err := c.get("/api/v1/twins", &twins); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	printOutput(twins, *outputFlag, func(v interface{}) {
		printTwinsTable(v)
	})
	return 0
}

func runTwinsGet(c *client, args []string, output string) int {
	fs := flag.NewFlagSet("twins get", flag.ContinueOnError)
	outputFlag := fs.String("output", output, "output format: json, yaml, or table")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: pheromone-ctl twins get <id>")
		return 1
	}
	id := fs.Arg(0)

	var twin json.RawMessage
	if err := c.get("/api/v1/twins/"+id, &twin); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	var t twinRow
	_ = json.Unmarshal(twin, &t)

	printOutput(twin, *outputFlag, func(v interface{}) {
		printTwinsTable([]twinRow{t})
	})
	return 0
}

func runTwinsChangesets(c *client, args []string, output string) int {
	fs := flag.NewFlagSet("twins changesets", flag.ContinueOnError)
	outputFlag := fs.String("output", output, "output format: json, yaml, or table")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: pheromone-ctl twins changesets <twin-id>")
		return 1
	}
	id := fs.Arg(0)

	var cs []changesetRow
	if err := c.get("/api/v1/twins/"+id+"/changesets", &cs); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	printOutput(cs, *outputFlag, func(v interface{}) {
		printChangesetsTable(v)
	})
	return 0
}

// -------------------------------------------------------------------------
// skills
// -------------------------------------------------------------------------

func runSkills(c *client, args []string, output string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "skills requires a subcommand: list")
		return 1
	}
	switch args[0] {
	case "list":
		return runSkillsList(c, args[1:], output)
	default:
		fmt.Fprintf(os.Stderr, "unknown skills subcommand %q\n", args[0])
		return 1
	}
}

func runSkillsList(c *client, args []string, output string) int {
	fs := flag.NewFlagSet("skills list", flag.ContinueOnError)
	outputFlag := fs.String("output", output, "output format: json, yaml, or table")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	var skills []skillRow
	if err := c.get("/api/v1/skills", &skills); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	printOutput(skills, *outputFlag, func(v interface{}) {
		printSkillsTable(v)
	})
	return 0
}

// -------------------------------------------------------------------------
// groups
// -------------------------------------------------------------------------

func runGroups(c *client, args []string, output string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "groups requires a subcommand: list, get, create, update, delete")
		return 1
	}
	switch args[0] {
	case "list":
		return runGroupsList(c, args[1:], output)
	case "get":
		return runGroupsGet(c, args[1:], output)
	case "create":
		return runGroupsCreate(c, args[1:])
	case "update":
		return runGroupsUpdate(c, args[1:])
	case "delete":
		return runGroupsDelete(c, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown groups subcommand %q\n", args[0])
		return 1
	}
}

func runGroupsList(c *client, args []string, output string) int {
	fs := flag.NewFlagSet("groups list", flag.ContinueOnError)
	outputFlag := fs.String("output", output, "output format: json, yaml, or table")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	var groups []groupRow
	if err := c.get("/api/v1/groups", &groups); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	printOutput(groups, *outputFlag, func(v interface{}) {
		printGroupsTable(v)
	})
	return 0
}

func runGroupsGet(c *client, args []string, output string) int {
	fs := flag.NewFlagSet("groups get", flag.ContinueOnError)
	outputFlag := fs.String("output", output, "output format: json, yaml, or table")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: pheromone-ctl groups get <id>")
		return 1
	}
	id := fs.Arg(0)

	var g groupRow
	if err := c.get("/api/v1/groups/"+id, &g); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	printOutput(g, *outputFlag, func(v interface{}) {
		printGroupsTable([]groupRow{g})
	})
	return 0
}

func runGroupsCreate(c *client, args []string) int {
	fs := flag.NewFlagSet("groups create", flag.ContinueOnError)
	name := fs.String("name", "", "group name (required)")
	groupType := fs.String("type", "", "group type: agents, twins, infrastructure (required)")
	description := fs.String("description", "", "group description")
	members := fs.String("members", "", "comma-separated list of member IDs")
	namespace := fs.String("twin-namespace", "", "twin namespace for the group")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if *name == "" || *groupType == "" {
		fmt.Fprintln(os.Stderr, "error: --name and --type are required")
		return 1
	}

	req := map[string]interface{}{
		"name":        *name,
		"type":        *groupType,
		"description": *description,
		"members":     splitCSV(*members),
	}
	if *namespace != "" {
		req["twin_namespace"] = *namespace
	}

	var g groupRow
	if err := c.post("/api/v1/groups", req, &g); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Printf("created group %s (%s)\n", g.ID, g.Name)
	return 0
}

func runGroupsUpdate(c *client, args []string) int {
	fs := flag.NewFlagSet("groups update", flag.ContinueOnError)
	name := fs.String("name", "", "new name")
	description := fs.String("description", "", "new description")
	members := fs.String("members", "", "comma-separated list of member IDs")
	namespace := fs.String("twin-namespace", "", "twin namespace")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: pheromone-ctl groups update <id> [flags]")
		return 1
	}
	id := fs.Arg(0)

	req := map[string]interface{}{}
	if *name != "" {
		req["name"] = *name
	}
	if *description != "" {
		req["description"] = *description
	}
	if *members != "" {
		req["members"] = splitCSV(*members)
	}
	if *namespace != "" {
		req["twin_namespace"] = *namespace
	}

	var g groupRow
	if err := c.put("/api/v1/groups/"+id, req, &g); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Printf("updated group %s (%s)\n", g.ID, g.Name)
	return 0
}

func runGroupsDelete(c *client, args []string) int {
	fs := flag.NewFlagSet("groups delete", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: pheromone-ctl groups delete <id>")
		return 1
	}
	id := fs.Arg(0)

	if err := c.delete("/api/v1/groups/"+id, nil); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Printf("deleted group %s\n", id)
	return 0
}

// -------------------------------------------------------------------------
// changesets
// -------------------------------------------------------------------------

func runChangesets(c *client, args []string, output string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "changesets requires a subcommand: list, get")
		return 1
	}
	switch args[0] {
	case "list":
		return runChangesetsList(c, args[1:], output)
	case "get":
		return runChangesetsGet(c, args[1:], output)
	default:
		fmt.Fprintf(os.Stderr, "unknown changesets subcommand %q\n", args[0])
		return 1
	}
}

func runChangesetsList(c *client, args []string, output string) int {
	fs := flag.NewFlagSet("changesets list", flag.ContinueOnError)
	outputFlag := fs.String("output", output, "output format: json, yaml, or table")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	var cs []changesetRow
	if err := c.get("/api/v1/changesets", &cs); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	printOutput(cs, *outputFlag, func(v interface{}) {
		printChangesetsTable(v)
	})
	return 0
}

func runChangesetsGet(c *client, args []string, output string) int {
	fs := flag.NewFlagSet("changesets get", flag.ContinueOnError)
	outputFlag := fs.String("output", output, "output format: json, yaml, or table")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: pheromone-ctl changesets get <id>")
		return 1
	}
	id := fs.Arg(0)

	var cs changesetRow
	if err := c.get("/api/v1/changesets/"+id, &cs); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	printOutput(cs, *outputFlag, func(v interface{}) {
		printChangesetsTable([]changesetRow{cs})
	})
	return 0
}

// -------------------------------------------------------------------------
// connections
// -------------------------------------------------------------------------

func runConnections(c *client, args []string, output string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "connections requires a subcommand: list")
		return 1
	}
	switch args[0] {
	case "list":
		return runConnectionsList(c, args[1:], output)
	default:
		fmt.Fprintf(os.Stderr, "unknown connections subcommand %q\n", args[0])
		return 1
	}
}

func runConnectionsList(c *client, args []string, output string) int {
	fs := flag.NewFlagSet("connections list", flag.ContinueOnError)
	outputFlag := fs.String("output", output, "output format: json, yaml, or table")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	var conns []connectionRow
	if err := c.get("/api/v1/connections", &conns); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	printOutput(conns, *outputFlag, func(v interface{}) {
		printConnectionsTable(v)
	})
	return 0
}

// -------------------------------------------------------------------------
// users
// -------------------------------------------------------------------------

func runUsers(c *client, args []string, output string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "users requires a subcommand: list, get, create, update, delete, api-keys")
		return 1
	}
	switch args[0] {
	case "list":
		return runUsersList(c, args[1:], output)
	case "get":
		return runUsersGet(c, args[1:], output)
	case "create":
		return runUsersCreate(c, args[1:])
	case "update":
		return runUsersUpdate(c, args[1:])
	case "delete":
		return runUsersDelete(c, args[1:])
	case "api-keys":
		return runUsersAPIKeys(c, args[1:], output)
	default:
		fmt.Fprintf(os.Stderr, "unknown users subcommand %q\n", args[0])
		return 1
	}
}

func runUsersList(c *client, args []string, output string) int {
	fs := flag.NewFlagSet("users list", flag.ContinueOnError)
	outputFlag := fs.String("output", output, "output format: json, yaml, or table")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	var users []userRow
	if err := c.get("/api/v1/users", &users); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	printOutput(users, *outputFlag, func(v interface{}) {
		printUsersTable(v)
	})
	return 0
}

func runUsersGet(c *client, args []string, output string) int {
	fs := flag.NewFlagSet("users get", flag.ContinueOnError)
	outputFlag := fs.String("output", output, "output format: json, yaml, or table")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: pheromone-ctl users get <username>")
		return 1
	}
	username := fs.Arg(0)

	var u userRow
	if err := c.get("/api/v1/users/"+username, &u); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	printOutput(u, *outputFlag, func(v interface{}) {
		printUsersTable([]userRow{u})
	})
	return 0
}

func runUsersCreate(c *client, args []string) int {
	fs := flag.NewFlagSet("users create", flag.ContinueOnError)
	username := fs.String("username", "", "username (required)")
	password := fs.String("password", "", "password (required)")
	role := fs.String("role", "", "role: admin, operator, observer (required)")
	displayName := fs.String("display-name", "", "display name")
	email := fs.String("email", "", "email address")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if *username == "" || *password == "" || *role == "" {
		fmt.Fprintln(os.Stderr, "error: --username, --password, and --role are required")
		return 1
	}

	req := map[string]interface{}{
		"username": *username,
		"password": *password,
		"role":     *role,
	}
	if *displayName != "" {
		req["display_name"] = *displayName
	}
	if *email != "" {
		req["email"] = *email
	}

	var u userRow
	if err := c.post("/api/v1/users", req, &u); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Printf("created user %s (role: %s)\n", u.Username, u.Role)
	return 0
}

func runUsersUpdate(c *client, args []string) int {
	fs := flag.NewFlagSet("users update", flag.ContinueOnError)
	role := fs.String("role", "", "new role")
	displayName := fs.String("display-name", "", "new display name")
	email := fs.String("email", "", "new email address")
	password := fs.String("password", "", "new password")
	disable := fs.Bool("disable", false, "disable the user account")
	enable := fs.Bool("enable", false, "enable the user account")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: pheromone-ctl users update <username> [flags]")
		return 1
	}
	if *disable && *enable {
		fmt.Fprintln(os.Stderr, "error: --disable and --enable are mutually exclusive")
		return 1
	}
	username := fs.Arg(0)

	req := map[string]interface{}{}
	if *role != "" {
		req["role"] = *role
	}
	if *displayName != "" {
		req["display_name"] = *displayName
	}
	if *email != "" {
		req["email"] = *email
	}
	if *password != "" {
		req["password"] = *password
	}
	if *disable {
		req["disabled"] = true
	}
	if *enable {
		req["disabled"] = false
	}

	var u userRow
	if err := c.put("/api/v1/users/"+username, req, &u); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Printf("updated user %s\n", u.Username)
	return 0
}

func runUsersDelete(c *client, args []string) int {
	fs := flag.NewFlagSet("users delete", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: pheromone-ctl users delete <username>")
		return 1
	}
	username := fs.Arg(0)

	if err := c.delete("/api/v1/users/"+username, nil); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Printf("deleted user %s\n", username)
	return 0
}

// -------------------------------------------------------------------------
// users api-keys
// -------------------------------------------------------------------------

func runUsersAPIKeys(c *client, args []string, output string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "users api-keys requires a subcommand: list, create, delete")
		return 1
	}
	switch args[0] {
	case "list":
		return runAPIKeysList(c, args[1:], output)
	case "create":
		return runAPIKeysCreate(c, args[1:])
	case "delete":
		return runAPIKeysDelete(c, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown api-keys subcommand %q\n", args[0])
		return 1
	}
}

func runAPIKeysList(c *client, args []string, output string) int {
	fs := flag.NewFlagSet("users api-keys list", flag.ContinueOnError)
	outputFlag := fs.String("output", output, "output format: json, yaml, or table")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: pheromone-ctl users api-keys list <username>")
		return 1
	}
	username := fs.Arg(0)

	var keys []apiKeyRow
	if err := c.get("/api/v1/users/"+username+"/api-keys", &keys); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	printOutput(keys, *outputFlag, func(v interface{}) {
		printAPIKeysTable(v)
	})
	return 0
}

func runAPIKeysCreate(c *client, args []string) int {
	fs := flag.NewFlagSet("users api-keys create", flag.ContinueOnError)
	name := fs.String("name", "", "key name (required)")
	expiresAt := fs.String("expires", "", "expiry time in RFC3339 format (e.g. 2026-12-31T00:00:00Z)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: pheromone-ctl users api-keys create <username> --name <name>")
		return 1
	}
	if *name == "" {
		fmt.Fprintln(os.Stderr, "error: --name is required")
		return 1
	}
	username := fs.Arg(0)

	req := map[string]interface{}{
		"name": *name,
	}
	if *expiresAt != "" {
		t, err := time.Parse(time.RFC3339, *expiresAt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: invalid --expires value %q: %v\n", *expiresAt, err)
			return 1
		}
		req["expires_at"] = t
	}

	var resp struct {
		Key    string    `json:"key"`
		APIKey apiKeyRow `json:"api_key"`
	}
	if err := c.post("/api/v1/users/"+username+"/api-keys", req, &resp); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Printf("created API key %s for user %s\n", resp.APIKey.ID, username)
	fmt.Printf("key: %s\n", resp.Key)
	fmt.Println("\nStore this key now — it will not be shown again.")
	return 0
}

func runAPIKeysDelete(c *client, args []string) int {
	fs := flag.NewFlagSet("users api-keys delete", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() < 2 {
		fmt.Fprintln(os.Stderr, "usage: pheromone-ctl users api-keys delete <username> <key-id>")
		return 1
	}
	username := fs.Arg(0)
	keyID := fs.Arg(1)

	if err := c.delete("/api/v1/users/"+username+"/api-keys/"+keyID, nil); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Printf("deleted API key %s for user %s\n", keyID, username)
	return 0
}

// -------------------------------------------------------------------------
// audit
// -------------------------------------------------------------------------

func runAudit(c *client, args []string, output string) int {
	fs := flag.NewFlagSet("audit", flag.ContinueOnError)
	outputFlag := fs.String("output", output, "output format: json, yaml, or table")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	var entries []auditRow
	if err := c.get("/api/v1/audit", &entries); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	printOutput(entries, *outputFlag, func(v interface{}) {
		printAuditTable(v)
	})
	return 0
}

// -------------------------------------------------------------------------
// helpers
// -------------------------------------------------------------------------

// splitCSV splits a comma-separated string into a slice of trimmed values,
// returning nil when the input is empty.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
