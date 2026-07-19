package audit_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/aegis-platform/aegis/internal/audit"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// MockRepository implements audit.Repository
type MockRepository struct {
	entries []*audit.Entry
	mu      sync.RWMutex
}

func (m *MockRepository) Insert(entry *audit.Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry.ID = int64(len(m.entries) + 1)
	m.entries = append(m.entries, entry)
	return nil
}

func (m *MockRepository) GetByID(id int64) (*audit.Entry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if id < 1 || id > int64(len(m.entries)) {
		return nil, errors.New("not found")
	}
	return m.entries[id-1], nil
}

func (m *MockRepository) GetRange(startID, endID int64) ([]*audit.Entry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*audit.Entry
	for _, e := range m.entries {
		if e.ID >= startID && e.ID <= endID {
			result = append(result, e)
		}
	}
	return result, nil
}

func (m *MockRepository) GetLatest() (*audit.Entry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.entries) == 0 {
		return nil, nil
	}
	return m.entries[len(m.entries)-1], nil
}

func (m *MockRepository) Query(filter audit.AuditFilter, cursor string, limit int) ([]*audit.Entry, string, error) {
	return nil, "", nil
}

// MockSigner implements audit.Signer
type MockSigner struct{}

func (m *MockSigner) Sign(data []byte) ([]byte, error) {
	return []byte("mock-signature"), nil
}

func (m *MockSigner) Verify(data, signature []byte) (bool, error) {
	return string(signature) == "mock-signature", nil
}

func TestAppendAndRetrieve(t *testing.T) {
	repo := &MockRepository{}
	signer := &MockSigner{}
	logger := zap.NewNop()
	
	svc, err := audit.NewService(repo, signer, logger)
	assert.NoError(t, err)
	
	entry := &audit.Entry{
		EventType: "TEST_EVENT",
		ActorType: audit.ActorTypeUser,
		Action:    "CREATE",
	}
	
	err = svc.Append(context.Background(), entry)
	assert.NoError(t, err)
	
	latest, err := repo.GetLatest()
	assert.NoError(t, err)
	assert.Equal(t, "TEST_EVENT", latest.EventType)
	assert.NotNil(t, latest.EntryHash)
}

func TestHashChaining(t *testing.T) {
	repo := &MockRepository{}
	signer := &MockSigner{}
	logger := zap.NewNop()
	
	svc, err := audit.NewService(repo, signer, logger)
	assert.NoError(t, err)
	
	e1 := &audit.Entry{EventType: "E1"}
	e2 := &audit.Entry{EventType: "E2"}
	
	_ = svc.Append(context.Background(), e1)
	_ = svc.Append(context.Background(), e2)
	
	assert.Equal(t, e1.EntryHash, e2.PreviousHash)
}

func TestVerifyChain(t *testing.T) {
	repo := &MockRepository{}
	signer := &MockSigner{}
	logger := zap.NewNop()
	
	svc, err := audit.NewService(repo, signer, logger)
	assert.NoError(t, err)
	
	for i := 0; i < 5; i++ {
		_ = svc.Append(context.Background(), &audit.Entry{EventType: "TEST"})
	}
	
	report, err := svc.VerifyChain(context.Background(), 1, 5)
	assert.NoError(t, err)
	assert.True(t, report.Verified)
	assert.Equal(t, int64(5), report.EntriesChecked)
}

func TestVerifyChainTampered(t *testing.T) {
	repo := &MockRepository{}
	signer := &MockSigner{}
	logger := zap.NewNop()
	
	svc, err := audit.NewService(repo, signer, logger)
	assert.NoError(t, err)
	
	for i := 0; i < 5; i++ {
		_ = svc.Append(context.Background(), &audit.Entry{EventType: "TEST"})
	}
	
	// Tamper with entry 3
	repo.mu.Lock()
	repo.entries[2].Action = "TAMPERED"
	repo.mu.Unlock()
	
	report, err := svc.VerifyChain(context.Background(), 1, 5)
	assert.NoError(t, err)
	assert.False(t, report.Verified)
	assert.Equal(t, int64(3), *report.FirstBrokenAt)
}

func TestCheckpoint(t *testing.T) {
	repo := &MockRepository{}
	signer := &MockSigner{}
	logger := zap.NewNop()
	
	svc, err := audit.NewService(repo, signer, logger)
	assert.NoError(t, err)
	
	_ = svc.Append(context.Background(), &audit.Entry{EventType: "TEST"})
	
	err = svc.Checkpoint(context.Background())
	assert.NoError(t, err)
	
	latest, _ := repo.GetLatest()
	assert.Equal(t, "SYSTEM_CHECKPOINT", latest.EventType)
	assert.Equal(t, []byte("mock-signature"), latest.CheckpointSig)
}

func TestConcurrentAppends(t *testing.T) {
	repo := &MockRepository{}
	signer := &MockSigner{}
	logger := zap.NewNop()
	
	svc, err := audit.NewService(repo, signer, logger)
	assert.NoError(t, err)
	
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = svc.Append(context.Background(), &audit.Entry{EventType: "CONCURRENT_TEST"})
		}(i)
	}
	wg.Wait()
	
	report, err := svc.VerifyChain(context.Background(), 1, 100)
	assert.NoError(t, err)
	assert.True(t, report.Verified)
}
