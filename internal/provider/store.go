package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

var ErrProfileNotFound = errors.New("profile not found")

type ProfileStore interface {
	ListProfiles() ([]contracts.AuthProfile, error)
	GetProfile(id string) (contracts.AuthProfile, error)
	ResolveProfile(profileID string, model string) (contracts.AuthProfile, error)
	SaveProfile(profile contracts.AuthProfile) error
	SetDefaultProfile(id string) error
}

type profileFile struct {
	Profiles         []contracts.AuthProfile `json:"profiles"`
	DefaultProfileID string                  `json:"default_profile_id"`
}

type memoryProfileStore struct {
	mu   sync.RWMutex
	data profileFile
}

type fileProfileStore struct {
	mu   sync.Mutex
	root string
}

func NewMemoryProfileStore() ProfileStore {
	return &memoryProfileStore{
		data: seededProfiles(),
	}
}

func NewFileProfileStore(root string) (ProfileStore, error) {
	store := &fileProfileStore{root: root}
	if err := store.ensureLayout(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *memoryProfileStore) ListProfiles() ([]contracts.AuthProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneProfiles(s.data.Profiles), nil
}

func (s *memoryProfileStore) GetProfile(id string) (contracts.AuthProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return findProfileByID(s.data.Profiles, id)
}

func (s *memoryProfileStore) ResolveProfile(profileID string, model string) (contracts.AuthProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return resolveProfileData(s.data, profileID, model)
}

func (s *memoryProfileStore) SaveProfile(profile contracts.AuthProfile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := upsertProfileData(s.data, profile)
	if err != nil {
		return err
	}
	s.data = data
	return nil
}

func (s *memoryProfileStore) SetDefaultProfile(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := findProfileByID(s.data.Profiles, id); err != nil {
		return err
	}
	s.data.DefaultProfileID = id
	return nil
}

func (s *fileProfileStore) ListProfiles() ([]contracts.AuthProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readProfilesLocked()
	if err != nil {
		return nil, err
	}
	return cloneProfiles(data.Profiles), nil
}

func (s *fileProfileStore) GetProfile(id string) (contracts.AuthProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readProfilesLocked()
	if err != nil {
		return contracts.AuthProfile{}, err
	}
	return findProfileByID(data.Profiles, id)
}

func (s *fileProfileStore) ResolveProfile(profileID string, model string) (contracts.AuthProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readProfilesLocked()
	if err != nil {
		return contracts.AuthProfile{}, err
	}
	return resolveProfileData(data, profileID, model)
}

func (s *fileProfileStore) SaveProfile(profile contracts.AuthProfile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readProfilesLocked()
	if err != nil {
		return err
	}
	data, err = upsertProfileData(data, profile)
	if err != nil {
		return err
	}
	return s.writeProfilesLocked(data)
}

func (s *fileProfileStore) SetDefaultProfile(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readProfilesLocked()
	if err != nil {
		return err
	}
	if _, err := findProfileByID(data.Profiles, id); err != nil {
		return err
	}
	data.DefaultProfileID = id
	return s.writeProfilesLocked(data)
}

func (s *fileProfileStore) ensureLayout() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ensureLayoutLocked()
}

