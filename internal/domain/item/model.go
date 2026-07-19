// Package item defines the Item aggregate root and its value objects for the
// AEGIS Question Bank bounded context.
//
// An Item represents a single assessment question with full psychometric metadata,
// IRT parameters, exposure control, multi-language support, and a cryptographically
// signed approval chain.
//
// The Item lifecycle follows a strict state machine:
//
//	DRAFT → REVIEW → CALIBRATION → PILOT → ACTIVE → SUSPENDED → RETIRED
//	                                                      ↓
//	                                                   ACTIVE (reactivation)
//
// All state transitions are validated and audited.
package item

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/aegis-platform/aegis/pkg/apperrors"
)

// ──────────────────────────────────────────────
//  Enums
// ──────────────────────────────────────────────

// ItemType represents the type of assessment question.
type ItemType string

const (
	ItemTypeMCQSingle       ItemType = "MCQ_SINGLE"
	ItemTypeMCQMulti        ItemType = "MCQ_MULTI"
	ItemTypeInteger         ItemType = "INTEGER"
	ItemTypeDescriptive     ItemType = "DESCRIPTIVE"
	ItemTypeMatching        ItemType = "MATCHING"
	ItemTypeAssertionReason ItemType = "ASSERTION_REASON"
	ItemTypeCaseBased       ItemType = "CASE_BASED"
)

// String returns the string representation of ItemType.
func (t ItemType) String() string { return string(t) }

// IsValid checks if the item type is a known valid type.
func (t ItemType) IsValid() bool {
	switch t {
	case ItemTypeMCQSingle, ItemTypeMCQMulti, ItemTypeInteger,
		ItemTypeDescriptive, ItemTypeMatching, ItemTypeAssertionReason,
		ItemTypeCaseBased:
		return true
	}
	return false
}

// ItemStatus represents the lifecycle state of an item.
type ItemStatus string

const (
	ItemStatusDraft       ItemStatus = "DRAFT"
	ItemStatusReview      ItemStatus = "REVIEW"
	ItemStatusCalibration ItemStatus = "CALIBRATION"
	ItemStatusPilot       ItemStatus = "PILOT"
	ItemStatusActive      ItemStatus = "ACTIVE"
	ItemStatusSuspended   ItemStatus = "SUSPENDED"
	ItemStatusRetired     ItemStatus = "RETIRED"
)

// String returns the string representation of ItemStatus.
func (s ItemStatus) String() string { return string(s) }

// IsValid checks if the status is a known valid status.
func (s ItemStatus) IsValid() bool {
	switch s {
	case ItemStatusDraft, ItemStatusReview, ItemStatusCalibration,
		ItemStatusPilot, ItemStatusActive, ItemStatusSuspended,
		ItemStatusRetired:
		return true
	}
	return false
}

// validTransitions defines the allowed state transitions for items.
// The key is the current state; the value is the set of valid next states.
var validTransitions = map[ItemStatus][]ItemStatus{
	ItemStatusDraft:       {ItemStatusReview},
	ItemStatusReview:      {ItemStatusDraft, ItemStatusCalibration},       // Can be sent back for revision
	ItemStatusCalibration: {ItemStatusReview, ItemStatusPilot},            // Can be sent back for re-review
	ItemStatusPilot:       {ItemStatusActive, ItemStatusCalibration},      // Can go back for re-calibration
	ItemStatusActive:      {ItemStatusSuspended, ItemStatusRetired},
	ItemStatusSuspended:   {ItemStatusActive, ItemStatusRetired},          // Can be reactivated
	ItemStatusRetired:     {},                                              // Terminal state
}

// CanTransitionTo checks if a transition from the current status to the target is valid.
func (s ItemStatus) CanTransitionTo(target ItemStatus) bool {
	targets, ok := validTransitions[s]
	if !ok {
		return false
	}
	for _, t := range targets {
		if t == target {
			return true
		}
	}
	return false
}

// ValidTransitions returns the list of valid next states from the current state.
func (s ItemStatus) ValidTransitions() []ItemStatus {
	return validTransitions[s]
}

// DifficultyLevel represents the categorical difficulty of an item.
type DifficultyLevel string

