// pheromone-server is the Pheromone server process.
//
// Usage:
//
//	pheromone-server [--config-path <dir>] [--format json|yaml]
//	pheromone-server config validate [--config-path <dir>]
//	pheromone-server config generate [--config-path <dir>] [--format json|yaml] [--component server|twin|listener|all]
//	pheromone-server serve [--config-path <dir>] [--ui-port <port>]
//	pheromone-server ui hash-password <password>
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"flag"

	"github.com/abuxton/pheromone/internal/api"
	"github.com/abuxton/pheromone/internal/config"
)

const usageText = `pheromone-server — Pheromone server process

Usage:
  pheromone-server [global flags] <command> [command flags]

Commands:
  config validate   Validate all configuration files in the config directory
  config generate   Generate a default configuration file
  serve             Start the management UI and REST API HTTP server
  ui hash-password  Generate a password hash suitable for ui.users[*].password_hash

Global Flags:
  --config-path <dir>   Directory containing server configuration files
                        (default: /etc/pheromone/server, env: PHEROMONE_SERVER_CONFIG_PATH)

Config Generate Flags:
  --format  <fmt>       Output format: json or yaml (default: yaml)
  --component <name>    Component to generate config for:
                        server, twin, listener, all (default: all)
  --output  <path>      Output file path (default: <config-path>/<component>.<format>)

Serve Flags:
  --ui-addr <addr>      Override the UI bind address (default: from config or 0.0.0.0)
  --ui-port <port>      Override the UI HTTP port (default: from config or 8081)

Examples:
  # Start the management UI on default port 8081
  pheromone-server serve

  # Start with a custom UI port
  pheromone-server serve --ui-port 9090

  # Validate the current server configuration
  pheromone-server config validate

  # Generate a default server config in YAML
  pheromone-server config generate --format yaml --component server

  # Generate a password hash for a new user
  pheromone-server ui hash-password mysecretpassword
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
	globalFlags := flag.NewFlagSet("pheromone-server", flag.ContinueOnError)
	configPath := globalFlags.String("config-path", configPathFromEnv(), "directory containing server configuration files")

	// Parse up to the first non-flag argument (the subcommand).
	if err := globalFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}

	remaining := globalFlags.Args()
	if len(remaining) == 0 {
		fmt.Fprint(os.Stderr, usageText)
		return 1
	}

	switch remaining[0] {
	case "config":
		return runConfig(remaining[1:], *configPath)
	case "serve":
		return runServe(remaining[1:], *configPath)
	case "ui":
		return runUI(remaining[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", remaining[0], usageText)
		return 1
	}
}

func runConfig(args []string, globalConfigPath string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "config requires a subcommand: validate or generate\n")
		return 1
	}

	switch args[0] {
	case "validate":
		return runConfigValidate(args[1:], globalConfigPath)
	case "generate":
		return runConfigGenerate(args[1:], globalConfigPath)
	default:
		fmt.Fprintf(os.Stderr, "unknown config subcommand %q\n", args[0])
		return 1
	}
}

func runConfigValidate(args []string, globalConfigPath string) int {
	fs := flag.NewFlagSet("config validate", flag.ContinueOnError)
	configPath := fs.String("config-path", globalConfigPath, "directory containing server configuration files")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	log.Info("validating server configuration", "path", *configPath)

	cfg, err := config.LoadServerConfig(*configPath)
	if err != nil {
		log.Error("failed to load configuration", "error", err)
		return 1
	}

	if err := config.ValidateServerConfig(cfg); err != nil {
		log.Error("configuration is invalid", "error", err)
		return 1
	}

	log.Info("configuration is valid",
		"path", *configPath,
		"server_address", fmt.Sprintf("%s:%d", cfg.Server.Address, cfg.Server.Port),
		"twins", len(cfg.Twins),
		"listeners", len(cfg.Listeners),
	)
	return 0
}

func runConfigGenerate(args []string, globalConfigPath string) int {
	fs := flag.NewFlagSet("config generate", flag.ContinueOnError)
	configPath := fs.String("config-path", globalConfigPath, "directory to write generated configuration files")
	format := fs.String("format", "yaml", "output format: json or yaml")
	component := fs.String("component", "all", "component to generate: server, twin, listener, all")
	output := fs.String("output", "", "explicit output file path (overrides config-path)")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	configFormat, err := config.ParseFormat(*format)
	if err != nil {
		log.Error("invalid format", "error", err)
		return 1
	}

	ext := string(configFormat)

	type generateOp struct {
		name string
		fn   func(path string, format config.Format) error
	}

	ops := map[string]generateOp{
		"server":   {"server", config.GenerateServerConfig},
		"twin":     {"twin", config.GenerateTwinConfig},
		"listener": {"listener", config.GenerateListenerConfig},
	}

	run := func(name string, fn func(string, config.Format) error) int {
		path := *output
		if path == "" {
			path = filepath.Join(*configPath, name+"."+ext)
		}
		log.Info("generating configuration", "component", name, "path", path, "format", configFormat)
		if err := fn(path, configFormat); err != nil {
			log.Error("generation failed", "component", name, "error", err)
			return 1
		}
		log.Info("generated configuration", "component", name, "path", path)
		return 0
	}

	if *component == "all" {
		for name, op := range ops {
			if code := run(name, op.fn); code != 0 {
				return code
			}
		}
		return 0
	}

	op, ok := ops[*component]
	if !ok {
		log.Error("unknown component", "component", *component, "valid", "server, twin, listener, all")
		return 1
	}
	return run(op.name, op.fn)
}

func configPathFromEnv() string {
	if v := os.Getenv("PHEROMONE_SERVER_CONFIG_PATH"); v != "" {
		return v
	}
	return config.DefaultServerConfigPath
}

// runServe starts the HTTP management UI and REST API server.
func runServe(args []string, globalConfigPath string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	configPath := fs.String("config-path", globalConfigPath, "directory containing server configuration files")
	uiAddr := fs.String("ui-addr", "", "override UI bind address")
	uiPort := fs.Int("ui-port", 0, "override UI HTTP port")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Attempt to load config; fall back to defaults if not found.
	cfg, err := config.LoadServerConfig(*configPath)
	if err != nil {
		log.Warn("could not load server config, using defaults", "path", *configPath, "error", err)
		cfg = &config.ServerConfig{}
	}

	uiCfg := cfg.UI
	if !uiCfg.Enabled && uiCfg.Port == 0 {
		// No explicit config – use defaults
		uiCfg.Enabled = true
	}
	if uiCfg.Port == 0 {
		uiCfg.Port = 8081
	}
	if uiCfg.SecretKey == "" {
		uiCfg.SecretKey = "pheromone-default-secret-change-in-production"
	}
	if len(uiCfg.Users) == 0 {
		// Default admin:admin account when no users are configured.
		hash, _ := api.GeneratePasswordHash("admin")
		uiCfg.Users = []config.UIUser{
			{Username: "admin", PasswordHash: hash, Role: "admin"},
		}
	}
	// Apply flag overrides.
	if *uiAddr != "" {
		uiCfg.Address = *uiAddr
	}
	if *uiPort != 0 {
		uiCfg.Port = *uiPort
	}

	srv := api.New(uiCfg, log)
	srv.Seed()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	addr := uiCfg.Address + ":" + strconv.Itoa(uiCfg.Port)
	if uiCfg.Address == "" {
		addr = "0.0.0.0:" + strconv.Itoa(uiCfg.Port)
	}
	log.Info("starting pheromone management UI", "addr", addr)

	if err := srv.ListenAndServe(ctx); err != nil {
		log.Error("server error", "error", err)
		return 1
	}
	log.Info("server stopped")
	return 0
}

// runUI handles the 'ui' subcommand (currently: hash-password).
func runUI(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "ui requires a subcommand: hash-password")
		return 1
	}
	switch args[0] {
	case "hash-password":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: pheromone-server ui hash-password <password>")
			return 1
		}
		hash, err := api.GeneratePasswordHash(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		fmt.Println(hash)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown ui subcommand %q\n", args[0])
		return 1
	}
}
