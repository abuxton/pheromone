package twin

import (
	"encoding/json"
	"fmt"
	"time"
)

// Twin represents a digital twin model
type Twin struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Type           string                 `json:"type"`  // "os" or "workload"
	State          string                 `json:"state"` // "active", "inactive", "stale"
	ConfigVersion  int                    `json:"config_version"`
	LastHeartbeat  time.Time              `json:"last_heartbeat"`
	LastConfigPush time.Time              `json:"last_config_push"`
	Metadata       map[string]string      `json:"metadata"`
	ConfigData     map[string]interface{} `json:"config_data"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// ToJSON serializes the twin to JSON
func (t *Twin) ToJSON() ([]byte, error) {
	data, err := json.Marshal(t)
	if err != nil {
		return nil, fmt.Errorf("marshal twin: %w", err)
	}
	return data, nil
}

// FromJSON deserializes JSON to a twin
func FromJSON(data []byte) (*Twin, error) {
	var t Twin
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("unmarshal twin: %w", err)
	}
	return &t, nil
}
