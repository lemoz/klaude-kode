package hooks

import "testing"

func TestValidateConfigAcceptsSupportedHooks(t *testing.T) {
	config := Config{
		Hooks: []Definition{
			{
				ID:             "session-start-log",
				Event:          EventSessionStart,
				Command:        "echo start",
				TimeoutSeconds: 10,
				Enabled:        true,
			},
			{
				ID:             "post-tool-log",
				Event:          EventPostToolUse,
				Command:        "echo tool",
				TimeoutSeconds: 5,
				Enabled:        true,
			},
		},
	}

	issues := ValidateConfig(config)
	if len(issues) != 0 {
		t.Fatalf("expected no validation issues, got %#v", issues)
	}
}

func TestValidateConfigRejectsMissingAndInvalidFields(t *testing.T) {
	config := Config{
		Hooks: []Definition{
			{
				ID:             "Bad Hook",
				Command:        "",
				TimeoutSeconds: -1,
			},
		},
	}

	issues := ValidateConfig(config)
	if len(issues) != 4 {
		t.Fatalf("expected 4 validation issues, got %#v", issues)
	}
}

func TestValidateConfigRejectsDuplicateIDs(t *testing.T) {
	config := Config{
		Hooks: []Definition{
			{
				ID:      "shared-id",
				Event:   EventSessionStart,
				Command: "echo start",
			},
			{
				ID:      "shared-id",
				Event:   EventSessionEnd,
				Command: "echo end",
			},
		},
	}

	issues := ValidateConfig(config)
	if len(issues) != 1 {
		t.Fatalf("expected 1 validation issue, got %#v", issues)
	}
}
