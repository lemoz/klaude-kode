package harness

import "sort"

func DiffPersistedEvalRuns(root string, leftRunID string, rightRunID string) (EvalRunDiff, error) {
	left, err := LoadEvalRun(root, leftRunID)
	if err != nil {
		return EvalRunDiff{}, err
	}
	right, err := LoadEvalRun(root, rightRunID)
	if err != nil {
		return EvalRunDiff{}, err
	}
	return DiffEvalRuns(left, right), nil
}

func DiffEvalRuns(left EvalRun, right EvalRun) EvalRunDiff {
	diff := EvalRunDiff{
		LeftRunID:        left.ID,
		RightRunID:       right.ID,
		LeftKind:         left.Kind,
		RightKind:        right.Kind,
		LeftStatus:       left.Status,
		RightStatus:      right.Status,
		LeftScore:        left.Score,
		RightScore:       right.Score,
		ScoreDelta:       right.Score - left.Score,
		LeftFailureCode:  failureCode(left.Failure),
		RightFailureCode: failureCode(right.Failure),
	}

	leftCases := mapBenchmarkCases(left.CaseResults)
	rightCases := mapBenchmarkCases(right.CaseResults)
	caseIDs := make([]string, 0, len(leftCases)+len(rightCases))
	seen := map[string]bool{}
	for caseID := range leftCases {
		caseIDs = append(caseIDs, caseID)
		seen[caseID] = true
	}
	for caseID := range rightCases {
		if !seen[caseID] {
			caseIDs = append(caseIDs, caseID)
		}
	}
	sort.Strings(caseIDs)

	for _, caseID := range caseIDs {
		leftCase := leftCases[caseID]
		rightCase := rightCases[caseID]
		diff.CaseDiffs = append(diff.CaseDiffs, BenchmarkCaseDiff{
			ID:               caseID,
			LeftStatus:       leftCase.Status,
			RightStatus:      rightCase.Status,
			LeftScore:        leftCase.Score,
			RightScore:       rightCase.Score,
			ScoreDelta:       rightCase.Score - leftCase.Score,
			LeftFailureCode:  failureCode(leftCase.Failure),
			RightFailureCode: failureCode(rightCase.Failure),
		})
	}

	return diff
}

func mapBenchmarkCases(cases []BenchmarkCaseResult) map[string]BenchmarkCaseResult {
	result := make(map[string]BenchmarkCaseResult, len(cases))
	for _, benchmarkCase := range cases {
		result[benchmarkCase.ID] = benchmarkCase
	}
	return result
}

func failureCode(failure *EvalFailureSummary) string {
	if failure == nil {
		return ""
	}
	return failure.Code
}
