package harness

import "time"

const (
	EvalRunStatusCompleted = "completed"
	EvalRunStatusFailed    = "failed"
	EvalRunKindReplay      = "replay"
	EvalRunKindBenchmark   = "benchmark"
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

type BenchmarkRunMetadata struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
	CaseCount   int    `json:"case_count"`
}

type BenchmarkCaseResult struct {
	ID         string              `json:"id"`
	ReplayPath string              `json:"replay_path"`
	Weight     float64             `json:"weight"`
	Status     string              `json:"status"`
	Score      float64             `json:"score"`
	Failure    *EvalFailureSummary `json:"failure"`
}

type BenchmarkCaseDiff struct {
	ID               string  `json:"id"`
	LeftStatus       string  `json:"left_status"`
	RightStatus      string  `json:"right_status"`
	LeftScore        float64 `json:"left_score"`
	RightScore       float64 `json:"right_score"`
	ScoreDelta       float64 `json:"score_delta"`
	LeftFailureCode  string  `json:"left_failure_code"`
	RightFailureCode string  `json:"right_failure_code"`
}

type EvalRun struct {
	ID            string                `json:"id"`
	Kind          string                `json:"kind"`
	SchemaVersion string                `json:"schema_version"`
	CreatedAt     time.Time             `json:"created_at"`
	Candidate     CandidateBundle       `json:"candidate"`
	ReplayPath    string                `json:"replay_path"`
	Benchmark     *BenchmarkRunMetadata `json:"benchmark"`
	Status        string                `json:"status"`
	Score         float64               `json:"score"`
	CaseResults   []BenchmarkCaseResult `json:"case_results"`
	Failure       *EvalFailureSummary   `json:"failure"`
}

type EvalRunDiff struct {
	LeftRunID        string              `json:"left_run_id"`
	RightRunID       string              `json:"right_run_id"`
	LeftKind         string              `json:"left_kind"`
	RightKind        string              `json:"right_kind"`
	LeftStatus       string              `json:"left_status"`
	RightStatus      string              `json:"right_status"`
	LeftScore        float64             `json:"left_score"`
	RightScore       float64             `json:"right_score"`
	ScoreDelta       float64             `json:"score_delta"`
	LeftFailureCode  string              `json:"left_failure_code"`
	RightFailureCode string              `json:"right_failure_code"`
	CaseDiffs        []BenchmarkCaseDiff `json:"case_diffs"`
}

type FrontierEntry struct {
	RunID       string    `json:"run_id"`
	Kind        string    `json:"kind"`
	Status      string    `json:"status"`
	Score       float64   `json:"score"`
	CreatedAt   time.Time `json:"created_at"`
	Benchmark   string    `json:"benchmark"`
	FailureCode string    `json:"failure_code"`
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
