package papergen

import (
	"context"
	"testing"

	"github.com/aegis-platform/aegis/internal/domain/blueprint"
	"github.com/aegis-platform/aegis/internal/domain/item"
	"github.com/aegis-platform/aegis/internal/domain/paper"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Mocks

type mockItemRepo struct {
	items   []*item.Item
	enemies []item.ItemEnemy
}

func (m *mockItemRepo) Create(ctx context.Context, item *item.Item) error { return nil }
func (m *mockItemRepo) GetByID(ctx context.Context, orgID, id uuid.UUID) (*item.Item, error) {
	return nil, nil
}
func (m *mockItemRepo) GetByExternalID(ctx context.Context, orgID uuid.UUID, externalID string) (*item.Item, error) {
	return nil, nil
}
func (m *mockItemRepo) Update(ctx context.Context, item *item.Item) error { return nil }
func (m *mockItemRepo) SoftDelete(ctx context.Context, orgID, id uuid.UUID, deletedBy uuid.UUID) error {
	return nil
}
func (m *mockItemRepo) List(ctx context.Context, filter item.ItemFilter, cursor string, limit int) ([]*item.Item, string, error) {
	return nil, "", nil
}
func (m *mockItemRepo) GetEligibleForPaperGeneration(ctx context.Context, orgID, subjectID uuid.UUID, maxExposureIndex float64) ([]*item.Item, error) {
	return m.items, nil
}
func (m *mockItemRepo) GetEnemies(ctx context.Context, itemID uuid.UUID) ([]item.ItemEnemy, error) {
	var res []item.ItemEnemy
	for _, e := range m.enemies {
		if e.ItemAID == itemID || e.ItemBID == itemID {
			res = append(res, e)
		}
	}
	return res, nil
}
func (m *mockItemRepo) CreateVersion(ctx context.Context, version *item.ItemVersion) error {
	return nil
}
func (m *mockItemRepo) GetVersionHistory(ctx context.Context, itemID uuid.UUID) ([]item.ItemVersion, error) {
	return nil, nil
}

type mockPaperRepo struct{}

func (m *mockPaperRepo) Save(paper *paper.Paper) error                         { return nil }
func (m *mockPaperRepo) FindByID(id uuid.UUID) (*paper.Paper, error)           { return nil, nil }
func (m *mockPaperRepo) FindByExamID(examID uuid.UUID) ([]*paper.Paper, error) { return nil, nil }

type mockCryptoSvc struct{}

func (m *mockCryptoSvc) EncryptItemIDs(itemIDs []uuid.UUID) ([]byte, string, error) {
	return []byte("encrypted"), "key1", nil
}
func (m *mockCryptoSvc) SignPaper(examID uuid.UUID, formNum int, hash [32]byte) ([]byte, error) {
	return []byte("signature"), nil
}

// Tests

func TestBuildConstraints(t *testing.T) {
	ch1ID := uuid.New()
	bp := &blueprint.Blueprint{
		TotalItems: 2,
		Constraints: blueprint.BlueprintConstraints{
			Chapters: []blueprint.ChapterConstraint{
				{ChapterID: ch1ID, MinItems: 1, MaxItems: 2, Weight: 1.0},
			},
			TimeBudgetSecs:   100,
			MaxExposureIndex: 0.8,
		},
	}

	items := []*item.Item{
		{ID: uuid.New(), ChapterID: ch1ID, EstimatedTimeSecs: 30, DifficultyLevel: item.DifficultyLevel("EASY")},
		{ID: uuid.New(), ChapterID: ch1ID, EstimatedTimeSecs: 40, DifficultyLevel: item.DifficultyLevel("MEDIUM")},
		{ID: uuid.New(), ChapterID: uuid.New(), EstimatedTimeSecs: 50, DifficultyLevel: item.DifficultyLevel("HARD")},
	}
	items[0].Exposure.ExposureIndex = 0.5
	items[1].Exposure.ExposureIndex = 0.9 // Over-exposed
	items[2].Exposure.ExposureIndex = 0.2

	enemies := []item.ItemEnemy{
		{ItemAID: items[0].ID, ItemBID: items[2].ID},
	}

	m := BuildConstraints(bp, items, enemies)

	if len(m.Variables) != 3 {
		t.Errorf("Expected 3 variables, got %d", len(m.Variables))
	}

	// Verify exposure control
	if m.Variables[1].Upper != 0.0 {
		t.Errorf("Expected over-exposed item to have upper bound 0.0")
	}

	// Check constraints count: total(1) + ch1_min/max(2) + diff_distribution(4) + cog_distribution(6) + time(1) + enemy(1) = 15
	if len(m.Constraints) != 15 {
		t.Errorf("Expected 15 constraints, got %d", len(m.Constraints))
	}
}

func TestSolveFeasible(t *testing.T) {
	ch1ID := uuid.New()
	items := make([]*item.Item, 10)
	for i := 0; i < 10; i++ {
		items[i] = &item.Item{
			ID:                uuid.New(),
			ChapterID:         ch1ID,
			EstimatedTimeSecs: 10,
			DifficultyLevel:   item.DifficultyLevel("EASY"),
			CognitiveLevel:    item.CognitiveLevel("REMEMBER"),
		}
	}

	bp := &blueprint.Blueprint{
		TotalItems: 2,
		Constraints: blueprint.BlueprintConstraints{
			Chapters: []blueprint.ChapterConstraint{
				{ChapterID: ch1ID, MinItems: 2, MaxItems: 2, Weight: 1.0},
			},
			TimeBudgetSecs:   100,
			MaxExposureIndex: 0.9,
			Difficulty: blueprint.DifficultyConstraint{
				Distribution: blueprint.DifficultyDistribution{
					Easy: 1.0,
				},
			},
			CognitiveLevels: blueprint.CognitiveLevelConstraint{
				Remember: 1.0,
			},
		},
	}

	logger, _ := zap.NewDevelopment()
	engine := NewEngine(
		&mockItemRepo{items: items},
		&mockPaperRepo{},
		&mockCryptoSvc{},
		logger,
	)

	req := GenerationRequest{
		ExamID:           uuid.New(),
		Blueprint:        bp,
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
	t.Logf("Paper Profile: %+v", p.Profile)
	if len(p.EncryptedItemIDs) == 0 {
		t.Errorf("Expected encrypted item IDs")
	}
	if p.Profile.ItemCount != 2 {
		t.Errorf("Expected profile item count to be 2, got %d", p.Profile.ItemCount)
	}
}
