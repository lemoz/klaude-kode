package harness

import (
	"path/filepath"
	"testing"
)

func TestReplayFixturesLoadFromRepository(t *testing.T) {
	for _, fixturePath := range []string{
		repoFixturePath("benchmarks", "replays", "pass-basic.json"),
		repoFixturePath("benchmarks", "replays", "fail-basic.json"),
	} {
		pack, err := LoadReplayPack(fixturePath)
		if err != nil {
			t.Fatalf("LoadReplayPack(%s) returned error: %v", fixturePath, err)
		}
		if pack.Session.SessionID == "" {
			t.Fatalf("expected session id in replay fixture %s", fixturePath)
		}
		if len(pack.Events) == 0 {
			t.Fatalf("expected events in replay fixture %s", fixturePath)
		}
	}
}

func TestBenchmarkFixtureLoadsFromRepository(t *testing.T) {
	path := repoFixturePath("benchmarks", "packs", "mixed-basic.json")

	pack, err := LoadBenchmarkPack(path)
	if err != nil {
		t.Fatalf("LoadBenchmarkPack(%s) returned error: %v", path, err)
	}
	if pack.Name != "mixed-basic" {
		t.Fatalf("expected mixed-basic benchmark fixture, got %#v", pack)
	}
	if len(pack.Cases) != 2 {
		t.Fatalf("expected 2 benchmark fixture cases, got %#v", pack)
	}
}

func repoFixturePath(parts ...string) string {
	all := append([]string{"/Users/cdossman/klaude-kode"}, parts...)
	return filepath.Join(all...)
}
