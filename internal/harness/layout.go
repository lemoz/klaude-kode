package harness

const (
	DefaultArtifactDirName = ".klaude-harness"
	DirCandidates          = "candidates"
	DirRuns                = "runs"
	DirReplayPacks         = "replay-packs"
	DirBenchmarks          = "benchmarks"
	DirIndexes             = "indexes"
	DirReports             = "reports"
	RunMetadataFile        = "run.json"
)

var artifactDirs = []string{
	DirCandidates,
	DirRuns,
	DirReplayPacks,
	DirBenchmarks,
	DirIndexes,
	DirReports,
}

func RequiredArtifactDirs() []string {
	dirs := make([]string, len(artifactDirs))
	copy(dirs, artifactDirs)
	return dirs
}
