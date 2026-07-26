package service_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/aegis-platform/aegis/internal/audit"
	"github.com/aegis-platform/aegis/internal/domain/item"
	"github.com/aegis-platform/aegis/internal/service"
	"github.com/aegis-platform/aegis/pkg/crypto"
)

// MockItemRepository implements service.ItemRepository
type MockItemRepository struct {
	mu           sync.RWMutex
	items        map[uuid.UUID]*item.Item
	versions     map[uuid.UUID][]item.ItemVersion
	deletedItems map[uuid.UUID]bool
}

func NewMockItemRepository() *MockItemRepository {
	return &MockItemRepository{
		items:        make(map[uuid.UUID]*item.Item),
		versions:     make(map[uuid.UUID][]item.ItemVersion),
		deletedItems: make(map[uuid.UUID]bool),
	}
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
	if m.deletedItems[id] {
		return nil, errors.New("item not found")
	}
	itm, ok := m.items[id]
	if !ok {
		return nil, errors.New("item not found")
	}
	return itm, nil
}

func (m *MockItemRepository) GetByExternalID(ctx context.Context, orgID uuid.UUID, externalID string) (*item.Item, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, itm := range m.items {
		if itm.ExternalID == externalID && !m.deletedItems[itm.ID] {
			return itm, nil
		}
	}
	return nil, errors.New("item not found")
}

func (m *MockItemRepository) Update(ctx context.Context, i *item.Item) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[i.ID] = i
	return nil
}

func (m *MockItemRepository) SoftDelete(ctx context.Context, orgID, id, deletedBy uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deletedItems[id] = true
	return nil
}

func (m *MockItemRepository) List(ctx context.Context, filter item.ItemFilter, cursor string, limit int) ([]*item.Item, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []*item.Item
	for _, itm := range m.items {
		if !m.deletedItems[itm.ID] {
			list = append(list, itm)
		}
	}
	return list, "", nil
}

func (m *MockItemRepository) GetEligibleForPaperGeneration(ctx context.Context, orgID, subjectID uuid.UUID, maxExposureIndex float64) ([]*item.Item, error) {
	return nil, nil
}

func (m *MockItemRepository) GetEnemies(ctx context.Context, itemID uuid.UUID) ([]item.ItemEnemy, error) {
	return nil, nil
}

func (m *MockItemRepository) CreateVersion(ctx context.Context, version *item.ItemVersion) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.versions[version.ItemID] = append(m.versions[version.ItemID], *version)
	return nil
}

func (m *MockItemRepository) GetVersionHistory(ctx context.Context, itemID uuid.UUID) ([]item.ItemVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.versions[itemID], nil
}

// MockKeyManager implements crypto.KeyManager
type MockKeyManager struct {
	pub ed25519.PublicKey
	prv ed25519.PrivateKey
}

func NewMockKeyManager() *MockKeyManager {
	pub, prv, _ := ed25519.GenerateKey(rand.Reader)
	return &MockKeyManager{pub: pub, prv: prv}
}

func (m *MockKeyManager) GenerateDataKey(keyID string) ([]byte, []byte, error) {
	plaintext := make([]byte, 32)
	rand.Read(plaintext)
	ciphertext := make([]byte, 32)
	copy(ciphertext, plaintext)
	return plaintext, ciphertext, nil
}

func (m *MockKeyManager) DecryptDataKey(keyID string, encryptedKey []byte) ([]byte, error) {
	decrypted := make([]byte, len(encryptedKey))
	copy(decrypted, encryptedKey)
	return decrypted, nil
}

func (m *MockKeyManager) GetSigningKey(keyID string) (ed25519.PrivateKey, error) {
	return m.prv, nil
}

func (m *MockKeyManager) GetVerificationKey(keyID string) (ed25519.PublicKey, error) {
	return m.pub, nil
}

// MockAuditRepo implements audit.Repository
type MockAuditRepo struct{}

func (m *MockAuditRepo) Insert(entry *audit.Entry) error                   { return nil }
func (m *MockAuditRepo) GetByID(id int64) (*audit.Entry, error)            { return nil, nil }
func (m *MockAuditRepo) GetRange(start, end int64) ([]*audit.Entry, error) { return nil, nil }
func (m *MockAuditRepo) GetLatest() (*audit.Entry, error)                  { return nil, nil }
func (m *MockAuditRepo) Query(f audit.AuditFilter, c string, l int) ([]*audit.Entry, string, error) {
	return nil, "", nil
}

// MockAuditSigner implements audit.Signer
type MockAuditSigner struct{}

func (m *MockAuditSigner) Sign(data []byte) ([]byte, error)            { return []byte("sig"), nil }
func (m *MockAuditSigner) Verify(data, signature []byte) (bool, error) { return true, nil }

func setupItemService(t *testing.T) (*service.ItemService, *MockItemRepository) {
	repo := NewMockItemRepository()
	km := NewMockKeyManager()

	logger := zap.NewNop()

	auditRepo := &MockAuditRepo{}
	auditSigner := &MockAuditSigner{}
	auditSvc, err := audit.NewService(auditRepo, auditSigner, logger)
	assert.NoError(t, err)

	encryptSvc := crypto.NewEncryptionService(km)
	signingSvc := crypto.NewSigningService(km)

	svc := service.NewItemService(repo, auditSvc, signingSvc, encryptSvc, logger)
	return svc, repo
}