const (
	DifficultyEasy     DifficultyLevel = "EASY"
	DifficultyMedium   DifficultyLevel = "MEDIUM"
	DifficultyHard     DifficultyLevel = "HARD"
	DifficultyVeryHard DifficultyLevel = "VERY_HARD"
)

// String returns the string representation.
func (d DifficultyLevel) String() string { return string(d) }

// IsValid checks if the difficulty level is valid.
func (d DifficultyLevel) IsValid() bool {
	switch d {
	case DifficultyEasy, DifficultyMedium, DifficultyHard, DifficultyVeryHard:
		return true
	}
	return false
}

// CognitiveLevel represents Bloom's Taxonomy cognitive levels.
type CognitiveLevel string

const (
	CognitiveLevelRemember   CognitiveLevel = "REMEMBER"
	CognitiveLevelUnderstand CognitiveLevel = "UNDERSTAND"
	CognitiveLevelApply      CognitiveLevel = "APPLY"
	CognitiveLevelAnalyze    CognitiveLevel = "ANALYZE"
	CognitiveLevelEvaluate   CognitiveLevel = "EVALUATE"
	CognitiveLevelCreate     CognitiveLevel = "CREATE"
)

// String returns the string representation.
func (c CognitiveLevel) String() string { return string(c) }

// IsValid checks if the cognitive level is valid.
func (c CognitiveLevel) IsValid() bool {
	switch c {
	case CognitiveLevelRemember, CognitiveLevelUnderstand, CognitiveLevelApply,
		CognitiveLevelAnalyze, CognitiveLevelEvaluate, CognitiveLevelCreate:
		return true
	}
	return false
}

// ReviewDecision represents the outcome of a review.
type ReviewDecision string

const (
	ReviewApproved ReviewDecision = "APPROVED"
	ReviewRejected ReviewDecision = "REJECTED"
	ReviewRevision ReviewDecision = "REVISION"
)

// ──────────────────────────────────────────────
//  Value Objects
// ──────────────────────────────────────────────

// ItemContent holds the question content in a structured format.
type ItemContent struct {
	// Stem is the main question text (may contain LaTeX).
	Stem string `json:"stem" db:"stem"`
	// Options holds the answer choices for MCQ-type items.
	Options []Option `json:"options,omitempty" db:"options"`
	// Media holds references to attached media (images, diagrams, audio).
	Media []MediaRef `json:"media,omitempty" db:"media"`
	// Passage holds a reading passage for comprehension-type items.
	Passage string `json:"passage,omitempty" db:"passage"`
	// SubQuestions holds sub-items for case-based questions.
	SubQuestions []SubQuestion `json:"sub_questions,omitempty" db:"sub_questions"`
}

// Option represents a single answer choice.
type Option struct {
	// Label is the option identifier (A, B, C, D, etc.).
	Label string `json:"label" db:"label"`
	// Content is the option text (may contain LaTeX).
	Content string `json:"content" db:"content"`
	// Media holds optional media for this option.
	Media []MediaRef `json:"media,omitempty" db:"media"`
}

// MediaRef holds a reference to a media asset.
type MediaRef struct {
	// Type is the media type (IMAGE, AUDIO, VIDEO, DIAGRAM).
	Type string `json:"type" db:"type"`
	// URL is the object storage URL.
	URL string `json:"url" db:"url"`
	// AltText is the accessibility description.
	AltText string `json:"alt_text" db:"alt_text"`
	// MimeType is the MIME type (e.g., image/png).
	MimeType string `json:"mime_type" db:"mime_type"`
}

// SubQuestion represents a sub-item within a case-based question.
type SubQuestion struct {
	Index         int      `json:"index" db:"index"`
	Stem          string   `json:"stem" db:"stem"`
	Options       []Option `json:"options,omitempty" db:"options"`
	MarkingScheme MarkingScheme `json:"marking_scheme" db:"marking_scheme"`
}

// MarkingScheme defines the scoring rules for an item.
type MarkingScheme struct {
	// CorrectMarks is the marks awarded for a correct answer.
	CorrectMarks float64 `json:"correct" db:"correct_marks"`
	// IncorrectMarks is the penalty for an incorrect answer (typically negative).
	IncorrectMarks float64 `json:"incorrect" db:"incorrect_marks"`
	// UnansweredMarks is the marks for an unanswered question (typically 0).
	UnansweredMarks float64 `json:"unanswered" db:"unanswered_marks"`
	// PartialMarks indicates if partial credit is allowed for multi-select.
	PartialMarks bool `json:"partial_marks" db:"partial_marks"`
}

