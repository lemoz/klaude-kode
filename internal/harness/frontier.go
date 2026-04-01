package harness

import "sort"

func ListFrontier(root string, limit int) ([]FrontierEntry, error) {
	runs, err := ListIndexedEvalRuns(root)
	if err != nil {
		return nil, err
	}

	entries := make([]FrontierEntry, 0, len(runs))
	for _, run := range runs {
		entry := FrontierEntry{
			RunID:       run.ID,
			Kind:        run.Kind,
			Status:      run.Status,
			Score:       run.Score,
			CreatedAt:   run.CreatedAt,
			FailureCode: failureCode(run.Failure),
		}
		if run.Benchmark != nil {
			entry.Benchmark = run.Benchmark.Name
		}
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(leftIndex int, rightIndex int) bool {
		left := entries[leftIndex]
		right := entries[rightIndex]
		if left.Score != right.Score {
			return left.Score > right.Score
		}
		return left.CreatedAt.After(right.CreatedAt)
	})

	if limit > 0 && len(entries) > limit {
		return entries[:limit], nil
	}
	return entries, nil
}
