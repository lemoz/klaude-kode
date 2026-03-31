package engine

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

var errSessionNotFound = errors.New("session not found")

type sessionStore interface {
	SessionExists(sessionID string) (bool, error)
	AppendEvent(event contracts.SessionEvent) error
	LoadEvents(sessionID string) ([]contracts.SessionEvent, error)
	UpsertSummary(summary contracts.SessionSummary) error
	LoadSummary(sessionID string) (contracts.SessionSummary, error)
	ListSummaries() ([]contracts.SessionSummary, error)
}

type fileSessionStore struct {
	mu   sync.Mutex
	root string
}

type sessionIndexFile struct {
	Sessions []contracts.SessionSummary `json:"sessions"`
}

func newFileSessionStore(root string) (*fileSessionStore, error) {
	store := &fileSessionStore{root: root}
	if err := store.ensureLayout(); err != nil {
		return nil, err
	}
	return store, nil
}

func DefaultStateRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".claude-next"
	}
	return filepath.Join(home, ".claude-next")
}

func (s *fileSessionStore) SessionExists(sessionID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.readIndexLocked()
	if err != nil {
		return false, err
	}
	for _, summary := range index.Sessions {
		if summary.SessionID == sessionID {
			return true, nil
		}
	}

	eventsPath := s.eventsPath(sessionID)
	_, err = os.Stat(eventsPath)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func (s *fileSessionStore) AppendEvent(event contracts.SessionEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureSessionLayoutLocked(event.SessionID); err != nil {
		return err
	}

	file, err := os.OpenFile(s.eventsPath(event.SessionID), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

func (s *fileSessionStore) LoadEvents(sessionID string) ([]contracts.SessionEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.Open(s.eventsPath(sessionID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errSessionNotFound
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1024*1024)

	var events []contracts.SessionEvent
	for scanner.Scan() {
		var event contracts.SessionEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, errSessionNotFound
	}
	return events, nil
}

func (s *fileSessionStore) UpsertSummary(summary contracts.SessionSummary) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.readIndexLocked()
	if err != nil {
		return err
	}

	found := false
	for i, existing := range index.Sessions {
		if existing.SessionID == summary.SessionID {
			index.Sessions[i] = summary
			found = true
			break
		}
	}
	if !found {
		index.Sessions = append(index.Sessions, summary)
	}

	sort.Slice(index.Sessions, func(i, j int) bool {
		if index.Sessions[i].UpdatedAt.Equal(index.Sessions[j].UpdatedAt) {
			return index.Sessions[i].SessionID < index.Sessions[j].SessionID
		}
		return index.Sessions[i].UpdatedAt.After(index.Sessions[j].UpdatedAt)
	})

	return s.writeIndexLocked(index)
}

func (s *fileSessionStore) LoadSummary(sessionID string) (contracts.SessionSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.readIndexLocked()
	if err != nil {
		return contracts.SessionSummary{}, err
	}
	for _, summary := range index.Sessions {
		if summary.SessionID == sessionID {
			return summary, nil
		}
	}
	return contracts.SessionSummary{}, errSessionNotFound
}

func (s *fileSessionStore) ListSummaries() ([]contracts.SessionSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.readIndexLocked()
	if err != nil {
		return nil, err
	}
	return append([]contracts.SessionSummary(nil), index.Sessions...), nil
}

func (s *fileSessionStore) ensureLayout() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ensureLayoutLocked()
}

func (s *fileSessionStore) ensureLayoutLocked() error {
	if err := os.MkdirAll(filepath.Join(s.root, "state"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(s.root, "sessions"), 0o755); err != nil {
		return err
	}
	indexPath := s.indexPath()
	if _, err := os.Stat(indexPath); errors.Is(err, os.ErrNotExist) {
		return s.writeIndexLocked(sessionIndexFile{})
	} else if err != nil {
		return err
	}
	return nil
}

func (s *fileSessionStore) ensureSessionLayoutLocked(sessionID string) error {
	if err := s.ensureLayoutLocked(); err != nil {
		return err
	}
	if err := os.MkdirAll(s.sessionDir(sessionID), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(s.sessionDir(sessionID), "artifacts"), 0o755); err != nil {
		return err
	}
	return nil
}

func (s *fileSessionStore) readIndexLocked() (sessionIndexFile, error) {
	if err := s.ensureLayoutLocked(); err != nil {
		return sessionIndexFile{}, err
	}

	data, err := os.ReadFile(s.indexPath())
	if err != nil {
		return sessionIndexFile{}, err
	}
	if len(data) == 0 {
		return sessionIndexFile{}, nil
	}

	var index sessionIndexFile
	if err := json.Unmarshal(data, &index); err != nil {
		return sessionIndexFile{}, fmt.Errorf("read session index: %w", err)
	}
	return index, nil
}

func (s *fileSessionStore) writeIndexLocked(index sessionIndexFile) error {
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(s.indexPath()), "session-index-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmpFile.Write(append(data, '\n')); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.indexPath())
}

func (s *fileSessionStore) sessionDir(sessionID string) string {
	return filepath.Join(s.root, "sessions", sessionID)
}

func (s *fileSessionStore) eventsPath(sessionID string) string {
	return filepath.Join(s.sessionDir(sessionID), "events.jsonl")
}

func (s *fileSessionStore) indexPath() string {
	return filepath.Join(s.root, "state", "session-index.json")
}
