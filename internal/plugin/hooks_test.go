package plugin

import "testing"

func TestParseHookManifest(t *testing.T) {
	data := []byte(`{"description":"Example hooks","hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"./hooks-handlers/session-start.sh"}]}],"PreToolUse":[{"matcher":"Edit|Write","hooks":[{"type":"command","command":"python3 ${CLAUDE_PLUGIN_ROOT}/hooks/security.py","timeout":10}]}]}}`)

	manifest, err := ParseHookManifest(data)
	if err != nil {
		t.Fatalf("ParseHookManifest returned error: %v", err)
	}

	if manifest.Description != "Example hooks" {
		t.Fatalf("expected description to parse, got %#v", manifest)
	}
	if len(manifest.Hooks["SessionStart"]) != 1 {
		t.Fatalf("expected session start hook matcher, got %#v", manifest.Hooks)
	}
}

func TestValidateHookManifestRejectsMalformedHooks(t *testing.T) {
	manifest := HookManifest{
		Hooks: map[string][]HookMatcher{
			"UnknownEvent": {
				{
					Hooks: []HookCommand{
						{Type: "script", Command: "", Timeout: -1},
					},
				},
			},
			"SessionStart": {
				{},
			},
		},
	}

	issues := ValidateHookManifest(manifest)
	if len(issues) != 5 {
		t.Fatalf("expected 5 validation issues, got %#v", issues)
	}
}