// IRTParameters holds the Item Response Theory parameters for the 3-Parameter Logistic model.
//
// The 3PL model defines the probability of a correct response as:
//
//	P(θ) = c + (1 - c) / (1 + exp(-a(θ - b)))
//
// where:
//   - a = discrimination parameter (slope at inflection point)
//   - b = difficulty parameter (location on ability scale, in logits)
//   - c = pseudo-guessing parameter (lower asymptote)
//   - θ = examinee ability
type IRTParameters struct {
	// A is the discrimination parameter. Higher values indicate better discrimination.
	// Typical range: 0.5 to 2.5. Values outside this range suggest calibration issues.
	A float64 `json:"a" db:"irt_a"`
	// B is the difficulty parameter in logits. Centered at 0 for average difficulty.
	// Typical range: -3.0 to +3.0.
	B float64 `json:"b" db:"irt_b"`
	// C is the pseudo-guessing parameter. Represents probability of correct response
	// by very low-ability examinees. For 4-option MCQ, theoretical value is 0.25.
	// Typical range: 0.0 to 0.35.
	C float64 `json:"c" db:"irt_c"`
	// SEA is the standard error of the discrimination parameter.
	SEA float64 `json:"se_a" db:"irt_se_a"`
	// SEB is the standard error of the difficulty parameter.
	SEB float64 `json:"se_b" db:"irt_se_b"`
	// SEC is the standard error of the guessing parameter.
	SEC float64 `json:"se_c" db:"irt_se_c"`
	// InformationAtZero is the Fisher information at θ=0 (precomputed for paper generation).
	InformationAtZero float64 `json:"info_at_0" db:"irt_info_at_0"`
	// CalibrationSampleSize is the number of examinees used for calibration.
	CalibrationSampleSize int `json:"calibration_sample" db:"calibration_sample"`
	// CalibrationDate is when the item was last calibrated.
	CalibrationDate *time.Time `json:"calibration_date,omitempty" db:"calibration_date"`
}

// Validate checks that IRT parameters are within acceptable ranges.
func (p IRTParameters) Validate() error {
	if p.A < 0.1 || p.A > 5.0 {
		return fmt.Errorf("discrimination parameter 'a' (%.3f) outside valid range [0.1, 5.0]", p.A)
	}
	if p.B < -4.0 || p.B > 4.0 {
		return fmt.Errorf("difficulty parameter 'b' (%.3f) outside valid range [-4.0, 4.0]", p.B)
	}
	if p.C < 0.0 || p.C > 0.5 {
		return fmt.Errorf("guessing parameter 'c' (%.3f) outside valid range [0.0, 0.5]", p.C)
	}
	if p.SEA < 0 || p.SEB < 0 || p.SEC < 0 {
		return fmt.Errorf("standard errors must be non-negative")
	}
	if p.CalibrationSampleSize < 100 {
		return fmt.Errorf("calibration sample size (%d) is below minimum of 100", p.CalibrationSampleSize)
	}
	return nil
}

// Probability returns P(θ) — the probability of a correct response at ability level θ.
func (p IRTParameters) Probability(theta float64) float64 {
	exponent := -p.A * (theta - p.B)
	return p.C + (1.0-p.C)/(1.0+math.Exp(exponent))
}

// Information returns I(θ) — the Fisher information at ability level θ.
// I(θ) = a² × (P(θ) - c)² × Q(θ) / ((1-c)² × P(θ))
func (p IRTParameters) Information(theta float64) float64 {
	prob := p.Probability(theta)
	q := 1.0 - prob
	numerator := p.A * p.A * (prob - p.C) * (prob - p.C) * q
	denominator := (1.0 - p.C) * (1.0 - p.C) * prob
	if denominator < 1e-15 {
		return 0.0
	}
	return numerator / denominator
}

