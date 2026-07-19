package papergen

import (
	"context"
	"testing"
	"time"

	"github.com/aegis-platform/aegis/internal/domain/blueprint"
	"github.com/aegis-platform/aegis/internal/domain/item"
	"github.com/aegis-platform/aegis/internal/domain/paper"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Mocks

type mockItemRepo struct {
	items []*item.Item
}

func (m *mockItemRepo) GetEligibleForPaperGeneration(ctx context.Context, examID uuid.UUID) ([]*item.Item, error) {
	return m.items, nil
}
func (m *mockItemRepo) Save(item *item.Item) error { return nil }
func (m *mockItemRepo) FindByID(id uuid.UUID) (*item.Item, error) { return nil, nil }

type mockPaperRepo struct{}
func (m *mockPaperRepo) Save(paper *paper.Paper) error { return nil }
func (m *mockPaperRepo) FindByID(id uuid.UUID) (*paper.Paper, error) { return nil, nil }
func (m *mockPaperRepo) FindByExamID(examID uuid.UUID) ([]*paper.Paper, error) { return nil, nil }

type mockEnemyRepo struct {
	enemies []item.ItemEnemy
}
func (m *mockEnemyRepo) GetEnemiesForExam(ctx context.Context, examID uuid.UUID) ([]item.ItemEnemy, error) {
	return m.enemies, nil
}
func (m *mockEnemyRepo) Save(enemy *item.ItemEnemy) error { return nil }

type mockCryptoSvc struct{}
func (m *mockCryptoSvc) EncryptItemIDs(itemIDs []uuid.UUID) ([]byte, string, error) {
	return []byte("encrypted"), "key1", nil
}
func (m *mockCryptoSvc) SignPaper(examID uuid.UUID, formNum int, hash [32]byte) ([]byte, error) {
	return []byte("signature"), nil
}

// Tests

func TestBuildConstraints(t *testing.T) {
	bp := &blueprint.Blueprint{
		TotalItems: 2,
		ChapterConstraints: map[string]blueprint.Range{
			"ch1": {Min: 1, Max: 2},
		},
		MaxTimeSecs:     100,
		MaxExposureRate: 0.8,
	}

	items := []*item.Item{
		{ID: uuid.New(), ChapterID: "ch1", EstimatedTimeSecs: 30, ExposureRate: 0.5},
		{ID: uuid.New(), ChapterID: "ch1", EstimatedTimeSecs: 40, ExposureRate: 0.9}, // Over-exposed
		{ID: uuid.New(), ChapterID: "ch2", EstimatedTimeSecs: 50, ExposureRate: 0.2},
	}

	enemies := []item.ItemEnemy{
		{ItemID1: items[0].ID, ItemID2: items[2].ID},
	}

	m := BuildConstraints(bp, items, enemies)

	if len(m.Variables) != 3 {
		t.Errorf("Expected 3 variables, got %d", len(m.Variables))
	}

	// Verify exposure control
	if m.Variables[1].Upper != 0.0 {
		t.Errorf("Expected over-exposed item to have upper bound 0.0")
	}

	// Check constraints count: total(1) + ch1_min/max(2) + time(1) + enemy(1) = 5
	if len(m.Constraints) != 5 {
		t.Errorf("Expected 5 constraints, got %d", len(m.Constraints))
	}
}

func TestSolveFeasible(t *testing.T) {
	items := make([]*item.Item, 10)
	for i := 0; i < 10; i++ {
		items[i] = &item.Item{
			ID:                uuid.New(),
			ChapterID:         "ch1",
			EstimatedTimeSecs: 10,
		}
	}

	bp := &blueprint.Blueprint{
		TotalItems:  5,
		MaxTimeSecs: 100,
	}

	m := BuildConstraints(bp, items, nil)
	
	// We set an objective to prefer the first 5 items
	for i := 0; i < 5; i++ {
		m.Variables[i].ObjCoeff = -1.0 // minimize
	}

	sol, err := m.Solve(5)
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}

	if sol.Status != StatusOptimal && sol.Status != StatusFeasible {
		t.Errorf("Expected Feasible or Optimal status, got %s", sol.Status)
	}

	// Count selected items
	count := 0
	for _, val := range sol.Values {
		if val > 0.5 {
			count++
		}
	}

	if count != 5 {
		t.Errorf("Expected 5 items selected, got %d", count)
	}
}

func TestSolveInfeasible(t *testing.T) {
	items := make([]*item.Item, 2)
	for i := 0; i < 2; i++ {
		items[i] = &item.Item{
			ID:                uuid.New(),
			EstimatedTimeSecs: 50,
		}
	}

	bp := &blueprint.Blueprint{
		TotalItems:  3, // Asking for 3 items from a pool of 2
		MaxTimeSecs: 100,
	}

	m := BuildConstraints(bp, items, nil)
	sol, err := m.Solve(5)
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}

	if sol.Status != StatusInfeasible {
		t.Errorf("Expected INFEASIBLE status, got %s", sol.Status)
	}
}

func TestGenerateEngine(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	items := []*item.Item{
		{ID: uuid.New(), ChapterID: "ch1", EstimatedTimeSecs: 30, DifficultyLevel: "EASY", IRTA: 1.0, IRTB: -1.0},
		{ID: uuid.New(), ChapterID: "ch1", EstimatedTimeSecs: 40, DifficultyLevel: "MEDIUM", IRTA: 1.2, IRTB: 0.0},
		{ID: uuid.New(), ChapterID: "ch2", EstimatedTimeSecs: 50, DifficultyLevel: "HARD", IRTA: 1.5, IRTB: 1.0},
	}

	engine := NewEngine(
		&mockItemRepo{items: items},
		&mockPaperRepo{},
		&mockEnemyRepo{enemies: nil},
		&mockCryptoSvc{},
		logger,
	)

	req := GenerationRequest{
		ExamID: uuid.New(),
		Blueprint: &blueprint.Blueprint{
			TotalItems: 2,
			ChapterConstraints: map[string]blueprint.Range{
				"ch1": {Min: 1, Max: 2},
			},
			MaxTimeSecs: 100,
		},
		FormCount:        2,
		MaxSolveTimeSecs: 5,
	}

	res, err := engine.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if len(res.Papers) != 2 {
		t.Errorf("Expected 2 papers, got %d", len(res.Papers))
	}

	p := res.Papers[0]
	if len(p.EncryptedItemIDs) == 0 {
		t.Errorf("Expected encrypted item IDs")
	}
	if p.Profile.ItemCount != 2 {
		t.Errorf("Expected profile item count to be 2, got %d", p.Profile.ItemCount)
	}
}
