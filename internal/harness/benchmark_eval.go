package harness

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

func RunBenchmarkEval(candidateRoot string, benchmarkPath string) (EvalRun, error) {
	validation, err := ValidateCandidateRoot(candidateRoot)
	if err != nil {
		return EvalRun{}, err
	}

	pack, err := LoadBenchmarkPack(benchmarkPath)
	if err != nil {
		return EvalRun{}, err
	}

	run := EvalRun{
		ID:            fmt.Sprintf("run_%d", time.Now().UTC().UnixNano()),
		Kind:          EvalRunKindBenchmark,
		SchemaVersion: contracts.SchemaVersionV1,
		CreatedAt:     time.Now().UTC(),
		Candidate:     validation.Candidate,
		Benchmark: &BenchmarkRunMetadata{
			Name:        pack.Name,
			Description: pack.Description,
			Path:        benchmarkPath,
			CaseCount:   len(pack.Cases),
		},
		Status: EvalRunStatusCompleted,
		Score:  1,
	}

	if !validation.Valid {
		run.Status = EvalRunStatusFailed
		run.Score = 0
		run.Failure = &EvalFailureSummary{
			Code:      "invalid_candidate",
			Message:   "candidate validation failed",
			Retryable: false,
		}
		return run, nil
	}
	if len(pack.Cases) == 0 {
		run.Status = EvalRunStatusFailed
		run.Score = 0
		run.Failure = &EvalFailureSummary{
			Code:      "invalid_benchmark_pack",
			Message:   "benchmark pack contains no cases",
			Retryable: false,
		}
		return run, nil
	}

	baseDir := filepath.Dir(benchmarkPath)
	var weightedScore float64
	var totalWeight float64
	failedCases := 0

	for _, benchmarkCase := range pack.Cases {
		weight := benchmarkCase.Weight
		if weight <= 0 {
			weight = 1
		}

		replayPath := benchmarkCase.ReplayPath
		if !filepath.IsAbs(replayPath) {
			replayPath = filepath.Join(baseDir, replayPath)
		}

		caseResult := BenchmarkCaseResult{
			ID:         benchmarkCase.ID,
			ReplayPath: replayPath,
			Weight:     weight,
			Status:     EvalRunStatusCompleted,
			Score:      1,
		}

		replayRun, replayErr := RunReplayEval(candidateRoot, replayPath)
		if replayErr != nil {
			caseResult.Status = EvalRunStatusFailed
			caseResult.Score = 0
			caseResult.Failure = &EvalFailureSummary{
				Code:      "benchmark_case_error",
				Message:   replayErr.Error(),
				Retryable: false,
			}
		} else {
			caseResult.Status = replayRun.Status
			caseResult.Score = replayRun.Score
			caseResult.Failure = replayRun.Failure
		}

		if caseResult.Status != EvalRunStatusCompleted {
			failedCases++
		}
		weightedScore += caseResult.Score * weight
		totalWeight += weight
		run.CaseResults = append(run.CaseResults, caseResult)
	}

	if totalWeight > 0 {
		run.Score = weightedScore / totalWeight
	}
	if failedCases > 0 {
		run.Status = EvalRunStatusFailed
		run.Failure = &EvalFailureSummary{
			Code:      "benchmark_cases_failed",
			Message:   fmt.Sprintf("%d benchmark cases failed", failedCases),
			Retryable: false,
		}
	}

	return run, nil
}
