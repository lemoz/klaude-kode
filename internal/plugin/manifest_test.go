package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateManifestAcceptsMinimalUpstreamShape(t *testing.T) {
	manifest := Manifest{
		Name:        "example-plugin",
		Description: "Example Claude Code plugin",
		Author: Author{
			Name:  "Anthropic",
			Email: "support@anthropic.com",
		},
	}

	issues := ValidateManifest(manifest)
	if len(issues) != 0 {
		t.Fatalf("expected no validation issues, got %#v", issues)
	}
}

func TestValidateManifestRejectsMissingRequiredFields(t *testing.T) {
	manifest := Manifest{}

	issues := ValidateManifest(manifest)
	if len(issues) != 3 {
		t.Fatalf("expected 3 validation issues, got %#v", issues)
	}
}

func TestValidateManifestRejectsInvalidNameAndEmail(t *testing.T) {
	manifest := Manifest{
		Name:        "Example Plugin",
		Description: "Example Claude Code plugin",
		Author: Author{
			Name:  "Anthropic",
			Email: "support-at-anthropic.com",
		},
	}

	issues := ValidateManifest(manifest)
	if len(issues) != 2 {
		t.Fatalf("expected 2 validation issues, got %#v", issues)
	}
}

func TestParseManifest(t *testing.T) {
	data := []byte(`{"name":"example-plugin","description":"Example Claude Code plugin","author":{"name":"Anthropic"}}`)

	manifest, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("ParseManifest returned error: %v", err)
	}

	if manifest.Name != "example-plugin" {
		t.Fatalf("expected name example-plugin, got %q", manifest.Name)
	}

	if manifest.Author.Name != "Anthropic" {
		t.Fatalf("expected author name Anthropic, got %q", manifest.Author.Name)
	}
}

func TestLoadManifest(t *testing.T) {
	root := t.TempDir()
	manifestDir := filepath.Join(root, ".claude-plugin")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	data := []byte(`{"name":"example-plugin","description":"Example Claude Code plugin","author":{"name":"Anthropic","email":"support@anthropic.com"}}`)
	if err := os.WriteFile(filepath.Join(manifestDir, "plugin.json"), data, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest returned error: %v", err)
	}

	if manifest.Author.Email != "support@anthropic.com" {
		t.Fatalf("expected author email support@anthropic.com, got %q", manifest.Author.Email)
	}
}
