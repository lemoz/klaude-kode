package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

const ManifestRelativePath = ".claude-plugin/plugin.json"
const MCPConfigRelativePath = ".mcp.json"

const (
	commandsDirName = "commands"
	agentsDirName   = "agents"
	hooksDirName    = "hooks"
	readmeFileName  = "README.md"
	skillsDirName   = "skills"
	skillFileName   = "SKILL.md"
)

var pluginNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type Author struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

type Manifest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Author      Author   `json:"author"`
	License     string   `json:"license,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
	Repository  string   `json:"repository,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
}

type Descriptor struct {
	Root             string            `json:"root"`
	Manifest         Manifest          `json:"manifest"`
	Commands         []string          `json:"commands,omitempty"`
	Agents           []string          `json:"agents,omitempty"`
	Skills           []string          `json:"skills,omitempty"`
	HookEvents       []string          `json:"hook_events,omitempty"`
	HookCount        int               `json:"hook_count"`
	HasREADME        bool              `json:"has_readme"`
	HasMCPConfig     bool              `json:"has_mcp_config"`
	ValidationIssues []ValidationIssue `json:"validation_issues,omitempty"`
}

type ValidationIssue struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func LoadManifest(root string) (Manifest, error) {
	path := filepath.Join(root, ManifestRelativePath)
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	manifest, err := ParseManifest(data)
	if err != nil {
		return Manifest{}, fmt.Errorf("parse plugin manifest: %w", err)
	}
	return manifest, nil
}

func ParseManifest(data []byte) (Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func Inspect(root string) (Descriptor, error) {
	manifest, err := LoadManifest(root)
	if err != nil {
		return Descriptor{}, err
	}

	commands, commandIssues, err := listMarkdownBasenames(filepath.Join(root, commandsDirName), commandsDirName)
	if err != nil {
		return Descriptor{}, fmt.Errorf("list commands: %w", err)
	}

	agents, agentIssues, err := listMarkdownBasenames(filepath.Join(root, agentsDirName), agentsDirName)
	if err != nil {
		return Descriptor{}, fmt.Errorf("list agents: %w", err)
	}

	skills, skillIssues, err := listSkills(filepath.Join(root, skillsDirName))
	if err != nil {
		return Descriptor{}, fmt.Errorf("list skills: %w", err)
	}

	hasReadme, readmeIssues, err := inspectRequiredFile(filepath.Join(root, readmeFileName), readmeFileName, true)
	if err != nil {
		return Descriptor{}, fmt.Errorf("check readme: %w", err)
	}

	hookStatus, hookIssues, err := InspectHookManifest(root)
	if err != nil {
		return Descriptor{}, fmt.Errorf("inspect hooks: %w", err)
	}

	hasMCPConfig, mcpIssues, err := inspectRequiredFile(filepath.Join(root, MCPConfigRelativePath), MCPConfigRelativePath, false)
	if err != nil {
		return Descriptor{}, fmt.Errorf("check mcp config: %w", err)
	}

	issues := make([]ValidationIssue, 0, len(commandIssues)+len(agentIssues)+len(skillIssues)+len(readmeIssues)+len(hookIssues)+len(mcpIssues))
	issues = append(issues, commandIssues...)
	issues = append(issues, agentIssues...)
	issues = append(issues, skillIssues...)
	issues = append(issues, readmeIssues...)
	issues = append(issues, hookIssues...)
	issues = append(issues, mcpIssues...)

	return Descriptor{
		Root:             root,
		Manifest:         manifest,
		Commands:         commands,
		Agents:           agents,
		Skills:           skills,
		HookEvents:       hookStatus.Events,
		HookCount:        hookStatus.CommandCount,
		HasREADME:        hasReadme,
		HasMCPConfig:     hasMCPConfig,
		ValidationIssues: issues,
	}, nil
}

func (d Descriptor) StatusPayload(pluginID string) contracts.PluginStatusPayload {
	issues := ValidateDescriptor(d)
	status := contracts.PluginStatusPayload{
		PluginID:   pluginID,
		Name:       d.Manifest.Name,
		Version:    d.Manifest.Version,
		Loaded:     len(issues) == 0,
		Valid:      len(issues) == 0,
		Commands:   append([]string(nil), d.Commands...),
		Agents:     append([]string(nil), d.Agents...),
		Skills:     append([]string(nil), d.Skills...),
		HookEvents: append([]string(nil), d.HookEvents...),
		HookCount:  d.HookCount,
		MCPServers: 0,
	}
	if status.PluginID == "" {
		status.PluginID = d.Manifest.Name
	}
	if d.HasMCPConfig {
		status.MCPServers = 1
	}
	if len(issues) > 0 {
		status.Error = summarizeIssues(issues)
	}
	return status
}

func ValidateDescriptor(descriptor Descriptor) []ValidationIssue {
	issues := make([]ValidationIssue, 0, len(descriptor.ValidationIssues)+5)
	issues = append(issues, ValidateManifest(descriptor.Manifest)...)
	issues = append(issues, descriptor.ValidationIssues...)
	return issues
}

func ValidateManifest(manifest Manifest) []ValidationIssue {
	issues := make([]ValidationIssue, 0, 5)

	name := strings.TrimSpace(manifest.Name)
	description := strings.TrimSpace(manifest.Description)
	version := strings.TrimSpace(manifest.Version)
	authorName := strings.TrimSpace(manifest.Author.Name)
	authorEmail := strings.TrimSpace(manifest.Author.Email)

	if name == "" {
		issues = append(issues, ValidationIssue{
			Field:   "name",
			Message: "name is required",
		})
	} else if !pluginNamePattern.MatchString(name) {
		issues = append(issues, ValidationIssue{
			Field:   "name",
			Message: "name must be lowercase letters, numbers, and hyphens only",
		})
	}

	if description == "" {
		issues = append(issues, ValidationIssue{
			Field:   "description",
			Message: "description is required",
		})
	}

	if version == "" {
		issues = append(issues, ValidationIssue{
			Field:   "version",
			Message: "version is required",
		})
	}

	if authorName == "" {
		issues = append(issues, ValidationIssue{
			Field:   "author.name",
			Message: "author name is required",
		})
	}

	if authorEmail != "" && !strings.Contains(authorEmail, "@") {
		issues = append(issues, ValidationIssue{
			Field:   "author.email",
			Message: "author email must contain @ when provided",
		})
	}

	for index, keyword := range manifest.Keywords {
		if strings.TrimSpace(keyword) == "" {
			issues = append(issues, ValidationIssue{
				Field:   fmt.Sprintf("keywords[%d]", index),
				Message: "keywords must not be blank",
			})
		}
	}

	return issues
}

func listMarkdownBasenames(dir string, field string) ([]string, []ValidationIssue, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		if isNotDir(err) {
			return nil, []ValidationIssue{{
				Field:   field,
				Message: fmt.Sprintf("%s must be a directory", field),
			}}, nil
		}
		return nil, nil, err
	}

	names := make([]string, 0, len(entries))
	issues := make([]ValidationIssue, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			issues = append(issues, ValidationIssue{
				Field:   filepath.ToSlash(filepath.Join(field, entry.Name())),
				Message: "nested directories are not supported here",
			})
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".md" {
			issues = append(issues, ValidationIssue{
				Field:   filepath.ToSlash(filepath.Join(field, name)),
				Message: "expected a markdown file",
			})
			continue
		}
		names = append(names, strings.TrimSuffix(name, filepath.Ext(name)))
	}
	sort.Strings(names)
	return names, issues, nil
}