func TestItemService_CreateItem(t *testing.T) {
	svc, _ := setupItemService(t)

	orgID := uuid.New()
	subjectID := uuid.New()
	chapterID := uuid.New()
	topicID := uuid.New()
	authorID := uuid.New()

	content := item.ItemContent{
		Stem: "What is 2 + 2?",
		Options: []item.Option{
			{Label: "A", Content: "4"},
			{Label: "B", Content: "5"},
		},
	}

	req := service.CreateItemRequest{
		ExternalID:      "Q-001",
		ItemType:        item.ItemTypeMCQSingle,
		OrganizationID:  orgID,
		SubjectID:       subjectID,
		ChapterID:       chapterID,
		TopicID:         topicID,
		Content:         content,
		AnswerKeyPlain:  []byte("A"),
		SolutionPlain:   []byte("2 + 2 = 4"),
		DifficultyLevel: item.DifficultyEasy,
		CognitiveLevel:  item.CognitiveLevelRemember,
		AuthorID:        authorID,
	}

	itm, err := svc.CreateItem(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, itm)
	assert.Equal(t, item.ItemStatusDraft, itm.Status)
	assert.Equal(t, "Q-001", itm.ExternalID)
	assert.Equal(t, authorID, itm.Approval.AuthorID)
	assert.NotEmpty(t, itm.AnswerKey)
}

func TestItemService_SubmitForReview(t *testing.T) {
	svc, repo := setupItemService(t)

	orgID := uuid.New()
	authorID := uuid.New()

	itm := &item.Item{
		ID:             uuid.New(),
		OrganizationID: orgID,
		ExternalID:     "Q-002",
		Type:           item.ItemTypeMCQSingle,
		Status:         item.ItemStatusDraft,
		Content: item.ItemContent{
			Stem: "Sample question",
			Options: []item.Option{
				{Label: "A", Content: "Option A"},
				{Label: "B", Content: "Option B"},
			},
		},
		Approval: item.ApprovalChain{
			AuthorID: authorID,
		},
	}
	err := repo.Create(context.Background(), itm)
	assert.NoError(t, err)

	// Non-author submits -> error
	err = svc.SubmitForReview(context.Background(), orgID, itm.ID, uuid.New())
	assert.Error(t, err)

	// Author submits -> success
	err = svc.SubmitForReview(context.Background(), orgID, itm.ID, authorID)
	assert.NoError(t, err)

	updated, err := repo.GetByID(context.Background(), orgID, itm.ID)
	assert.NoError(t, err)
	assert.Equal(t, item.ItemStatusReview, updated.Status)
}

func TestItemService_ReviewItem(t *testing.T) {
	svc, repo := setupItemService(t)

	orgID := uuid.New()
	authorID := uuid.New()
	reviewerID := uuid.New()

	itm := &item.Item{
		ID:             uuid.New(),
		OrganizationID: orgID,
		ExternalID:     "Q-003",
		Status:         item.ItemStatusReview,
		Approval: item.ApprovalChain{
			AuthorID: authorID,
		},
	}
	err := repo.Create(context.Background(), itm)
	assert.NoError(t, err)

	// Author reviews their own item -> forbidden
	err = svc.ReviewItem(context.Background(), orgID, itm.ID, service.ReviewItemRequest{
		ReviewerID: authorID,
		Decision:   item.ReviewApproved,
		Comments:   "Looks good",
	})
	assert.Error(t, err)

	// Proper reviewer reviews -> success
	err = svc.ReviewItem(context.Background(), orgID, itm.ID, service.ReviewItemRequest{
		ReviewerID: reviewerID,
		Decision:   item.ReviewApproved,
		Comments:   "Looks good",
	})
	assert.NoError(t, err)

	updated, err := repo.GetByID(context.Background(), orgID, itm.ID)
	assert.NoError(t, err)
	assert.Equal(t, item.ItemStatusCalibration, updated.Status)
	assert.Equal(t, reviewerID, *updated.Approval.ReviewerID)
}

func TestItemService_CalibrateAndActivate(t *testing.T) {
	svc, repo := setupItemService(t)

	orgID := uuid.New()
	authorID := uuid.New()
	reviewerID := uuid.New()
	psychID := uuid.New()
	approverID := uuid.New()

	itm := &item.Item{
		ID:             uuid.New(),
		OrganizationID: orgID,
		ExternalID:     "Q-004",
		Status:         item.ItemStatusCalibration,
		Approval: item.ApprovalChain{
			AuthorID:   authorID,
			ReviewerID: &reviewerID,
		},
	}
	err := repo.Create(context.Background(), itm)
	assert.NoError(t, err)

	// Calibrate
	params := item.IRTParameters{
		A:                     1.5,
		B:                     0.5,
		C:                     0.2,
		CalibrationSampleSize: 500,
	}
	err = svc.CalibrateItem(context.Background(), orgID, itm.ID, service.CalibrateItemRequest{
		PsychometricianID: psychID,
		Params:            params,
	})
	assert.NoError(t, err)

	updated, err := repo.GetByID(context.Background(), orgID, itm.ID)
	assert.NoError(t, err)
	assert.Equal(t, item.ItemStatusPilot, updated.Status)

	// Activate
	err = svc.ActivateItem(context.Background(), orgID, itm.ID, approverID)
	assert.NoError(t, err)

	activeItem, err := repo.GetByID(context.Background(), orgID, itm.ID)
	assert.NoError(t, err)
	assert.Equal(t, item.ItemStatusActive, activeItem.Status)
}
