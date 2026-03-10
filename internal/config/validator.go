package config

import (
	"errors"
	"fmt"
	"strings"
)

// ValidationError is returned by Validate functions when one or more fields
// fail validation. All individual errors are collected so callers receive a
// complete report in a single call.
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("config validation failed (%d error(s)):\n  - %s",
		len(e.Errors), strings.Join(e.Errors, "\n  - "))
}

// ValidateServerConfig checks that cfg is internally consistent and contains
// all required fields. It returns a *ValidationError listing every problem
// found, or nil if the configuration is valid.
func ValidateServerConfig(cfg *ServerConfig) error {
	var errs []string

	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		errs = append(errs, fmt.Sprintf("server.port %d is out of range (1-65535)", cfg.Server.Port))
	}

	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if cfg.Server.LogLevel != "" && !validLogLevels[strings.ToLower(cfg.Server.LogLevel)] {
		errs = append(errs, fmt.Sprintf("server.log_level %q is invalid; must be one of debug, info, warn, error", cfg.Server.LogLevel))
	}

	// TLS: both cert and key must be provided together.
	if (cfg.Server.TLSCertFile != "") != (cfg.Server.TLSKeyFile != "") {
		errs = append(errs, "server.tls_cert_file and server.tls_key_file must both be set or both be empty")
	}

	twinIDs := make(map[string]int)
	for i, twin := range cfg.Twins {
		if err := validateTwinConfig(&twin); err != nil {
			var ve *ValidationError
			if errors.As(err, &ve) {
				for _, e := range ve.Errors {
					errs = append(errs, fmt.Sprintf("twins[%d]: %s", i, e))
				}
			}
		}
		if _, seen := twinIDs[twin.ID]; seen {
			errs = append(errs, fmt.Sprintf("twins[%d]: duplicate twin id %q", i, twin.ID))
		}
		twinIDs[twin.ID] = i
	}

	listenerNames := make(map[string]int)
	for i, l := range cfg.Listeners {
		if err := validateListenerConfig(&l); err != nil {
			var ve *ValidationError
			if errors.As(err, &ve) {
				for _, e := range ve.Errors {
					errs = append(errs, fmt.Sprintf("listeners[%d]: %s", i, e))
				}
			}
		}
		if _, seen := listenerNames[l.Name]; seen {
			errs = append(errs, fmt.Sprintf("listeners[%d]: duplicate listener name %q", i, l.Name))
		}
		listenerNames[l.Name] = i
	}

	if err := ValidateUIConfig(&cfg.UI); err != nil {
		var ve *ValidationError
		if errors.As(err, &ve) {
			errs = append(errs, ve.Errors...)
		}
	}

	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	return nil
}

// ValidateUIConfig validates the UIConfig section of the server configuration.
func ValidateUIConfig(cfg *UIConfig) error {
	if !cfg.Enabled {
		return nil
	}
	var errs []string

	if cfg.Port < 1 || cfg.Port > 65535 {
		errs = append(errs, fmt.Sprintf("ui.port %d is out of range (1-65535)", cfg.Port))
	}

	if cfg.SecretKey == "" {
		errs = append(errs, "ui.secret_key is required when ui is enabled")
	}

	validRoles := map[string]bool{"admin": true, "viewer": true}
	usernames := make(map[string]int)
	for i, u := range cfg.Users {
		if u.Username == "" {
			errs = append(errs, fmt.Sprintf("ui.users[%d]: username is required", i))
		}
		if u.PasswordHash == "" {
			errs = append(errs, fmt.Sprintf("ui.users[%d]: password_hash is required", i))
		}
		if u.Role == "" {
			errs = append(errs, fmt.Sprintf("ui.users[%d]: role is required", i))
		} else if !validRoles[strings.ToLower(u.Role)] {
			errs = append(errs, fmt.Sprintf("ui.users[%d]: role %q is invalid; must be admin or viewer", i, u.Role))
		}
		if _, seen := usernames[u.Username]; seen && u.Username != "" {
			errs = append(errs, fmt.Sprintf("ui.users[%d]: duplicate username %q", i, u.Username))
		}
		usernames[u.Username] = i
	}

	if tlsErrs := validateUITLSConfig(&cfg.TLS); len(tlsErrs) > 0 {
		errs = append(errs, tlsErrs...)
	}

	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	return nil
}