// ClassicalStats holds classical test theory statistics.
type ClassicalStats struct {
	// PValue is the proportion of examinees answering correctly (item difficulty in CTT).
	PValue float64 `json:"p_value" db:"p_value"`
	// DiscriminationIndex is the upper-lower 27% group difference.
	DiscriminationIndex float64 `json:"discrimination_index" db:"discrimination_idx"`
	// PointBiserial is the point-biserial correlation between item score and total score.
	PointBiserial float64 `json:"point_biserial" db:"point_biserial"`
	// DistractorAnalysis shows the proportion selecting each option.
	DistractorAnalysis map[string]float64 `json:"distractor_analysis" db:"distractor_analysis"`
}

// ExposureControl tracks how frequently an item has been used.
type ExposureControl struct {
	// ExposureCount is the total number of times this item has been administered.
	ExposureCount int `json:"exposure_count" db:"exposure_count"`
	// MaxExposure is the maximum allowed administrations before retirement.
	MaxExposure int `json:"max_exposure" db:"max_exposure"`
	// ExposureIndex is the ratio of exposure_count to total exams conducted.
	// Range: 0.0 to 1.0. Items with high indices should be retired.
	ExposureIndex float64 `json:"exposure_index" db:"exposure_index"`
	// LastUsedAt is the timestamp of the last administration.
	LastUsedAt *time.Time `json:"last_used_at,omitempty" db:"last_used_at"`
	// CooldownUntil prevents reuse until this time (exposure control mechanism).
	CooldownUntil *time.Time `json:"cooldown_until,omitempty" db:"cooldown_until"`
}

// IsAvailable checks if the item is available for selection (not in cooldown, not over-exposed).
func (e ExposureControl) IsAvailable(maxExposureIndex float64) bool {
	if e.ExposureCount >= e.MaxExposure {
		return false
	}
	if e.ExposureIndex >= maxExposureIndex {
		return false
	}
	if e.CooldownUntil != nil && time.Now().Before(*e.CooldownUntil) {
		return false
	}
	return true
}

// ApprovalChain records the cryptographically signed approval workflow.
type ApprovalChain struct {
	// Author
	AuthorID        uuid.UUID  `json:"author_id" db:"author_id"`
	AuthorSignature []byte     `json:"-" db:"author_signature"` // Never serialize signatures to API

	// Reviewer
	ReviewerID       *uuid.UUID     `json:"reviewer_id,omitempty" db:"reviewer_id"`
	ReviewerSignature []byte        `json:"-" db:"reviewer_signature"`
	ReviewerDecision  ReviewDecision `json:"reviewer_decision,omitempty" db:"reviewer_decision"`
	ReviewedAt        *time.Time     `json:"reviewed_at,omitempty" db:"reviewed_at"`

	// Psychometrician
	PsychometricianID  *uuid.UUID `json:"psychometrician_id,omitempty" db:"psychometrician_id"`
	PsychometricianSig []byte     `json:"-" db:"psychometrician_sig"`

	// Final Approver
	ApproverID        *uuid.UUID `json:"approver_id,omitempty" db:"approver_id"`
	ApproverSignature []byte     `json:"-" db:"approver_signature"`
	ApprovedAt        *time.Time `json:"approved_at,omitempty" db:"approved_at"`
}

// ValidateSeparationOfDuties ensures that no single person fills multiple roles.
func (a ApprovalChain) ValidateSeparationOfDuties() error {
	ids := make(map[uuid.UUID]string)
	ids[a.AuthorID] = "author"

	if a.ReviewerID != nil {
		if role, exists := ids[*a.ReviewerID]; exists {
			return fmt.Errorf("separation of duties violation: reviewer is same as %s", role)
		}
		ids[*a.ReviewerID] = "reviewer"
	}
	if a.PsychometricianID != nil {
		if role, exists := ids[*a.PsychometricianID]; exists {
			return fmt.Errorf("separation of duties violation: psychometrician is same as %s", role)
		}
		ids[*a.PsychometricianID] = "psychometrician"
	}
	if a.ApproverID != nil {
		if role, exists := ids[*a.ApproverID]; exists {
			return fmt.Errorf("separation of duties violation: approver is same as %s", role)
		}
	}
	return nil
}

// ──────────────────────────────────────────────
//  Aggregate Root
// ──────────────────────────────────────────────

