package mcp

import (
	"fmt"
	"sort"
	"sync"
)

type MemoryArtifactStore struct {
	mu    sync.RWMutex
	items map[string]string
}

func NewMemoryArtifactStore() *MemoryArtifactStore {
	return &MemoryArtifactStore{
		items: map[string]string{},
	}
}

func (s *MemoryArtifactStore) Upsert(id string, payload string) error {
	if s == nil {
		return fmt.Errorf("artifact store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[id] = payload
	return nil
}

func (s *MemoryArtifactStore) Get(id string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("artifact store is nil")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	payload, ok := s.items[id]
	if !ok {
		return "", fmt.Errorf("artifact not found: %s", id)
	}
	return payload, nil
}

func (s *MemoryArtifactStore) Delete(id string) error {
	if s == nil {
		return fmt.Errorf("artifact store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[id]; !ok {
		return fmt.Errorf("artifact not found: %s", id)
	}
	delete(s.items, id)
	return nil
}

func (s *MemoryArtifactStore) List() ([]string, error) {
	if s == nil {
		return nil, fmt.Errorf("artifact store is nil")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.items))
	for id := range s.items {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}