func listSkills(root string) ([]string, []ValidationIssue, error) {
	skills := make([]string, 0)
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		if isNotDir(err) {
			return nil, []ValidationIssue{{
				Field:   skillsDirName,
				Message: "skills must be a directory",
			}}, nil
		}
		return nil, nil, err
	}
	if !info.IsDir() {
		return nil, []ValidationIssue{{
			Field:   skillsDirName,
			Message: "skills must be a directory",
		}}, nil
	}

	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Name() != skillFileName {
			return nil
		}
		relativeDir, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		skills = append(skills, filepath.ToSlash(relativeDir))
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	sort.Strings(skills)
	return skills, nil, nil
}

func inspectRequiredFile(path string, field string, required bool) (bool, []ValidationIssue, error) {
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return false, []ValidationIssue{{
				Field:   field,
				Message: fmt.Sprintf("%s must be a file", field),
			}}, nil
		}
		return true, nil, nil
	}
	if os.IsNotExist(err) {
		if !required {
			return false, nil, nil
		}
		return false, []ValidationIssue{{
			Field:   field,
			Message: fmt.Sprintf("%s is required", field),
		}}, nil
	}
	return false, nil, err
}

func isNotDir(err error) bool {
	pathErr, ok := err.(*os.PathError)
	if !ok {
		return false
	}
	return pathErr.Err == syscall.ENOTDIR
}

func summarizeIssues(issues []ValidationIssue) string {
	parts := make([]string, 0, len(issues))
	for _, issue := range issues {
		parts = append(parts, fmt.Sprintf("%s: %s", issue.Field, issue.Message))
	}
	return strings.Join(parts, "; ")
}
