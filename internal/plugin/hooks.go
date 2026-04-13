package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const HooksConfigRelativePath = "hooks/hooks.json"

var supportedHookEvents = map[string]struct{}{
	"Notification":     {},
	"PostToolUse":      {},
	"PreCompact":       {},
	"PreToolUse":       {},
	"SessionEnd":       {},
	"SessionStart":     {},
	"Stop":             {},
	"SubagentStop":     {},
	"UserPromptSubmit": {},
}

type HookManifest struct {
	Description string                   `json:"description,omitempty"`
	Hooks       map[string][]HookMatcher `json:"hooks,omitempty"`
}

type HookMatcher struct {
	Description string        `json:"description,omitempty"`
	Matcher     string        `json:"matcher,omitempty"`
	Hooks       []HookCommand `json:"hooks,omitempty"`
}

type HookCommand struct {
	Type        string `json:"type"`
	Command     string `json:"command,omitempty"`
	Timeout     int    `json:"timeout,omitempty"`
	Description string `json:"description,omitempty"`
}

type HookStatus struct {
	Description  string   `json:"description,omitempty"`
	Events       []string `json:"events,omitempty"`
	MatcherCount int      `json:"matcher_count"`
	CommandCount int      `json:"command_count"`
}

func LoadHookManifest(root string) (HookManifest, error) {
	data, err := os.ReadFile(filepath.Join(root, HooksConfigRelativePath))
	if err != nil {
		return HookManifest{}, err
	}
	manifest, err := ParseHookManifest(data)
	if err != nil {
		return HookManifest{}, fmt.Errorf("parse hooks manifest: %w", err)
	}
	return manifest, nil
}

func ParseHookManifest(data []byte) (HookManifest, error) {
	var manifest HookManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return HookManifest{}, err
	}
	return manifest, nil
}

func InspectHookManifest(root string) (HookStatus, []ValidationIssue, error) {
	hooksRoot := filepath.Join(root, hooksDirName)
	info, err := os.Stat(hooksRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return HookStatus{}, nil, nil
		}
		if isNotDir(err) {
			return HookStatus{}, []ValidationIssue{{
				Field:   hooksDirName,
				Message: "hooks must be a directory",
			}}, nil
		}
		return HookStatus{}, nil, err
	}
	if !info.IsDir() {
		return HookStatus{}, []ValidationIssue{{
			Field:   hooksDirName,
			Message: "hooks must be a directory",
		}}, nil
	}

	manifest, err := LoadHookManifest(root)
	if err != nil {
		if os.IsNotExist(err) {
			return HookStatus{}, []ValidationIssue{{
				Field:   HooksConfigRelativePath,
				Message: "hooks/hooks.json is required when hooks are present",
			}}, nil
		}
		return HookStatus{}, nil, err
	}

	issues := ValidateHookManifest(manifest)
	status := HookStatus{
		Description: strings.TrimSpace(manifest.Description),
		Events:      hookEvents(manifest),
	}
	for _, matchers := range manifest.Hooks {
		status.MatcherCount += len(matchers)
		for _, matcher := range matchers {
			status.CommandCount += len(matcher.Hooks)
		}
	}

	return status, issues, nil
}

func ValidateHookManifest(manifest HookManifest) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	if len(manifest.Hooks) == 0 {
		return append(issues, ValidationIssue{
			Field:   "hooks",
			Message: "at least one hook event is required",
		})
	}

	for eventName, matchers := range manifest.Hooks {
		if _, ok := supportedHookEvents[eventName]; !ok {
			issues = append(issues, ValidationIssue{
				Field:   fmt.Sprintf("hooks.%s", eventName),
				Message: "event is not a recognized Claude Code hook event",
			})
		}
		if len(matchers) == 0 {
			issues = append(issues, ValidationIssue{
				Field:   fmt.Sprintf("hooks.%s", eventName),
				Message: "at least one matcher is required",
			})
			continue
		}
		for matcherIndex, matcher := range matchers {
			prefix := fmt.Sprintf("hooks.%s[%d]", eventName, matcherIndex)
			if len(matcher.Hooks) == 0 {
				issues = append(issues, ValidationIssue{
					Field:   prefix + ".hooks",
					Message: "at least one hook command is required",
				})
				continue
			}
			for hookIndex, hook := range matcher.Hooks {
				hookPrefix := fmt.Sprintf("%s.hooks[%d]", prefix, hookIndex)
				if strings.TrimSpace(hook.Type) == "" {
					issues = append(issues, ValidationIssue{
						Field:   hookPrefix + ".type",
						Message: "type is required",
					})
				} else if hook.Type != "command" {
					issues = append(issues, ValidationIssue{
						Field:   hookPrefix + ".type",
						Message: "only command hooks are supported",
					})
				}
				if strings.TrimSpace(hook.Command) == "" {
					issues = append(issues, ValidationIssue{
						Field:   hookPrefix + ".command",
						Message: "command is required",
					})
				}
				if hook.Timeout < 0 {
					issues = append(issues, ValidationIssue{
						Field:   hookPrefix + ".timeout",
						Message: "timeout must be zero or greater",
					})
				}
			}
		}
	}

	return issues
}

func hookEvents(manifest HookManifest) []string {
	events := make([]string, 0, len(manifest.Hooks))
	for eventName := range manifest.Hooks {
		events = append(events, eventName)
	}
	sort.Strings(events)
	return events
}
