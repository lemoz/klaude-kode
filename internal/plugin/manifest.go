package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

const ManifestRelativePath = ".claude-plugin/plugin.json"
const MCPConfigRelativePath = ".mcp.json"

const (
	commandsDirName = "commands"
	agentsDirName   = "agents"
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
	Root         string   `json:"root"`
	Manifest     Manifest `json:"manifest"`
	Commands     []string `json:"commands,omitempty"`
	Agents       []string `json:"agents,omitempty"`
	Skills       []string `json:"skills,omitempty"`
	HasMCPConfig bool     `json:"has_mcp_config"`
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

	commands, err := listMarkdownBasenames(filepath.Join(root, commandsDirName))
	if err != nil {
		return Descriptor{}, fmt.Errorf("list commands: %w", err)
	}

	agents, err := listMarkdownBasenames(filepath.Join(root, agentsDirName))
	if err != nil {
		return Descriptor{}, fmt.Errorf("list agents: %w", err)
	}

	skills, err := listSkills(filepath.Join(root, skillsDirName))
	if err != nil {
		return Descriptor{}, fmt.Errorf("list skills: %w", err)
	}

	hasMCPConfig, err := fileExists(filepath.Join(root, MCPConfigRelativePath))
	if err != nil {
		return Descriptor{}, fmt.Errorf("check mcp config: %w", err)
	}

	return Descriptor{
		Root:         root,
		Manifest:     manifest,
		Commands:     commands,
		Agents:       agents,
		Skills:       skills,
		HasMCPConfig: hasMCPConfig,
	}, nil
}

func (d Descriptor) StatusPayload(pluginID string) contracts.PluginStatusPayload {
	issues := ValidateManifest(d.Manifest)
	status := contracts.PluginStatusPayload{
		PluginID:   pluginID,
		Name:       d.Manifest.Name,
		Version:    d.Manifest.Version,
		Loaded:     len(issues) == 0,
		Valid:      len(issues) == 0,
		Commands:   append([]string(nil), d.Commands...),
		Agents:     append([]string(nil), d.Agents...),
		Skills:     append([]string(nil), d.Skills...),
		HookCount:  0,
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

func listMarkdownBasenames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".md" {
			continue
		}
		names = append(names, strings.TrimSuffix(name, filepath.Ext(name)))
	}
	sort.Strings(names)
	return names, nil
}

func listSkills(root string) ([]string, error) {
	skills := make([]string, 0)
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
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
		return nil, err
	}

	sort.Strings(skills)
	return skills, nil
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func summarizeIssues(issues []ValidationIssue) string {
	parts := make([]string, 0, len(issues))
	for _, issue := range issues {
		parts = append(parts, fmt.Sprintf("%s: %s", issue.Field, issue.Message))
	}
	return strings.Join(parts, "; ")
}
