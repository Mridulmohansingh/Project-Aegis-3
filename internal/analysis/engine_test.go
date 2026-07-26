package analysis_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/aegis-platform/aegis/internal/analysis"
	"github.com/aegis-platform/aegis/internal/domain/exam"
	"github.com/aegis-platform/aegis/internal/domain/item"
)

// MockItemRepository implements item.ItemRepository
type MockItemRepository struct {
	mu    sync.RWMutex
	items map[uuid.UUID]*item.Item
}

func (m *MockItemRepository) Create(ctx context.Context, i *item.Item) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[i.ID] = i
	return nil
}

func (m *MockItemRepository) GetByID(ctx context.Context, orgID, id uuid.UUID) (*item.Item, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.items[id], nil
}

func (m *MockItemRepository) GetByExternalID(ctx context.Context, orgID uuid.UUID, externalID string) (*item.Item, error) {
	return nil, nil
}

func (m *MockItemRepository) Update(ctx context.Context, i *item.Item) error { return nil }
func (m *MockItemRepository) SoftDelete(ctx context.Context, orgID, id, deletedBy uuid.UUID) error {
	return nil
}

func (m *MockItemRepository) List(ctx context.Context, filter item.ItemFilter, cursor string, limit int) ([]*item.Item, string, error) {
	return nil, "", nil
}

func (m *MockItemRepository) GetEligibleForPaperGeneration(ctx context.Context, orgID, subjectID uuid.UUID, maxExposureIndex float64) ([]*item.Item, error) {
	return nil, nil
}

func (m *MockItemRepository) GetEnemies(ctx context.Context, itemID uuid.UUID) ([]item.ItemEnemy, error) {
	return nil, nil
}

func (m *MockItemRepository) CreateVersion(ctx context.Context, version *item.ItemVersion) error {
	return nil
}

func (m *MockItemRepository) GetVersionHistory(ctx context.Context, itemID uuid.UUID) ([]item.ItemVersion, error) {
	return nil, nil
}

// MockResponseRepository implements exam.ResponseRepository
type MockResponseRepository struct{}

func (m *MockResponseRepository) Upsert(ctx interface{}, response *exam.Response) error { return nil }
func (m *MockResponseRepository) UpsertBatch(ctx interface{}, responses []*exam.Response) error {
	return nil
}

func (m *MockResponseRepository) GetBySession(ctx interface{}, sessionID uuid.UUID) ([]*exam.Response, error) {
	return nil, nil
}

func (m *MockResponseRepository) GetByExamAndCandidate(ctx interface{}, examID, candidateID uuid.UUID) ([]*exam.Response, error) {
	return nil, nil
}

func (m *MockResponseRepository) GetByExamAndItem(ctx interface{}, examID, itemID uuid.UUID) ([]*exam.Response, error) {
	return nil, nil
}

func TestComputeClassicalStats(t *testing.T) {
	itemRepo := &MockItemRepository{items: make(map[uuid.UUID]*item.Item)}
	respRepo := &MockResponseRepository{}
	logger := zap.NewNop()

	engine := analysis.NewEngine(itemRepo, respRepo, logger)

	examID := uuid.New()
	orgID := uuid.New()

	itm1 := &item.Item{ID: uuid.New(), OrganizationID: orgID, ExternalID: "Q-01"}
	itm2 := &item.Item{ID: uuid.New(), OrganizationID: orgID, ExternalID: "Q-02"}
	itemsList := []*item.Item{itm1, itm2}

	cand1 := uuid.New()
	cand2 := uuid.New()

	opt1 := 1
	opt2 := 2

	responses := map[uuid.UUID][]*exam.Response{
		cand1: {
			{ItemID: itm1.ID, SelectedOption: &opt1, TimeSpentMs: 1000},
			{ItemID: itm2.ID, SelectedOption: &opt2, TimeSpentMs: 2000},
		},
		cand2: {
			{ItemID: itm1.ID, SelectedOption: &opt1, TimeSpentMs: 1500},
			{ItemID: itm2.ID, SelectedOption: nil, TimeSpentMs: 500}, // omitted
		},
	}

	stats, err := engine.ComputeClassicalStats(context.Background(), examID, itemsList, responses)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(stats))

	// Statistics for itm1
	var stat1 analysis.ClassicalItemStats
	for _, s := range stats {
		if s.ItemID == itm1.ID {
			stat1 = s
		}
	}
	assert.Equal(t, 2, stat1.TotalResponses)
	assert.Equal(t, 2, stat1.CorrectCount)
	assert.Equal(t, 1.0, stat1.PValue)

	// Statistics for itm2
	var stat2 analysis.ClassicalItemStats
	for _, s := range stats {
		if s.ItemID == itm2.ID {
			stat2 = s
		}
	}
	assert.Equal(t, 1, stat2.TotalResponses)
	assert.Equal(t, 1, stat2.OmittedCount)
}

func TestComputeExamStatistics(t *testing.T) {
	itemRepo := &MockItemRepository{items: make(map[uuid.UUID]*item.Item)}
	respRepo := &MockResponseRepository{}
	logger := zap.NewNop()

	engine := analysis.NewEngine(itemRepo, respRepo, logger)

	rawScores := []float64{80.0, 90.0, 70.0, 85.0, 95.0}
	thetas := []float64{1.2, 1.8, 0.5, 1.3, 2.1}
	itemVariances := []float64{0.15, 0.20, 0.10}

	stats := engine.ComputeExamStatistics(rawScores, thetas, 3, itemVariances)

	assert.Equal(t, 5, stats.TotalAppeared)
	assert.Equal(t, 84.0, stats.MeanRawScore)
	assert.Equal(t, 70.0, stats.MinRawScore)
	assert.Equal(t, 95.0, stats.MaxRawScore)
	assert.True(t, stats.CronbachAlpha > 0)
}
