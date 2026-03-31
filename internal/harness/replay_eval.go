package harness

import (
	"fmt"
	"time"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

func RunReplayEval(candidateRoot string, replayPath string) (EvalRun, error) {
	validation, err := ValidateCandidateRoot(candidateRoot)
	if err != nil {
		return EvalRun{}, err
	}

	run := EvalRun{
		ID:            fmt.Sprintf("run_%d", time.Now().UTC().UnixNano()),
		SchemaVersion: contracts.SchemaVersionV1,
		CreatedAt:     time.Now().UTC(),
		Candidate:     validation.Candidate,
		ReplayPath:    replayPath,
		Status:        EvalRunStatusCompleted,
		Score:         1,
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

	pack, err := LoadReplayPack(replayPath)
	if err != nil {
		return EvalRun{}, err
	}
	if pack.Session.SessionID == "" || len(pack.Events) == 0 {
		run.Status = EvalRunStatusFailed
		run.Score = 0
		run.Failure = &EvalFailureSummary{
			Code:      "invalid_replay_pack",
			Message:   "replay pack is missing required session data",
			Retryable: false,
		}
		return run, nil
	}
	if pack.Summary.TerminalOutcome != contracts.TerminalOutcomeSuccess {
		run.Status = EvalRunStatusFailed
		run.Score = 0
		run.Failure = &EvalFailureSummary{
			Code:      "replay_terminal_outcome",
			Message:   fmt.Sprintf("replay pack terminal outcome is %s", pack.Summary.TerminalOutcome),
			Retryable: false,
		}
		return run, nil
	}

	return run, nil
}