// Item is the aggregate root for the Question Bank bounded context.
// It represents a single assessment item (question) with complete metadata.
type Item struct {
	// Identity
	ID             uuid.UUID `json:"id" db:"id"`
	OrganizationID uuid.UUID `json:"organization_id" db:"organization_id"`
	ExternalID     string    `json:"external_id" db:"external_id"`

	// Classification
	Type           ItemType       `json:"item_type" db:"item_type"`
	Status         ItemStatus     `json:"status" db:"status"`
	DifficultyLevel DifficultyLevel `json:"difficulty_level" db:"difficulty_level"`
	CognitiveLevel CognitiveLevel  `json:"cognitive_level" db:"cognitive_level"`

	// Taxonomy
	SubjectID       uuid.UUID  `json:"subject_id" db:"subject_id"`
	ChapterID       uuid.UUID  `json:"chapter_id" db:"chapter_id"`
	TopicID         uuid.UUID  `json:"topic_id" db:"topic_id"`
	SubTopicID      *uuid.UUID `json:"sub_topic_id,omitempty" db:"sub_topic_id"`
	LearningOutcomeID *uuid.UUID `json:"learning_outcome_id,omitempty" db:"learning_outcome_id"`

	// Content (encrypted at rest via application-layer encryption)
	Content       ItemContent   `json:"content" db:"question_content"`
	AnswerKey     []byte        `json:"-" db:"answer_key"`      // AES-256-GCM encrypted, never in API response
	Solution      []byte        `json:"-" db:"solution"`         // AES-256-GCM encrypted
	MarkingScheme MarkingScheme `json:"marking_scheme" db:"marking_scheme"`
	EstimatedTimeSecs int       `json:"estimated_time_secs" db:"estimated_time_secs"`

	// Psychometric Parameters
	IRTParams      *IRTParameters  `json:"irt_parameters,omitempty" db:"-"` // Flattened to columns in DB
	ClassicalStats *ClassicalStats `json:"classical_stats,omitempty" db:"-"` // Flattened to columns in DB

	// Exposure Control
	Exposure ExposureControl `json:"exposure" db:"-"` // Flattened to columns in DB

	// Language & Variants
	PrimaryLanguage string     `json:"primary_language" db:"primary_language"`
	VariantGroupID  *uuid.UUID `json:"variant_group_id,omitempty" db:"variant_group_id"`
	ParentItemID    *uuid.UUID `json:"parent_item_id,omitempty" db:"parent_item_id"`

	// Approval Chain
	Approval ApprovalChain `json:"approval" db:"-"` // Flattened to columns in DB

	// Metadata
	Tags      []string  `json:"tags" db:"tags"`
	Version   int       `json:"version" db:"version"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	CreatedBy uuid.UUID `json:"created_by" db:"created_by"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
	UpdatedBy uuid.UUID `json:"updated_by" db:"updated_by"`
	DeletedAt *time.Time `json:"-" db:"deleted_at"`
}

// NewItem creates a new Item in DRAFT status with the given metadata.
func NewItem(
	orgID uuid.UUID,
	externalID string,
	itemType ItemType,
	subjectID, chapterID, topicID uuid.UUID,
	content ItemContent,
	markingScheme MarkingScheme,
	authorID uuid.UUID,
) (*Item, error) {
	if !itemType.IsValid() {
		return nil, apperrors.NewValidation("invalid item type", map[string]string{
			"item_type": fmt.Sprintf("'%s' is not a valid item type", itemType),
		})
	}

	now := time.Now().UTC()
	return &Item{
		ID:             uuid.New(),
		OrganizationID: orgID,
		ExternalID:     externalID,
		Type:           itemType,
		Status:         ItemStatusDraft,
		SubjectID:      subjectID,
		ChapterID:      chapterID,
		TopicID:        topicID,
		Content:        content,
		MarkingScheme:  markingScheme,
		EstimatedTimeSecs: 120, // Default 2 minutes
		PrimaryLanguage:   "en",
		Exposure: ExposureControl{
			MaxExposure: 50,
		},
		Approval: ApprovalChain{
			AuthorID: authorID,
		},
		Version:   1,
		CreatedAt: now,
		CreatedBy: authorID,
		UpdatedAt: now,
		UpdatedBy: authorID,
	}, nil
}

