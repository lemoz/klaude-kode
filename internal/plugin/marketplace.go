package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const MarketplaceRelativePath = ".claude-plugin/marketplace.json"

type MarketplaceManifest struct {
	Schema      string              `json:"$schema,omitempty"`
	Name        string              `json:"name"`
	Version     string              `json:"version"`
	Description string              `json:"description"`
	Owner       Author              `json:"owner"`
	Plugins     []MarketplacePlugin `json:"plugins,omitempty"`
}

type MarketplacePlugin struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version,omitempty"`
	Author      Author `json:"author,omitempty"`
	Source      string `json:"source"`
	Category    string `json:"category"`
}

type MarketplaceDescriptor struct {
	Root             string              `json:"root"`
	Manifest         MarketplaceManifest `json:"manifest"`
	ValidationIssues []ValidationIssue   `json:"validation_issues,omitempty"`
}

type MarketplaceStatus struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	PluginCount int      `json:"plugin_count"`
	Categories  []string `json:"categories,omitempty"`
	Valid       bool     `json:"valid"`
	Error       string   `json:"error,omitempty"`
}

func LoadMarketplaceManifest(root string) (MarketplaceManifest, error) {
	path := filepath.Join(root, MarketplaceRelativePath)
	data, err := os.ReadFile(path)
	if err != nil {
		return MarketplaceManifest{}, err
	}
	manifest, err := ParseMarketplaceManifest(data)
	if err != nil {
		return MarketplaceManifest{}, fmt.Errorf("parse marketplace manifest: %w", err)
	}
	return manifest, nil
}

func ParseMarketplaceManifest(data []byte) (MarketplaceManifest, error) {
	var manifest MarketplaceManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return MarketplaceManifest{}, err
	}
	return manifest, nil
}

func InspectMarketplace(root string) (MarketplaceDescriptor, error) {
	manifest, err := LoadMarketplaceManifest(root)
	if err != nil {
		return MarketplaceDescriptor{}, err
	}
	return MarketplaceDescriptor{
		Root:             root,
		Manifest:         manifest,
		ValidationIssues: validateMarketplaceSources(root, manifest),
	}, nil
}

func (d MarketplaceDescriptor) Status() MarketplaceStatus {
	issues := ValidateMarketplaceDescriptor(d)
	status := MarketplaceStatus{
		Name:        d.Manifest.Name,
		Version:     d.Manifest.Version,
		PluginCount: len(d.Manifest.Plugins),
		Categories:  marketplaceCategories(d.Manifest),
		Valid:       len(issues) == 0,
	}
	if len(issues) > 0 {
		status.Error = summarizeIssues(issues)
	}
	return status
}

func ValidateMarketplaceDescriptor(descriptor MarketplaceDescriptor) []ValidationIssue {
	issues := make([]ValidationIssue, 0, len(descriptor.ValidationIssues)+8)
	issues = append(issues, ValidateMarketplaceManifest(descriptor.Manifest)...)
	issues = append(issues, descriptor.ValidationIssues...)
	return issues
}

func ValidateMarketplaceManifest(manifest MarketplaceManifest) []ValidationIssue {
	issues := make([]ValidationIssue, 0, 8)

	if strings.TrimSpace(manifest.Name) == "" {
		issues = append(issues, ValidationIssue{
			Field:   "name",
			Message: "name is required",
		})
	}
	if strings.TrimSpace(manifest.Version) == "" {
		issues = append(issues, ValidationIssue{
			Field:   "version",
			Message: "version is required",
		})
	}
	if strings.TrimSpace(manifest.Description) == "" {
		issues = append(issues, ValidationIssue{
			Field:   "description",
			Message: "description is required",
		})
	}
	if strings.TrimSpace(manifest.Owner.Name) == "" {
		issues = append(issues, ValidationIssue{
			Field:   "owner.name",
			Message: "owner name is required",
		})
	}
	if manifest.Owner.Email != "" && !strings.Contains(manifest.Owner.Email, "@") {
		issues = append(issues, ValidationIssue{
			Field:   "owner.email",
			Message: "owner email must contain @ when provided",
		})
	}
	if len(manifest.Plugins) == 0 {
		issues = append(issues, ValidationIssue{
			Field:   "plugins",
			Message: "at least one plugin is required",
		})
		return issues
	}

	seenNames := make(map[string]struct{}, len(manifest.Plugins))
	for index, entry := range manifest.Plugins {
		prefix := fmt.Sprintf("plugins[%d]", index)
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			issues = append(issues, ValidationIssue{
				Field:   prefix + ".name",
				Message: "name is required",
			})
		} else {
			if _, exists := seenNames[name]; exists {
				issues = append(issues, ValidationIssue{
					Field:   prefix + ".name",
					Message: "plugin names must be unique",
				})
			}
			seenNames[name] = struct{}{}
		}
		if strings.TrimSpace(entry.Description) == "" {
			issues = append(issues, ValidationIssue{
				Field:   prefix + ".description",
				Message: "description is required",
			})
		}
		if strings.TrimSpace(entry.Source) == "" {
			issues = append(issues, ValidationIssue{
				Field:   prefix + ".source",
				Message: "source is required",
			})
		}
		if strings.TrimSpace(entry.Category) == "" {
			issues = append(issues, ValidationIssue{
				Field:   prefix + ".category",
				Message: "category is required",
			})
		}
		if entry.Author.Email != "" && !strings.Contains(entry.Author.Email, "@") {
			issues = append(issues, ValidationIssue{
				Field:   prefix + ".author.email",
				Message: "author email must contain @ when provided",
			})
		}
	}

	return issues
}

func validateMarketplaceSources(root string, manifest MarketplaceManifest) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	for index, entry := range manifest.Plugins {
		field := fmt.Sprintf("plugins[%d].source", index)
		source := strings.TrimSpace(entry.Source)
		if source == "" {
			continue
		}
		if filepath.IsAbs(source) {
			issues = append(issues, ValidationIssue{
				Field:   field,
				Message: "source must be relative to the marketplace root",
			})
			continue
		}
		cleanSource := filepath.Clean(source)
		if cleanSource == "." || cleanSource == ".." || strings.HasPrefix(cleanSource, ".."+string(filepath.Separator)) {
			issues = append(issues, ValidationIssue{
				Field:   field,
				Message: "source must stay within the marketplace root",
			})
			continue
		}
		info, err := os.Stat(filepath.Join(root, cleanSource))
		if err != nil {
			if os.IsNotExist(err) {
				issues = append(issues, ValidationIssue{
					Field:   field,
					Message: "source path does not exist",
				})
				continue
			}
			issues = append(issues, ValidationIssue{
				Field:   field,
				Message: err.Error(),
			})
			continue
		}
		if !info.IsDir() {
			issues = append(issues, ValidationIssue{
				Field:   field,
				Message: "source must point to a plugin directory",
			})
		}
	}
	return issues
}

func marketplaceCategories(manifest MarketplaceManifest) []string {
	seen := make(map[string]struct{}, len(manifest.Plugins))
	categories := make([]string, 0, len(manifest.Plugins))
	for _, entry := range manifest.Plugins {
		category := strings.TrimSpace(entry.Category)
		if category == "" {
			continue
		}
		if _, exists := seen[category]; exists {
			continue
		}
		seen[category] = struct{}{}
		categories = append(categories, category)
	}
	sort.Strings(categories)
	return categories
}
