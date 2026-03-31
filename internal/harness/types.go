package harness

import "time"

type CandidateBundle struct {
	SchemaVersion    string    `json:"schema_version"`
	CreatedAt        time.Time `json:"created_at"`
	Root             string    `json:"root"`
	EngineVersion    string    `json:"engine_version"`
	ShellVersion     string    `json:"shell_version"`
	DefaultProfileID string    `json:"default_profile_id"`
	DefaultModel     string    `json:"default_model"`
}