// TransitionTo changes the item status after validating the transition.
func (i *Item) TransitionTo(newStatus ItemStatus, actorID uuid.UUID) error {
	if !newStatus.IsValid() {
		return apperrors.NewValidation("invalid target status", map[string]string{
			"status": fmt.Sprintf("'%s' is not a valid status", newStatus),
		})
	}
	if !i.Status.CanTransitionTo(newStatus) {
		return apperrors.NewInvalidStateTransition("Item", i.Status.String(), newStatus.String())
	}

	i.Status = newStatus
	i.UpdatedAt = time.Now().UTC()
	i.UpdatedBy = actorID
	i.Version++
	return nil
}

// SetIRTParameters sets the psychometric IRT parameters after validation.
func (i *Item) SetIRTParameters(params IRTParameters) error {
	if err := params.Validate(); err != nil {
		return apperrors.NewValidation("invalid IRT parameters", map[string]string{
			"irt": err.Error(),
		})
	}
	// Precompute information at θ=0 for efficient paper generation queries
	params.InformationAtZero = params.Information(0.0)
	i.IRTParams = &params
	return nil
}

// IncrementExposure increments the exposure counter and recalculates the exposure index.
func (i *Item) IncrementExposure(totalExamsToDate int) {
	i.Exposure.ExposureCount++
	now := time.Now().UTC()
	i.Exposure.LastUsedAt = &now

	if totalExamsToDate > 0 {
		i.Exposure.ExposureIndex = float64(i.Exposure.ExposureCount) / float64(totalExamsToDate)
	}
}

// ShouldRetire checks if the item should be automatically retired based on exposure.
func (i *Item) ShouldRetire() bool {
	return i.Exposure.ExposureCount >= i.Exposure.MaxExposure
}

// ContentHash returns the SHA-256 hash of the item content for integrity verification.
func (i *Item) ContentHash() [32]byte {
	data, _ := json.Marshal(struct {
		Content       ItemContent   `json:"content"`
		MarkingScheme MarkingScheme `json:"marking_scheme"`
		AnswerKey     []byte        `json:"answer_key"`
	}{
		Content:       i.Content,
		MarkingScheme: i.MarkingScheme,
		AnswerKey:     i.AnswerKey,
	})
	return sha256.Sum256(data)
}

// ──────────────────────────────────────────────
//  Item Version (Entity)
// ──────────────────────────────────────────────

// ChangeType represents the type of change made to an item.
type ChangeType string

const (
	ChangeTypeContent   ChangeType = "CONTENT"
	ChangeTypeMetadata  ChangeType = "METADATA"
	ChangeTypeStatus    ChangeType = "STATUS"
	ChangeTypeIRTUpdate ChangeType = "IRT_UPDATE"
)

// ItemVersion records a single version change in an item's history.
// This is an immutable entity — once created, it must never be modified.
type ItemVersion struct {
	ID           uuid.UUID       `json:"id" db:"id"`
	ItemID       uuid.UUID       `json:"item_id" db:"item_id"`
	Version      int             `json:"version" db:"version"`
	ChangeType   ChangeType      `json:"change_type" db:"change_type"`
	PreviousData json.RawMessage `json:"previous_data" db:"previous_data"`
	NewData      json.RawMessage `json:"new_data" db:"new_data"`
	ChangedBy    uuid.UUID       `json:"changed_by" db:"changed_by"`
	ChangedAt    time.Time       `json:"changed_at" db:"changed_at"`
	ChangeReason string          `json:"change_reason" db:"change_reason"`
	DigitalSig   []byte          `json:"-" db:"digital_sig"` // Ed25519 signature
}

// ──────────────────────────────────────────────
//  Item Enemy (Value Object)
// ──────────────────────────────────────────────

