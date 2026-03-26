package store

import (
	"encoding/json"
	"os"
	"sync"
)

// SeenStore persists review IDs already sent to Slack so we don't resend them.
type SeenStore struct {
	mu   sync.RWMutex
	path string
	ids  map[string]struct{}
}

func New(path string) (*SeenStore, error) {
	s := &SeenStore{path: path, ids: make(map[string]struct{})}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *SeenStore) IsSeen(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.ids[id]
	return ok
}

func (s *SeenStore) MarkSeen(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ids[id] = struct{}{}
	return s.save()
}

func (s *SeenStore) load() error {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		return err
	}
	for _, id := range ids {
		s.ids[id] = struct{}{}
	}
	return nil
}

func (s *SeenStore) save() error {
	ids := make([]string, 0, len(s.ids))
	for id := range s.ids {
		ids = append(ids, id)
	}
	data, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}
