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

func TestMemoryProfileStoreSaveProfileAddsAndUpdatesProfiles(t *testing.T) {
	store := NewMemoryProfileStore()

	err := store.SaveProfile(contracts.AuthProfile{
		ID:           "openrouter-alt",
		Kind:         contracts.AuthProfileOpenRouterAPIKey,
		Provider:     contracts.ProviderOpenRouter,
		DisplayName:  "OpenRouter Alt",
		DefaultModel: "openrouter/auto",
		Settings: map[string]string{
			"credential_ref": "env://OPENROUTER_API_KEY",
			"api_base":       "https://openrouter.ai/api/v1",
		},
	})
	if err != nil {
		t.Fatalf("SaveProfile returned error: %v", err)
	}

	profile, err := store.GetProfile("openrouter-alt")
	if err != nil {
		t.Fatalf("GetProfile returned error: %v", err)
	}
	if profile.DisplayName != "OpenRouter Alt" {
		t.Fatalf("expected saved display name, got %q", profile.DisplayName)
	}

	err = store.SaveProfile(contracts.AuthProfile{
		ID:           "openrouter-alt",
		Kind:         contracts.AuthProfileOpenRouterAPIKey,
		Provider:     contracts.ProviderOpenRouter,
		DisplayName:  "OpenRouter Alt Updated",
		DefaultModel: "anthropic/claude-sonnet-4.5",
		Settings: map[string]string{
			"credential_ref": "env://OPENROUTER_API_KEY",
			"api_base":       "https://openrouter.ai/api/v1",
		},
	})
	if err != nil {
		t.Fatalf("SaveProfile update returned error: %v", err)
	}

	updated, err := store.GetProfile("openrouter-alt")
	if err != nil {
		t.Fatalf("GetProfile returned error: %v", err)
	}
	if updated.DisplayName != "OpenRouter Alt Updated" {
		t.Fatalf("expected updated display name, got %q", updated.DisplayName)
	}
	if updated.DefaultModel != "anthropic/claude-sonnet-4.5" {
		t.Fatalf("expected updated default model, got %q", updated.DefaultModel)
	}
}

func TestFileProfileStoreSetDefaultProfilePersists(t *testing.T) {
	root := t.TempDir()

	store, err := NewFileProfileStore(root)
	if err != nil {
		t.Fatalf("NewFileProfileStore returned error: %v", err)
	}

	err = store.SaveProfile(contracts.AuthProfile{
		ID:           "openrouter-alt",
		Kind:         contracts.AuthProfileOpenRouterAPIKey,
		Provider:     contracts.ProviderOpenRouter,
		DisplayName:  "OpenRouter Alt",
		DefaultModel: "openrouter/auto",
		Settings: map[string]string{
			"credential_ref": "env://OPENROUTER_API_KEY",
			"api_base":       "https://openrouter.ai/api/v1",
		},
	})
	if err != nil {
		t.Fatalf("SaveProfile returned error: %v", err)
	}
	if err := store.SetDefaultProfile("openrouter-alt"); err != nil {
		t.Fatalf("SetDefaultProfile returned error: %v", err)
	}

	resolved, err := store.ResolveProfile("", "")
	if err != nil {
		t.Fatalf("ResolveProfile returned error: %v", err)
	}
	if resolved.ID != "openrouter-alt" {
		t.Fatalf("expected openrouter-alt as default profile, got %s", resolved.ID)
	}
}
