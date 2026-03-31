package harness

import "time"

func SummarizeIndexedEvalRuns(root string) (EvalRunSummary, error) {
	runs, err := ListIndexedEvalRuns(root)
	if err != nil {
		return EvalRunSummary{}, err
	}

	summary := EvalRunSummary{
		ArtifactRoot: root,
		FailureCodes: map[string]int{},
	}
	if len(runs) == 0 {
		return summary, nil
	}

	var scoreTotal float64
	var latestTime time.Time
	for _, run := range runs {
		summary.TotalRuns++
		scoreTotal += run.Score
		switch run.Status {
		case EvalRunStatusCompleted:
			summary.Completed++
		case EvalRunStatusFailed:
			summary.Failed++
		}
		if run.Failure != nil && run.Failure.Code != "" {
			summary.FailureCodes[run.Failure.Code]++
		}
		if summary.LatestRunID == "" || run.CreatedAt.After(latestTime) {
			summary.LatestRunID = run.ID
			summary.LatestStatus = run.Status
			latestTime = run.CreatedAt
		}
	}
	summary.AverageScore = scoreTotal / float64(len(runs))
	return summary, nil
}
