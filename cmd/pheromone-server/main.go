// pheromone-server is the Pheromone server process.
//
// Usage:
//
//	pheromone-server [--config-path <dir>] [--format json|yaml]
//	pheromone-server config validate [--config-path <dir>]
//	pheromone-server config generate [--config-path <dir>] [--format json|yaml] [--component server|twin|listener|all]
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/abuxton/pheromone/internal/config"
)

const usageText = `pheromone-server — Pheromone server process

Usage:
  pheromone-server [global flags] <command> [command flags]

Commands:
  config validate   Validate all configuration files in the config directory
  config generate   Generate a default configuration file

Global Flags:
  --config-path <dir>   Directory containing server configuration files
                        (default: /etc/pheromone/server, env: PHEROMONE_SERVER_CONFIG_PATH)

Config Generate Flags:
  --format  <fmt>       Output format: json or yaml (default: yaml)
  --component <name>    Component to generate config for:
                        server, twin, listener, all (default: all)
  --output  <path>      Output file path (default: <config-path>/<component>.<format>)

Examples:
  # Validate the current server configuration
  pheromone-server config validate

  # Generate a default server config in YAML
  pheromone-server config generate --format yaml --component server

  # Generate all component configs in JSON to a custom path
  pheromone-server config generate --format json --component all --config-path /opt/pheromone/server
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
