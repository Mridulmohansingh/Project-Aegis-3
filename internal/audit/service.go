package audit

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ActorType represents the type of actor triggering the audit event
type ActorType string

const (
	ActorTypeUser    ActorType = "USER"
	ActorTypeSystem  ActorType = "SYSTEM"
	ActorTypeService ActorType = "SERVICE"
)

// Entry represents a single audit log entry
type Entry struct {
	ID             int64
	EventTime      time.Time
	EventType      string    // e.g., "ITEM_CREATED", "PAPER_GENERATED", "EXAM_STARTED"
	ActorID        uuid.UUID
	ActorType      ActorType // USER, SYSTEM, SERVICE
	ActorIP        string
	ResourceType   string    // e.g., "item", "paper", "exam", "response"
	ResourceID     uuid.UUID
	OrganizationID uuid.UUID
	Action         string    // e.g., "CREATE", "UPDATE", "DELETE", "TRANSITION"
	Detail         map[string]interface{}
	PreviousHash   []byte    // Hash of previous entry
	EntryHash      []byte    // SHA-256(PreviousHash || serialized entry data)
	CheckpointSig  []byte    // Ed25519 signature (only on checkpoint entries)
}

// IntegrityReport holds the result of a chain verification
type IntegrityReport struct {
	Verified       bool
	EntriesChecked int64
	FirstBrokenAt  *int64 // nil if all verified
	ExpectedHash   []byte
	ActualHash     []byte
}

// AuditFilter for querying entries
type AuditFilter struct {
	ActorID      uuid.UUID
	ResourceType string
	Action       string
}

// Repository interface for audit persistence
type Repository interface {
	Insert(entry *Entry) error
	GetByID(id int64) (*Entry, error)
	GetRange(startID, endID int64) ([]*Entry, error)
	GetLatest() (*Entry, error)
	Query(filter AuditFilter, cursor string, limit int) ([]*Entry, string, error)
}

// Signer interface for digital signatures
type Signer interface {
	Sign(data []byte) ([]byte, error)
	Verify(data, signature []byte) (bool, error)
}

// Service provides an append-only audit log with Merkle chain integrity.
type Service struct {
	repo     Repository
	signer   Signer
	logger   *zap.Logger
	lastHash []byte
	mu       sync.RWMutex
}

// NewService creates a new audit Service.
func NewService(repo Repository, signer Signer, logger *zap.Logger) (*Service, error) {
	svc := &Service{
		repo:   repo,
		signer: signer,
		logger: logger,
	}

	latest, err := repo.GetLatest()
	if err != nil {
		svc.lastHash = nil
	} else if latest != nil {
		svc.lastHash = latest.EntryHash
	}

	return svc, nil
}

// Append adds a new entry to the audit log with hash chaining.
func (s *Service) Append(ctx context.Context, entry *Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry.PreviousHash = s.lastHash
	entry.EventTime = time.Now().UTC()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	hasher := sha256.New()
	hasher.Write(entry.PreviousHash)
	hasher.Write(data)
	entry.EntryHash = hasher.Sum(nil)

	if err := s.repo.Insert(entry); err != nil {
		return err
	}

	s.lastHash = entry.EntryHash
	return nil
}

// VerifyChain verifies the integrity of the audit log from entry startID to endID.
func (s *Service) VerifyChain(ctx context.Context, startID, endID int64) (*IntegrityReport, error) {
	entries, err := s.repo.GetRange(startID, endID)
	if err != nil {
		return nil, err
	}
	
	if len(entries) == 0 {
		return &IntegrityReport{Verified: true}, nil
	}

	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		
		if i > 0 {
			if string(entry.PreviousHash) != string(entries[i-1].EntryHash) {
				id := entry.ID
				return &IntegrityReport{
					Verified:       false,
					EntriesChecked: int64(i + 1),
					FirstBrokenAt:  &id,
					ExpectedHash:   entries[i-1].EntryHash,
					ActualHash:     entry.PreviousHash,
				}, nil
			}
		}

		entryCopy := *entry
		entryCopy.EntryHash = nil
		data, err := json.Marshal(&entryCopy)
		if err != nil {
			return nil, err
		}

		hasher := sha256.New()
		hasher.Write(entry.PreviousHash)
		hasher.Write(data)
		calcHash := hasher.Sum(nil)

		if string(calcHash) != string(entry.EntryHash) {
			id := entry.ID
			return &IntegrityReport{
				Verified:       false,
				EntriesChecked: int64(i + 1),
				FirstBrokenAt:  &id,
				ExpectedHash:   calcHash,
				ActualHash:     entry.EntryHash,
			}, nil
		}
	}

	return &IntegrityReport{
		Verified:       true,
		EntriesChecked: int64(len(entries)),
	}, nil
}

// Checkpoint creates a signed checkpoint entry.
func (s *Service) Checkpoint(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.lastHash == nil {
		return errors.New("no entries to checkpoint")
	}

	sig, err := s.signer.Sign(s.lastHash)
	if err != nil {
		return err
	}

	entry := &Entry{
		EventType:     "SYSTEM_CHECKPOINT",
		ActorType:     ActorTypeSystem,
		Action:        "CHECKPOINT",
		EventTime:     time.Now().UTC(),
		PreviousHash:  s.lastHash,
		CheckpointSig: sig,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	hasher := sha256.New()
	hasher.Write(entry.PreviousHash)
	hasher.Write(data)
	entry.EntryHash = hasher.Sum(nil)

	if err := s.repo.Insert(entry); err != nil {
		return err
	}

	s.lastHash = entry.EntryHash
	return nil
}
