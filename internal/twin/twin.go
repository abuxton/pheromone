package twin

import (
	"encoding/json"
	"time"
)

// Twin represents a digital twin model
type Twin struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Type            string                 `json:"type"` // "os" or "workload"
	State           string                 `json:"state"` // "active", "inactive", "stale"
	ConfigVersion   int                    `json:"config_version"`
	LastHeartbeat   time.Time              `json:"last_heartbeat"`
	LastConfigPush  time.Time              `json:"last_config_push"`
	Metadata        map[string]string      `json:"metadata"`
	ConfigData      map[string]interface{} `json:"config_data"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

// ToJSON serializes the twin to JSON
func (t *Twin) ToJSON() ([]byte, error) {
	return json.Marshal(t)
}

// FromJSON deserializes JSON to a twin
func FromJSON(data []byte) (*Twin, error) {
	var t Twin
	err := json.Unmarshal(data, &t)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