// validateUITLSConfig validates the UITLSConfig section.
// Returns a slice of error strings (empty on success).
func validateUITLSConfig(t *UITLSConfig) []string {
	if !t.Enabled {
		return nil
	}
	var errs []string

	// The server must always have its own certificate and private key.
	if t.CertFile == "" {
		errs = append(errs, "ui.tls.cert_file is required when ui.tls.enabled is true")
	}
	if t.KeyFile == "" {
		errs = append(errs, "ui.tls.key_file is required when ui.tls.enabled is true")
	}

	// ca_bundle_file and use_os_cert_store are mutually exclusive.
	if t.UseOSCertStore && t.CABundleFile != "" {
		errs = append(errs, "ui.tls.ca_bundle_file must not be set when ui.tls.use_os_cert_store is true")
	}

	return errs
}
func ValidateAgentConfig(cfg *AgentConfig) error {
	var errs []string

	if cfg.Agent.ServerAddr == "" {
		errs = append(errs, "agent.server_addr is required")
	}

	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if cfg.Agent.LogLevel != "" && !validLogLevels[strings.ToLower(cfg.Agent.LogLevel)] {
		errs = append(errs, fmt.Sprintf("agent.log_level %q is invalid; must be one of debug, info, warn, error", cfg.Agent.LogLevel))
	}

	if err := validateDistribution(&cfg.Agent.Distribution); err != nil {
		var ve *ValidationError
		if errors.As(err, &ve) {
			errs = append(errs, ve.Errors...)
		}
	}

	skillNames := make(map[string]int)
	for i, s := range cfg.Skills {
		if s.Name == "" {
			errs = append(errs, fmt.Sprintf("skills[%d]: name is required", i))
		}
		if _, seen := skillNames[s.Name]; seen && s.Name != "" {
			errs = append(errs, fmt.Sprintf("skills[%d]: duplicate skill name %q", i, s.Name))
		}
		skillNames[s.Name] = i
	}

	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	return nil
}

// validateTwinConfig validates an individual TwinConfig.
func validateTwinConfig(t *TwinConfig) error {
	var errs []string
	if t.ID == "" {
		errs = append(errs, "id is required")
	}
	validTypes := map[string]bool{"os": true, "workload": true}
	if t.Type == "" {
		errs = append(errs, "type is required (must be os or workload)")
	} else if !validTypes[strings.ToLower(t.Type)] {
		errs = append(errs, fmt.Sprintf("type %q is invalid; must be os or workload", t.Type))
	}
	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	return nil
}

// validateListenerConfig validates an individual ListenerConfig.
func validateListenerConfig(l *ListenerConfig) error {
	var errs []string
	if l.Name == "" {
		errs = append(errs, "name is required")
	}
	validTypes := map[string]bool{"webhook": true, "nats": true, "kafka": true}
	if l.Type == "" {
		errs = append(errs, "type is required (must be webhook, nats, or kafka)")
	} else if !validTypes[strings.ToLower(l.Type)] {
		errs = append(errs, fmt.Sprintf("type %q is invalid; must be webhook, nats, or kafka", l.Type))
	}
	if l.Address == "" {
		errs = append(errs, "address is required")
	}
	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	return nil
}

// validateDistribution validates DistributionSettings.
func validateDistribution(d *DistributionSettings) error {
	if d.Mode == "" {
		return nil // will be defaulted by applyAgentDefaults
	}
	validModes := map[string]bool{"server": true, "remote": true, "local": true}
	if !validModes[strings.ToLower(d.Mode)] {
		return &ValidationError{Errors: []string{
			fmt.Sprintf("agent.distribution.mode %q is invalid; must be server, remote, or local", d.Mode),
		}}
	}
	if d.Mode == "remote" && d.RemoteURL == "" {
		return &ValidationError{Errors: []string{
			"agent.distribution.remote_url is required when mode is remote",
		}}
	}
	if d.Mode == "local" && d.LocalDir == "" {
		return &ValidationError{Errors: []string{
			"agent.distribution.local_dir is required when mode is local",
		}}
	}
	return nil
}
