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

type EvalFailureSummary struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type EvalRun struct {
	ID            string              `json:"id"`
	SchemaVersion string              `json:"schema_version"`
	CreatedAt     time.Time           `json:"created_at"`
	Candidate     CandidateBundle     `json:"candidate"`
	ReplayPath    string              `json:"replay_path"`
	Status        string              `json:"status"`
	Score         float64             `json:"score"`
	Failure       *EvalFailureSummary `json:"failure"`
}
