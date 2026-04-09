package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateMarketplaceManifestAcceptsUpstreamShape(t *testing.T) {
	manifest := MarketplaceManifest{
		Name:        "claude-code-plugins",
		Version:     "1.0.0",
		Description: "Bundled plugins for Claude Code",
		Owner: Author{
			Name:  "Anthropic",
			Email: "support@anthropic.com",
		},
		Plugins: []MarketplacePlugin{
			{
				Name:        "agent-sdk-dev",
				Description: "Development kit for Agent SDK work",
				Source:      "./plugins/agent-sdk-dev",
				Category:    "development",
			},
			{
				Name:        "code-review",
				Description: "Automated PR review toolkit",
				Version:     "1.0.0",
				Author: Author{
					Name:  "Anthropic",
					Email: "support@anthropic.com",
				},
				Source:   "./plugins/code-review",
				Category: "productivity",
			},
		},
	}

	issues := ValidateMarketplaceManifest(manifest)
	if len(issues) != 0 {
		t.Fatalf("expected no validation issues, got %#v", issues)
	}
}

func TestValidateMarketplaceManifestRejectsMissingFieldsAndDuplicates(t *testing.T) {
	manifest := MarketplaceManifest{
		Plugins: []MarketplacePlugin{
			{Name: "duplicate"},
			{Name: "duplicate"},
		},
	}

	issues := ValidateMarketplaceManifest(manifest)
	if len(issues) != 11 {
		t.Fatalf("expected 11 validation issues, got %#v", issues)
	}
}

func TestParseMarketplaceManifest(t *testing.T) {
	data := []byte(`{"name":"claude-code-plugins","version":"1.0.0","description":"Bundled plugins","owner":{"name":"Anthropic"},"plugins":[{"name":"code-review","description":"PR review toolkit","source":"./plugins/code-review","category":"productivity"}]}`)

	manifest, err := ParseMarketplaceManifest(data)
	if err != nil {
		t.Fatalf("ParseMarketplaceManifest returned error: %v", err)
	}

	if manifest.Name != "claude-code-plugins" {
		t.Fatalf("expected marketplace name, got %q", manifest.Name)
	}
	if len(manifest.Plugins) != 1 || manifest.Plugins[0].Name != "code-review" {
		t.Fatalf("expected one plugin entry, got %#v", manifest.Plugins)
	}
}

func TestInspectMarketplaceBuildsStatusAndCategories(t *testing.T) {
	root := createMarketplaceRoot(t)

	descriptor, err := InspectMarketplace(root)
	if err != nil {
		t.Fatalf("InspectMarketplace returned error: %v", err)
	}

	status := descriptor.Status()
	if !status.Valid {
		t.Fatalf("expected valid marketplace status, got %#v", status)
	}
	if status.Name != "claude-code-plugins" || status.Version != "1.0.0" {
		t.Fatalf("expected marketplace identity, got %#v", status)
	}
	if status.PluginCount != 2 {
		t.Fatalf("expected plugin count 2, got %#v", status)
	}
	expectedCategories := []string{"development", "productivity"}
	if len(status.Categories) != len(expectedCategories) {
		t.Fatalf("expected categories %#v, got %#v", expectedCategories, status.Categories)
	}
	for index, category := range expectedCategories {
		if status.Categories[index] != category {
			t.Fatalf("expected category %q at index %d, got %#v", category, index, status.Categories)
		}
	}
}

func TestInspectMarketplaceRejectsEscapingAndMissingSources(t *testing.T) {
	root := t.TempDir()
	manifestDir := filepath.Join(root, ".claude-plugin")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	data := []byte(`{"name":"claude-code-plugins","version":"1.0.0","description":"Bundled plugins","owner":{"name":"Anthropic"},"plugins":[{"name":"escape","description":"Bad source","source":"../outside","category":"development"},{"name":"missing","description":"Missing source","source":"./plugins/missing","category":"productivity"}]}`)
	if err := os.WriteFile(filepath.Join(manifestDir, "marketplace.json"), data, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	descriptor, err := InspectMarketplace(root)
	if err != nil {
		t.Fatalf("InspectMarketplace returned error: %v", err)
	}

	issues := ValidateMarketplaceDescriptor(descriptor)
	if len(issues) != 2 {
		t.Fatalf("expected 2 validation issues, got %#v", issues)
	}

	status := descriptor.Status()
	if status.Valid {
		t.Fatalf("expected invalid marketplace status, got %#v", status)
	}
	if status.Error == "" {
		t.Fatalf("expected validation summary in marketplace status, got %#v", status)
	}
}

func createMarketplaceRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	manifestDir := filepath.Join(root, ".claude-plugin")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	for _, relativeDir := range []string{
		filepath.Join("plugins", "agent-sdk-dev"),
		filepath.Join("plugins", "code-review"),
	} {
		if err := os.MkdirAll(filepath.Join(root, relativeDir), 0o755); err != nil {
			t.Fatalf("MkdirAll returned error for %s: %v", relativeDir, err)
		}
	}
	data := []byte(`{"$schema":"https://anthropic.com/claude-code/marketplace.schema.json","name":"claude-code-plugins","version":"1.0.0","description":"Bundled plugins for Claude Code","owner":{"name":"Anthropic","email":"support@anthropic.com"},"plugins":[{"name":"agent-sdk-dev","description":"Development kit for Agent SDK work","source":"./plugins/agent-sdk-dev","category":"development"},{"name":"code-review","description":"Automated PR review toolkit","version":"1.0.0","author":{"name":"Anthropic","email":"support@anthropic.com"},"source":"./plugins/code-review","category":"productivity"}]}`)
	if err := os.WriteFile(filepath.Join(manifestDir, "marketplace.json"), data, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return root
}
