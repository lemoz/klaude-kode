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
		Version:     "1.0.0",
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
	if len(issues) != 4 {
		t.Fatalf("expected 4 validation issues, got %#v", issues)
	}
}

func TestValidateManifestRejectsInvalidNameAndEmail(t *testing.T) {
	manifest := Manifest{
		Name:        "Example Plugin",
		Description: "Example Claude Code plugin",
		Version:     "1.0.0",
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
	data := []byte(`{"name":"example-plugin","description":"Example Claude Code plugin","version":"1.0.0","author":{"name":"Anthropic"}}`)

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

	if manifest.Version != "1.0.0" {
		t.Fatalf("expected version 1.0.0, got %q", manifest.Version)
	}
}

func TestLoadManifest(t *testing.T) {
	root := t.TempDir()
	manifestDir := filepath.Join(root, ".claude-plugin")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	data := []byte(`{"name":"example-plugin","description":"Example Claude Code plugin","version":"1.0.0","author":{"name":"Anthropic","email":"support@anthropic.com"}}`)
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

func TestInspectDiscoversPluginContributions(t *testing.T) {
	root := t.TempDir()
	manifestDir := filepath.Join(root, ".claude-plugin")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	data := []byte(`{"name":"example-plugin","description":"Example Claude Code plugin","version":"1.2.3","author":{"name":"Anthropic","email":"support@anthropic.com"}}`)
	if err := os.WriteFile(filepath.Join(manifestDir, "plugin.json"), data, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Example plugin\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error for README.md: %v", err)
	}

	for _, relativePath := range []string{
		filepath.Join("commands", "review.md"),
		filepath.Join("commands", "diagnose.md"),
		filepath.Join("agents", "frontend.md"),
		filepath.Join("skills", "deploy", "SKILL.md"),
		filepath.Join("skills", "ops", "incident", "SKILL.md"),
		".mcp.json",
	} {
		fullPath := filepath.Join(root, relativePath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("MkdirAll returned error for %s: %v", relativePath, err)
		}
		if err := os.WriteFile(fullPath, []byte("content"), 0o644); err != nil {
			t.Fatalf("WriteFile returned error for %s: %v", relativePath, err)
		}
	}

	descriptor, err := Inspect(root)
	if err != nil {
		t.Fatalf("Inspect returned error: %v", err)
	}

	if len(descriptor.Commands) != 2 || descriptor.Commands[0] != "diagnose" || descriptor.Commands[1] != "review" {
		t.Fatalf("expected sorted commands, got %#v", descriptor.Commands)
	}

	if len(descriptor.Agents) != 1 || descriptor.Agents[0] != "frontend" {
		t.Fatalf("expected frontend agent, got %#v", descriptor.Agents)
	}

	expectedSkills := []string{"deploy", "ops/incident"}
	if len(descriptor.Skills) != len(expectedSkills) {
		t.Fatalf("expected %d skills, got %#v", len(expectedSkills), descriptor.Skills)
	}
	for index, skill := range expectedSkills {
		if descriptor.Skills[index] != skill {
			t.Fatalf("expected skill %q at index %d, got %#v", skill, index, descriptor.Skills)
		}
	}

	if !descriptor.HasREADME {
		t.Fatalf("expected descriptor to report README.md")
	}

	if err := os.MkdirAll(filepath.Join(root, "hooks"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error for hooks: %v", err)
	}
	hooksConfig := []byte(`{"description":"Example plugin hooks","hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"./hooks-handlers/session-start.sh"}]}],"PostToolUse":[{"matcher":"Edit|Write","hooks":[{"type":"command","command":"python3 ${CLAUDE_PLUGIN_ROOT}/hooks/notify.py","timeout":5}]}]}}`)
	if err := os.WriteFile(filepath.Join(root, "hooks", "hooks.json"), hooksConfig, 0o644); err != nil {
		t.Fatalf("WriteFile returned error for hooks.json: %v", err)
	}

	descriptor, err = Inspect(root)
	if err != nil {
		t.Fatalf("Inspect returned error after hooks config: %v", err)
	}

	if descriptor.HookCount != 2 {
		t.Fatalf("expected descriptor to report 2 configured hooks, got %#v", descriptor)
	}
	if len(descriptor.HookEvents) != 2 || descriptor.HookEvents[0] != "PostToolUse" || descriptor.HookEvents[1] != "SessionStart" {
		t.Fatalf("expected descriptor to project hook events, got %#v", descriptor.HookEvents)
	}

	if !descriptor.HasMCPConfig {
		t.Fatalf("expected descriptor to report .mcp.json")
	}

	status := descriptor.StatusPayload("")
	if !status.Loaded || !status.Valid {
		t.Fatalf("expected valid loaded status, got %#v", status)
	}
	if status.PluginID != "example-plugin" || status.Version != "1.2.3" {
		t.Fatalf("expected plugin identity in status, got %#v", status)
	}
	if len(status.Skills) != 2 || status.MCPServers != 1 || status.HookCount != 2 || len(status.HookEvents) != 2 {
		t.Fatalf("expected skills, hooks, and mcp projection in status, got %#v", status)
	}
}

func TestDescriptorStatusPayloadCarriesValidationErrors(t *testing.T) {
	descriptor := Descriptor{
		Manifest: Manifest{
			Name:        "bad plugin",
			Description: "Broken plugin",
			Author: Author{
				Name: "Anthropic",
			},
		},
	}

	status := descriptor.StatusPayload("")
	if status.Valid || status.Loaded {
		t.Fatalf("expected invalid unloaded status, got %#v", status)
	}
	if status.PluginID != "bad plugin" {
		t.Fatalf("expected plugin id to fall back to manifest name, got %#v", status)
	}
	if status.Error == "" {
		t.Fatalf("expected validation summary in error field, got %#v", status)
	}
}

func TestInspectReportsMissingReadmeAndMalformedContributionLayout(t *testing.T) {
	root := t.TempDir()
	manifestDir := filepath.Join(root, ".claude-plugin")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	data := []byte(`{"name":"example-plugin","description":"Example Claude Code plugin","version":"1.2.3","author":{"name":"Anthropic","email":"support@anthropic.com"}}`)
	if err := os.WriteFile(filepath.Join(manifestDir, "plugin.json"), data, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(root, "commands", "nested"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error for nested command: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "agents"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error for agents: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "agents", "oops.txt"), []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error for invalid agent file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "hooks"), []byte("not-a-dir"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error for hooks: %v", err)
	}

	descriptor, err := Inspect(root)
	if err != nil {
		t.Fatalf("Inspect returned error: %v", err)
	}

	issues := ValidateDescriptor(descriptor)
	if len(issues) != 4 {
		t.Fatalf("expected 4 validation issues, got %#v", issues)
	}

	status := descriptor.StatusPayload("")
	if status.Valid || status.Loaded {
		t.Fatalf("expected invalid unloaded status, got %#v", status)
	}
	if status.Error == "" {
		t.Fatalf("expected status error summary, got %#v", status)
	}
}
