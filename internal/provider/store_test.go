package provider

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

func TestMemoryProfileStoreSeedsDefaultProfiles(t *testing.T) {
	store := NewMemoryProfileStore()

	profiles, err := store.ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles returned error: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("expected 2 seeded profiles, got %d", len(profiles))
	}

	anthropic, err := store.ResolveProfile("", "")
	if err != nil {
		t.Fatalf("ResolveProfile returned error: %v", err)
	}
	if anthropic.ID != "anthropic-main" {
		t.Fatalf("expected default anthropic profile, got %s", anthropic.ID)
	}

	openRouter, err := store.ResolveProfile("", "openrouter/auto")
	if err != nil {
		t.Fatalf("ResolveProfile returned error: %v", err)
	}
	if openRouter.ID != "openrouter-main" {
		t.Fatalf("expected openrouter default profile, got %s", openRouter.ID)
	}
}

func TestFileProfileStoreCreatesProfilesJSON(t *testing.T) {
	root := t.TempDir()

	store, err := NewFileProfileStore(root)
	if err != nil {
		t.Fatalf("NewFileProfileStore returned error: %v", err)
	}

	profile, err := store.ResolveProfile("", "")
	if err != nil {
		t.Fatalf("ResolveProfile returned error: %v", err)
	}
	if profile.ID != "anthropic-main" {
		t.Fatalf("expected anthropic-main default profile, got %s", profile.ID)
	}

	profilesPath := filepath.Join(root, "profiles", "profiles.json")
	if _, err := os.Stat(profilesPath); err != nil {
		t.Fatalf("expected profiles.json at %s: %v", profilesPath, err)
	}
}

func TestResolveProfileRejectsUnknownExplicitProfile(t *testing.T) {
	store := NewMemoryProfileStore()

	_, err := store.ResolveProfile("missing-profile", "")
	if !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("expected ErrProfileNotFound, got %v", err)
	}
}

func TestResolveProfileReturnsExplicitStoredProfile(t *testing.T) {
	store := NewMemoryProfileStore()

	profile, err := store.ResolveProfile("openrouter-main", "")
	if err != nil {
		t.Fatalf("ResolveProfile returned error: %v", err)
	}
	if profile.Provider != contracts.ProviderOpenRouter {
		t.Fatalf("expected openrouter provider, got %s", profile.Provider)
	}
	if profile.DefaultModel != "anthropic/claude-sonnet-4.5" {
		t.Fatalf("expected seeded openrouter default model, got %q", profile.DefaultModel)
	}
}
