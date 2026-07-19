package paper

import (
	"crypto/sha256"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Paper represents a generated test form
type Paper struct {
	ID               uuid.UUID
	ExamID           uuid.UUID
	PaperCode        string
	FormNumber       int
	EncryptedItemIDs []byte    // AES-256-GCM encrypted JSON array of item UUIDs
	EncryptedKeyID   string    // HSM key reference used for encryption
	ItemSequenceHash [32]byte  // SHA-256 of ordered item IDs for integrity verification

	// Psychometric profile
	Profile PaperProfile

	// Generation metadata
	GenerationLog GenerationLog
	GeneratedAt   time.Time
	GeneratedBy   string // "SYSTEM" or admin UUID
	DigitalSig    []byte // Ed25519 signature over (ExamID || FormNumber || ItemSequenceHash)
}

// PaperProfile holds the psychometric properties of the paper
type PaperProfile struct {
	MeanDifficulty   float64             // Mean IRT-b of selected items
	StdDifficulty    float64             // Std dev of IRT-b
	TestInformation  map[float64]float64 // theta → I(theta)
	ReliabilityEst   float64             // Estimated marginal reliability
	TotalTimeEstSecs int                 // Sum of estimated_time_secs
	ItemCount        int
	DifficultyDist   map[string]int      // difficulty_level → count
	CognitiveDist    map[string]int      // cognitive_level → count
	ChapterDist      map[string]int      // chapter_id → count
}

// GenerationLog captures the solver metrics
type GenerationLog struct {
	SolverName        string
	SolveTimeMs       int64
	OptimalityGap     float64
	Iterations        int
	ConstraintsMet    []string // List of satisfied constraints
	ConstraintsFailed []string // List of violated constraints (should be empty for valid papers)
	ObjectiveValue    float64  // MIP objective function value
	ItemPoolSize      int      // Size of eligible item pool
	FeasibilityStatus string   // "OPTIMAL", "FEASIBLE", "INFEASIBLE"
}

// PaperRepository defines operations on Paper entities
type PaperRepository interface {
	Save(paper *Paper) error
	FindByID(id uuid.UUID) (*Paper, error)
	FindByExamID(examID uuid.UUID) ([]*Paper, error)
}

// NewPaper creates a new Paper instance and computes the initial hash
func NewPaper(examID uuid.UUID, paperCode string, formNumber int, generatedBy string) *Paper {
	return &Paper{
		ID:          uuid.New(),
		ExamID:      examID,
		PaperCode:   paperCode,
		FormNumber:  formNumber,
		GeneratedAt: time.Now().UTC(),
		GeneratedBy: generatedBy,
		Profile: PaperProfile{
			TestInformation: make(map[float64]float64),
			DifficultyDist:  make(map[string]int),
			CognitiveDist:   make(map[string]int),
			ChapterDist:     make(map[string]int),
		},
		GenerationLog: GenerationLog{
			ConstraintsMet:    make([]string, 0),
			ConstraintsFailed: make([]string, 0),
		},
	}
}

// SetItemSequence sets the item sequence and computes its SHA-256 hash
func (p *Paper) SetItemSequence(itemIDs []uuid.UUID) {
	hasher := sha256.New()
	for _, id := range itemIDs {
		hasher.Write(id[:])
	}
	copy(p.ItemSequenceHash[:], hasher.Sum(nil))
}

// Validate checks the basic integrity of the paper
func (p *Paper) Validate() error {
	if p.ID == uuid.Nil {
		return errors.New("paper ID cannot be nil")
	}
	if p.ExamID == uuid.Nil {
		return errors.New("exam ID cannot be nil")
	}
	if p.FormNumber <= 0 {
		return errors.New("form number must be positive")
	}
	if len(p.EncryptedItemIDs) == 0 {
		return errors.New("encrypted item IDs cannot be empty")
	}
	emptyHash := [32]byte{}
	if p.ItemSequenceHash == emptyHash {
		return errors.New("item sequence hash cannot be empty")
	}
	return nil
}
