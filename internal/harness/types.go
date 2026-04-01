package harness

import "time"

const (
	EvalRunStatusCompleted = "completed"
	EvalRunStatusFailed    = "failed"
)

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

type EvalRunSummary struct {
	ArtifactRoot string         `json:"artifact_root"`
	TotalRuns    int            `json:"total_runs"`
	Completed    int            `json:"completed"`
	Failed       int            `json:"failed"`
	AverageScore float64        `json:"average_score"`
	LatestRunID  string         `json:"latest_run_id"`
	LatestStatus string         `json:"latest_status"`
	FailureCodes map[string]int `json:"failure_codes"`
}

type BenchmarkCase struct {
	ID         string  `json:"id"`
	ReplayPath string  `json:"replay_path"`
	Weight     float64 `json:"weight"`
}

type BenchmarkPack struct {
	SchemaVersion string          `json:"schema_version"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Cases         []BenchmarkCase `json:"cases"`
}

type CandidateValidationResult struct {
	Valid     bool            `json:"valid"`
	Issues    []string        `json:"issues"`
	Candidate CandidateBundle `json:"candidate"`
}
