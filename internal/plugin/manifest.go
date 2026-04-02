package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const ManifestRelativePath = ".claude-plugin/plugin.json"

var pluginNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type Author struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

type Manifest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Author      Author   `json:"author"`
	Homepage    string   `json:"homepage,omitempty"`
	Repository  string   `json:"repository,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
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

func ValidateManifest(manifest Manifest) []ValidationIssue {
	issues := make([]ValidationIssue, 0, 4)

	name := strings.TrimSpace(manifest.Name)
	description := strings.TrimSpace(manifest.Description)
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
