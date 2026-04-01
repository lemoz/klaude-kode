package harness

import (
	"encoding/json"
	"os"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

func LoadReplayPack(path string) (contracts.ReplayPack, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return contracts.ReplayPack{}, err
	}

	var pack contracts.ReplayPack
	if err := json.Unmarshal(data, &pack); err != nil {
		return contracts.ReplayPack{}, err
	}
	return pack, nil
}

func LoadBenchmarkPack(path string) (BenchmarkPack, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return BenchmarkPack{}, err
	}

	var pack BenchmarkPack
	if err := json.Unmarshal(data, &pack); err != nil {
		return BenchmarkPack{}, err
	}
	return pack, nil
}
