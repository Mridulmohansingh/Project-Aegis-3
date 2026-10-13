package audit

import (
	"errors"
	"sync"
)

// MemoryRepository provides a concurrent-safe in-memory audit log repository.
type MemoryRepository struct {
	mu      sync.RWMutex
	entries []*Entry
}

// NewMemoryRepository creates a new in-memory audit log repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		entries: make([]*Entry, 0),
	}
}

func (m *MemoryRepository) Insert(entry *Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry.ID = int64(len(m.entries) + 1)
	m.entries = append(m.entries, entry)
	return nil
}

func (m *MemoryRepository) GetByID(id int64) (*Entry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if id < 1 || id > int64(len(m.entries)) {
		return nil, errors.New("audit entry not found")
	}
	return m.entries[id-1], nil
}

func (m *MemoryRepository) GetRange(startID, endID int64) ([]*Entry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Entry
	for _, e := range m.entries {
		if e.ID >= startID && e.ID <= endID {
			result = append(result, e)
		}
	}
	return result, nil
}

func (m *MemoryRepository) GetLatest() (*Entry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.entries) == 0 {
		return nil, nil
	}
	return m.entries[len(m.entries)-1], nil
}

func (m *MemoryRepository) Query(filter AuditFilter, cursor string, limit int) ([]*Entry, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Entry
	for _, e := range m.entries {
		if filter.ResourceType != "" && e.ResourceType != filter.ResourceType {
			continue
		}
		if filter.Action != "" && e.Action != filter.Action {
			continue
		}
		result = append(result, e)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, "", nil
}
