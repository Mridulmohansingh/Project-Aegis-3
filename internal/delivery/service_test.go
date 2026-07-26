package delivery_test

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

	"github.com/aegis-platform/aegis/internal/delivery"
	"github.com/aegis-platform/aegis/internal/domain/exam"
	"github.com/aegis-platform/aegis/pkg/crypto"
)

// MockExamRepository implements exam.ExamRepository
type MockExamRepository struct {
	mu    sync.RWMutex
	exams map[uuid.UUID]*exam.Exam
}

func (m *MockExamRepository) Create(ctx interface{}, e *exam.Exam) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.exams[e.ID] = e
	return nil
}

func (m *MockExamRepository) GetByID(ctx interface{}, orgID, id uuid.UUID) (*exam.Exam, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.exams[id]
	if !ok {
		return nil, errors.New("exam not found")
	}
	return e, nil
}

func (m *MockExamRepository) Update(ctx interface{}, e *exam.Exam) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.exams[e.ID] = e
	return nil
}

func (m *MockExamRepository) List(ctx interface{}, orgID uuid.UUID, status *exam.ExamStatus, cursor string, limit int) ([]*exam.Exam, string, error) {
	return nil, "", nil
}

// MockSessionRepository implements exam.SessionRepository
type MockSessionRepository struct {
	mu       sync.RWMutex
	sessions map[uuid.UUID]*exam.ExamSession
}

func (m *MockSessionRepository) Create(ctx interface{}, s *exam.ExamSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.ID] = s
	return nil
}

func (m *MockSessionRepository) GetByID(ctx interface{}, id uuid.UUID) (*exam.ExamSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, errors.New("session not found")
	}
	return s, nil
}

func (m *MockSessionRepository) GetByCandidateAndExam(ctx interface{}, candidateID, examID uuid.UUID) (*exam.ExamSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.sessions {
		if s.CandidateID == candidateID && s.ExamID == examID {
			return s, nil
		}
	}
	return nil, nil
}

func (m *MockSessionRepository) Update(ctx interface{}, s *exam.ExamSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.ID] = s
	return nil
}

func (m *MockSessionRepository) CountActive(ctx interface{}, examID uuid.UUID) (int, error) {
	return 0, nil
}

// MockResponseRepository implements exam.ResponseRepository
type MockResponseRepository struct {
	mu        sync.RWMutex
	responses map[uuid.UUID]*exam.Response
}

func (m *MockResponseRepository) Upsert(ctx interface{}, r *exam.Response) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[r.ID] = r
	return nil
}

func (m *MockResponseRepository) UpsertBatch(ctx interface{}, rs []*exam.Response) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range rs {
		m.responses[r.ID] = r
	}
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
	return make([]byte, 32), make([]byte, 32), nil
}

func (m *MockKeyManager) DecryptDataKey(keyID string, encryptedKey []byte) ([]byte, error) {
	return make([]byte, 32), nil
}

func (m *MockKeyManager) GetSigningKey(keyID string) (ed25519.PrivateKey, error)      { return m.prv, nil }
func (m *MockKeyManager) GetVerificationKey(keyID string) (ed25519.PublicKey, error) { return m.pub, nil }

func setupDeliveryService(t *testing.T) (*delivery.Service, *MockExamRepository, *MockSessionRepository, *MockResponseRepository) {
	examRepo := &MockExamRepository{exams: make(map[uuid.UUID]*exam.Exam)}
	sessionRepo := &MockSessionRepository{sessions: make(map[uuid.UUID]*exam.ExamSession)}
	responseRepo := &MockResponseRepository{responses: make(map[uuid.UUID]*exam.Response)}
	
	km := NewMockKeyManager()
	encryptSvc := crypto.NewEncryptionService(km)
	logger := zap.NewNop()

	svc := delivery.NewService(examRepo, sessionRepo, responseRepo, encryptSvc, logger)
	return svc, examRepo, sessionRepo, responseRepo
}

func TestInitializeSession(t *testing.T) {
	svc, examRepo, _, _ := setupDeliveryService(t)

	examID := uuid.New()
	candID := uuid.New()
	paperID := uuid.New()

	// Exam not active
	examRepo.exams[examID] = &exam.Exam{
		ID:     examID,
		Status: exam.ExamStatusDraft,
	}

	session, err := svc.InitializeSession(context.Background(), delivery.InitializeSessionRequest{
		ExamID:      examID,
		CandidateID: candID,
		PaperID:     paperID,
	})
	assert.Error(t, err)
	assert.Nil(t, session)

	// Exam is active
	examRepo.exams[examID] = &exam.Exam{
		ID:              examID,
		Status:          exam.ExamStatusActive,
		DurationMinutes: 180,
	}

	session, err = svc.InitializeSession(context.Background(), delivery.InitializeSessionRequest{
		ExamID:      examID,
		CandidateID: candID,
		PaperID:     paperID,
	})
	assert.NoError(t, err)
	assert.NotNil(t, session)
	assert.Equal(t, exam.SessionAuthenticated, session.Status)
}

func TestStartAndSubmitAnswer(t *testing.T) {
	svc, examRepo, sessionRepo, respRepo := setupDeliveryService(t)

	examID := uuid.New()
	candID := uuid.New()
	paperID := uuid.New()
	itemID := uuid.New()

	examRepo.exams[examID] = &exam.Exam{
		ID:              examID,
		Status:          exam.ExamStatusActive,
		DurationMinutes: 180,
	}

	session, err := svc.InitializeSession(context.Background(), delivery.InitializeSessionRequest{
		ExamID:      examID,
		CandidateID: candID,
		PaperID:     paperID,
	})
	assert.NoError(t, err)

	// Start Session
	err = svc.StartSession(context.Background(), session.ID, 180)
	assert.NoError(t, err)

	// Verify status updated in DB/repo
	dbSession, err := sessionRepo.GetByID(context.Background(), session.ID)
	assert.NoError(t, err)
	assert.Equal(t, exam.SessionInProgress, dbSession.Status)

	// Submit Answer
	opt := 2
	err = svc.SubmitAnswer(context.Background(), delivery.SubmitAnswerRequest{
		SessionID:      session.ID,
		ItemID:         itemID,
		SectionIndex:   0,
		QuestionIndex:  1,
		SelectedOption: &opt,
		TimeSpentMs:    5000,
	})
	assert.NoError(t, err)

	// Check saved responses
	assert.Equal(t, 1, len(respRepo.responses))
}