// ItemEnemy records a pair of items that must never appear on the same test form.
// This prevents content overlap or information leakage between items.
type ItemEnemy struct {
	ItemAID   uuid.UUID `json:"item_a_id" db:"item_a_id"`
	ItemBID   uuid.UUID `json:"item_b_id" db:"item_b_id"`
	Reason    string    `json:"reason" db:"reason"`
	CreatedBy uuid.UUID `json:"created_by" db:"created_by"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// NewItemEnemy creates an enemy pair with canonical ordering (A < B).
func NewItemEnemy(idA, idB uuid.UUID, reason string, createdBy uuid.UUID) ItemEnemy {
	// Canonical ordering ensures uniqueness regardless of insertion order
	if idA.String() > idB.String() {
		idA, idB = idB, idA
	}
	return ItemEnemy{
		ItemAID:   idA,
		ItemBID:   idB,
		Reason:    reason,
		CreatedBy: createdBy,
		CreatedAt: time.Now().UTC(),
	}
}

// ──────────────────────────────────────────────
//  Item Translation (Entity)
// ──────────────────────────────────────────────

// TranslationStatus represents the status of a translation.
type TranslationStatus string

const (
	TranslationStatusDraft    TranslationStatus = "DRAFT"
	TranslationStatusReview   TranslationStatus = "REVIEW"
	TranslationStatusVerified TranslationStatus = "VERIFIED"
	TranslationStatusFlagged  TranslationStatus = "FLAGGED" // DIF detected
)

// ItemTranslation holds a translated version of an item.
type ItemTranslation struct {
	ID                uuid.UUID         `json:"id" db:"id"`
	ItemID            uuid.UUID         `json:"item_id" db:"item_id"`
	Language          string            `json:"language" db:"language"` // ISO 639-1 (hi, ta, te, bn, mr, etc.)
	Content           ItemContent       `json:"content" db:"question_content"`
	TranslationStatus TranslationStatus `json:"translation_status" db:"translation_status"`
	TranslatorID      *uuid.UUID        `json:"translator_id,omitempty" db:"translator_id"`
	VerifierID        *uuid.UUID        `json:"verifier_id,omitempty" db:"verifier_id"`
	VerifiedAt        *time.Time        `json:"verified_at,omitempty" db:"verified_at"`
	DIFFlag           bool              `json:"dif_flag" db:"dif_flag"` // Differential Item Functioning flag
	CreatedAt         time.Time         `json:"created_at" db:"created_at"`
}

// ──────────────────────────────────────────────
//  Repository Interface
// ──────────────────────────────────────────────

// ItemFilter defines the filter criteria for querying items.
type ItemFilter struct {
	OrganizationID *uuid.UUID       `json:"organization_id"`
	SubjectID      *uuid.UUID       `json:"subject_id"`
	ChapterID      *uuid.UUID       `json:"chapter_id"`
	TopicID        *uuid.UUID       `json:"topic_id"`
	Status         *ItemStatus      `json:"status"`
	DifficultyLevel *DifficultyLevel `json:"difficulty_level"`
	CognitiveLevel *CognitiveLevel  `json:"cognitive_level"`
	ItemType       *ItemType        `json:"item_type"`
	Tags           []string         `json:"tags"`
	HasIRTParams   *bool            `json:"has_irt_params"`
	MaxExposureIndex *float64       `json:"max_exposure_index"`
	Language       *string          `json:"language"`
}

// ItemRepository defines the persistence operations for items.
type ItemRepository interface {
	// Create persists a new item.
	Create(item *Item) error
	// GetByID retrieves an item by its UUID.
	GetByID(orgID, id uuid.UUID) (*Item, error)
	// GetByExternalID retrieves an item by its external identifier within an organization.
	GetByExternalID(orgID uuid.UUID, externalID string) (*Item, error)
	// Update persists changes to an existing item with optimistic locking.
	Update(item *Item) error
	// SoftDelete marks an item as deleted without physical removal.
	SoftDelete(orgID, id uuid.UUID, deletedBy uuid.UUID) error
	// List retrieves a filtered, paginated list of items.
	List(filter ItemFilter, cursor string, limit int) ([]*Item, string, error)
	// GetEligibleForPaperGeneration returns active items matching paper generation criteria.
	GetEligibleForPaperGeneration(orgID, subjectID uuid.UUID, maxExposureIndex float64) ([]*Item, error)
	// GetEnemies returns all enemy pairs for a given item.
	GetEnemies(itemID uuid.UUID) ([]ItemEnemy, error)
	// CreateVersion records an item version change.
	CreateVersion(version *ItemVersion) error
	// GetVersionHistory returns the version history of an item.
	GetVersionHistory(itemID uuid.UUID) ([]ItemVersion, error)
}