func (s *fileProfileStore) ensureLayoutLocked() error {
	if err := os.MkdirAll(filepath.Join(s.root, "profiles"), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(s.profilesPath()); errors.Is(err, os.ErrNotExist) {
		return s.writeProfilesLocked(seededProfiles())
	} else if err != nil {
		return err
	}
	return nil
}

func (s *fileProfileStore) readProfilesLocked() (profileFile, error) {
	if err := s.ensureLayoutLocked(); err != nil {
		return profileFile{}, err
	}

	data, err := os.ReadFile(s.profilesPath())
	if err != nil {
		return profileFile{}, err
	}

	var file profileFile
	if err := json.Unmarshal(data, &file); err != nil {
		return profileFile{}, err
	}
	if len(file.Profiles) == 0 {
		file = seededProfiles()
		if err := s.writeProfilesLocked(file); err != nil {
			return profileFile{}, err
		}
	}
	return file, nil
}

func (s *fileProfileStore) writeProfilesLocked(file profileFile) error {
	encoded, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return os.WriteFile(s.profilesPath(), encoded, 0o644)
}

func (s *fileProfileStore) profilesPath() string {
	return filepath.Join(s.root, "profiles", "profiles.json")
}

func seededProfiles() profileFile {
	return profileFile{
		Profiles: []contracts.AuthProfile{
			{
				ID:           "anthropic-main",
				Kind:         contracts.AuthProfileAnthropicOAuth,
				Provider:     contracts.ProviderAnthropic,
				DisplayName:  "Anthropic Main",
				DefaultModel: "claude-sonnet-4-6",
				Settings: map[string]string{
					"credential_ref": "keychain://anthropic-main",
					"account_scope":  "claude",
					"oauth_host":     "https://claude.ai",
				},
			},
			{
				ID:           "openrouter-main",
				Kind:         contracts.AuthProfileOpenRouterAPIKey,
				Provider:     contracts.ProviderOpenRouter,
				DisplayName:  "OpenRouter Main",
				DefaultModel: "anthropic/claude-sonnet-4.5",
				Settings: map[string]string{
					"credential_ref": "keychain://openrouter-main",
					"api_base":       "https://openrouter.ai/api/v1",
					"app_name":       "Klaude Kode",
					"http_referer":   "https://local.cli",
				},
			},
		},
		DefaultProfileID: "anthropic-main",
	}
}

func resolveProfileData(data profileFile, profileID string, model string) (contracts.AuthProfile, error) {
	if profileID != "" && !IsLegacyProfileID(profileID) {
		return findProfileByID(data.Profiles, profileID)
	}

	if !hasExplicitProviderHint(profileID, model) && data.DefaultProfileID != "" {
		profile, err := findProfileByID(data.Profiles, data.DefaultProfileID)
		if err == nil {
			return profile, nil
		}
	}

	providerKind := inferProviderKind(profileID, model)
	if profile, ok := profileForProvider(data, providerKind); ok {
		return profile, nil
	}

	if data.DefaultProfileID != "" {
		profile, err := findProfileByID(data.Profiles, data.DefaultProfileID)
		if err == nil {
			return profile, nil
		}
	}
	if len(data.Profiles) > 0 {
		return cloneProfile(data.Profiles[0]), nil
	}
	return contracts.AuthProfile{}, fmt.Errorf("no auth profiles are configured")
}

func profileForProvider(data profileFile, kind contracts.ProviderKind) (contracts.AuthProfile, bool) {
	if data.DefaultProfileID != "" {
		profile, err := findProfileByID(data.Profiles, data.DefaultProfileID)
		if err == nil && profile.Provider == kind {
			return profile, true
		}
	}

	for _, profile := range data.Profiles {
		if profile.Provider == kind {
			return cloneProfile(profile), true
		}
	}
	return contracts.AuthProfile{}, false
}

func findProfileByID(profiles []contracts.AuthProfile, id string) (contracts.AuthProfile, error) {
	for _, profile := range profiles {
		if profile.ID == id {
			return cloneProfile(profile), nil
		}
	}
	return contracts.AuthProfile{}, fmt.Errorf("%w: %s", ErrProfileNotFound, id)
}

func cloneProfiles(profiles []contracts.AuthProfile) []contracts.AuthProfile {
	cloned := make([]contracts.AuthProfile, 0, len(profiles))
	for _, profile := range profiles {
		cloned = append(cloned, cloneProfile(profile))
	}
	return cloned
}

func cloneProfile(profile contracts.AuthProfile) contracts.AuthProfile {
	cloned := profile
	if profile.Settings != nil {
		cloned.Settings = make(map[string]string, len(profile.Settings))
		for key, value := range profile.Settings {
			cloned.Settings[key] = value
		}
	}
	return cloned
}

func hasExplicitProviderHint(profileID string, model string) bool {
	lowerProfileID := strings.ToLower(strings.TrimSpace(profileID))
	lowerModel := strings.ToLower(strings.TrimSpace(model))
	if lowerModel != "" {
		return true
	}
	return strings.Contains(lowerProfileID, "openrouter") || strings.Contains(lowerProfileID, "anthropic")
}

func upsertProfileData(data profileFile, profile contracts.AuthProfile) (profileFile, error) {
	if profile.ID == "" {
		return profileFile{}, fmt.Errorf("profile id is required")
	}
	if profile.Provider == "" {
		return profileFile{}, fmt.Errorf("profile provider is required")
	}
	if profile.Kind == "" {
		return profileFile{}, fmt.Errorf("profile kind is required")
	}

	next := profileFile{
		Profiles:         cloneProfiles(data.Profiles),
		DefaultProfileID: data.DefaultProfileID,
	}
	replaced := false
	for index, existing := range next.Profiles {
		if existing.ID == profile.ID {
			next.Profiles[index] = cloneProfile(profile)
			replaced = true
			break
		}
	}
	if !replaced {
		next.Profiles = append(next.Profiles, cloneProfile(profile))
	}
	return next, nil
}
