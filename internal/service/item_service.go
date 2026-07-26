// Package service implements the Question Bank business logic layer.
//
// The ItemService orchestrates item lifecycle operations:
//   - CRUD with validation and audit logging
//   - State machine transitions (Draft → Review → Calibration → Active → Retired)
//   - Separation of duties enforcement
//   - Version history recording with digital signatures
//   - Exposure tracking and automatic retirement
//
// It is the single source of truth for item business rules. HTTP handlers
// and future gRPC handlers delegate all logic to this service.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegis-platform/aegis/internal/audit"
	"github.com/aegis-platform/aegis/internal/domain/item"
	"github.com/aegis-platform/aegis/pkg/apperrors"
	"github.com/aegis-platform/aegis/pkg/crypto"
)

// ItemRepository defines the persistence interface.
// Matches item.ItemRepository but with context parameter.
type ItemRepository interface {
	Create(ctx context.Context, i *item.Item) error
	GetByID(ctx context.Context, orgID, id uuid.UUID) (*item.Item, error)
	GetByExternalID(ctx context.Context, orgID uuid.UUID, externalID string) (*item.Item, error)
	Update(ctx context.Context, i *item.Item) error
	SoftDelete(ctx context.Context, orgID, id, deletedBy uuid.UUID) error
	List(ctx context.Context, filter item.ItemFilter, cursor string, limit int) ([]*item.Item, string, error)
	GetEligibleForPaperGeneration(ctx context.Context, orgID, subjectID uuid.UUID, maxExposureIndex float64) ([]*item.Item, error)
	GetEnemies(ctx context.Context, itemID uuid.UUID) ([]item.ItemEnemy, error)
	CreateVersion(ctx context.Context, version *item.ItemVersion) error
	GetVersionHistory(ctx context.Context, itemID uuid.UUID) ([]item.ItemVersion, error)
}

// ItemService implements the business logic for the Question Bank.
type ItemService struct {
	repo       ItemRepository
	auditSvc   *audit.Service
	signingSvc *crypto.SigningService
	encryptSvc *crypto.EncryptionService
	logger     *zap.Logger
}

// NewItemService creates a new ItemService with all dependencies.
func NewItemService(
	repo ItemRepository,
	auditSvc *audit.Service,
	signingSvc *crypto.SigningService,
	encryptSvc *crypto.EncryptionService,
	logger *zap.Logger,
) *ItemService {
	return &ItemService{
		repo:       repo,
		auditSvc:   auditSvc,
		signingSvc: signingSvc,
		encryptSvc: encryptSvc,
		logger:     logger.With(zap.String("component", "item_service")),
	}
}

// CreateItemRequest holds the parameters for creating a new item.
type CreateItemRequest struct {
	ExternalID        string
	ItemType          item.ItemType
	OrganizationID    uuid.UUID
	SubjectID         uuid.UUID
	ChapterID         uuid.UUID
	TopicID           uuid.UUID
	SubTopicID        *uuid.UUID
	LearningOutcomeID *uuid.UUID
	Content           item.ItemContent
	AnswerKeyPlain    []byte // Plaintext answer key (will be encrypted)
	SolutionPlain     []byte // Plaintext solution (will be encrypted)
	MarkingScheme     item.MarkingScheme
	DifficultyLevel   item.DifficultyLevel
	CognitiveLevel    item.CognitiveLevel
	EstimatedTimeSecs int
	PrimaryLanguage   string
	Tags              []string
	AuthorID          uuid.UUID
}

// CreateItem creates a new item in DRAFT status, encrypts the answer key,
// and records the creation in the audit log.
func (s *ItemService) CreateItem(ctx context.Context, req CreateItemRequest) (*item.Item, error) {
	// Create domain object
	newItem, err := item.NewItem(
		req.OrganizationID,
		req.ExternalID,
		req.ItemType,
		req.SubjectID, req.ChapterID, req.TopicID,
		req.Content,
		req.MarkingScheme,
		req.AuthorID,
	)
	if err != nil {
		return nil, err
	}

	// Set optional fields
	newItem.SubTopicID = req.SubTopicID
	newItem.LearningOutcomeID = req.LearningOutcomeID
	newItem.DifficultyLevel = req.DifficultyLevel
	newItem.CognitiveLevel = req.CognitiveLevel
	if req.EstimatedTimeSecs > 0 {
		newItem.EstimatedTimeSecs = req.EstimatedTimeSecs
	}
	if req.PrimaryLanguage != "" {
		newItem.PrimaryLanguage = req.PrimaryLanguage
	}
	newItem.Tags = req.Tags

	// Encrypt answer key
	if len(req.AnswerKeyPlain) > 0 {
		encrypted, err := s.encryptSvc.Encrypt(
			req.AnswerKeyPlain,
			"aegis-item-keys",
			[]byte(newItem.ID.String()), // AAD: item ID prevents ciphertext transplant
		)
		if err != nil {
			return nil, apperrors.NewInternal("failed to encrypt answer key", err)
		}
		newItem.AnswerKey = encrypted.Ciphertext
	}

	// Encrypt solution if provided
	if len(req.SolutionPlain) > 0 {
		encrypted, err := s.encryptSvc.Encrypt(
			req.SolutionPlain,
			"aegis-item-keys",
			[]byte(newItem.ID.String()),
		)
		if err != nil {
			return nil, apperrors.NewInternal("failed to encrypt solution", err)
		}
		newItem.Solution = encrypted.Ciphertext
	}

	// Sign the content (author signature)
	contentHash := newItem.ContentHash()
	sig, err := s.signingSvc.Sign(contentHash[:], "aegis-author-signing")
	if err != nil {
		s.logger.Warn("failed to sign item content, continuing without signature", zap.Error(err))
	} else {
		newItem.Approval.AuthorSignature = sig
	}

	// Persist
	if err := s.repo.Create(ctx, newItem); err != nil {
		return nil, err
	}

	// Audit log
	s.publishAudit(ctx, "ITEM_CREATED", req.AuthorID, "item", newItem.ID, req.OrganizationID, "CREATE", map[string]interface{}{
		"external_id": req.ExternalID,
		"item_type":   string(req.ItemType),
		"subject_id":  req.SubjectID.String(),
	})

	s.logger.Info("item created",
		zap.String("item_id", newItem.ID.String()),
		zap.String("external_id", newItem.ExternalID),
		zap.String("author_id", req.AuthorID.String()),
	)

	return newItem, nil
}

// GetItem retrieves an item by ID.
func (s *ItemService) GetItem(ctx context.Context, orgID, itemID uuid.UUID) (*item.Item, error) {
	return s.repo.GetByID(ctx, orgID, itemID)
}

// UpdateItemRequest holds the parameters for updating an item.
type UpdateItemRequest struct {
	Content           *item.ItemContent
	MarkingScheme     *item.MarkingScheme
	DifficultyLevel   *item.DifficultyLevel
	CognitiveLevel    *item.CognitiveLevel
	EstimatedTimeSecs *int
	Tags              []string
	ExpectedVersion   int // Optimistic locking
	ActorID           uuid.UUID
}

// UpdateItem updates an item's content and metadata.
// Only DRAFT items can be content-edited. Metadata can be updated in other states.
func (s *ItemService) UpdateItem(ctx context.Context, orgID, itemID uuid.UUID, req UpdateItemRequest) (*item.Item, error) {
	existing, err := s.repo.GetByID(ctx, orgID, itemID)
	if err != nil {
		return nil, err
	}

	// Content changes only allowed in DRAFT
	if req.Content != nil && existing.Status != item.ItemStatusDraft {
		return nil, apperrors.NewConflict("content can only be modified in DRAFT status")
	}

	// Capture previous state for version history
	previousData, _ := json.Marshal(existing)
	changeType := item.ChangeTypeMetadata

	// Apply changes
	if req.Content != nil {
		existing.Content = *req.Content
		changeType = item.ChangeTypeContent
	}
	if req.MarkingScheme != nil {
		existing.MarkingScheme = *req.MarkingScheme
	}
	if req.DifficultyLevel != nil {
		existing.DifficultyLevel = *req.DifficultyLevel
	}
	if req.CognitiveLevel != nil {
		existing.CognitiveLevel = *req.CognitiveLevel
	}
	if req.EstimatedTimeSecs != nil {
		existing.EstimatedTimeSecs = *req.EstimatedTimeSecs
	}
	if req.Tags != nil {
		existing.Tags = req.Tags
	}

	existing.UpdatedAt = time.Now().UTC()
	existing.UpdatedBy = req.ActorID
	existing.Version++

	// Persist with optimistic lock
	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}

	// Record version history
	newData, _ := json.Marshal(existing)
	s.recordVersion(ctx, existing.ID, existing.Version, changeType, previousData, newData, req.ActorID, "Content/metadata update")

	// Audit
	s.publishAudit(ctx, "ITEM_UPDATED", req.ActorID, "item", itemID, orgID, "UPDATE", map[string]interface{}{
		"change_type": string(changeType),
		"version":     existing.Version,
	})

	return existing, nil
}

// DeleteItem soft-deletes an item.
func (s *ItemService) DeleteItem(ctx context.Context, orgID, itemID, actorID uuid.UUID) error {
	existing, err := s.repo.GetByID(ctx, orgID, itemID)
	if err != nil {
		return err
	}

	// Cannot delete ACTIVE items — must retire first
	if existing.Status == item.ItemStatusActive {
		return apperrors.NewConflict("cannot delete ACTIVE items; retire the item first")
	}

	if err := s.repo.SoftDelete(ctx, orgID, itemID, actorID); err != nil {
		return err
	}

	s.publishAudit(ctx, "ITEM_DELETED", actorID, "item", itemID, orgID, "DELETE", map[string]interface{}{
		"previous_status": string(existing.Status),
	})

	return nil
}

// ListItems retrieves a filtered, paginated list of items.
func (s *ItemService) ListItems(ctx context.Context, filter item.ItemFilter, cursor string, limit int) ([]*item.Item, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.repo.List(ctx, filter, cursor, limit)
}

// SubmitForReview transitions an item from DRAFT to REVIEW.
func (s *ItemService) SubmitForReview(ctx context.Context, orgID, itemID, actorID uuid.UUID) error {
	existing, err := s.repo.GetByID(ctx, orgID, itemID)
	if err != nil {
		return err
	}

	// Only the author can submit for review
	if existing.Approval.AuthorID != actorID {
		return apperrors.NewForbidden("only the item author can submit for review")
	}

	// Validate content completeness before review
	if existing.Content.Stem == "" {
		return apperrors.NewValidation("item not ready for review", map[string]string{
			"stem": "question stem is required",
		})
	}
	if existing.Type == item.ItemTypeMCQSingle && len(existing.Content.Options) < 2 {
		return apperrors.NewValidation("item not ready for review", map[string]string{
			"options": "MCQ items require at least 2 options",
		})
	}

	previousStatus := existing.Status
	if err := existing.TransitionTo(item.ItemStatusReview, actorID); err != nil {
		return err
	}

	if err := s.repo.Update(ctx, existing); err != nil {
		return err
	}

	s.publishAudit(ctx, "ITEM_SUBMITTED_FOR_REVIEW", actorID, "item", itemID, orgID, "TRANSITION", map[string]interface{}{
		"from_status": string(previousStatus),
		"to_status":   string(item.ItemStatusReview),
	})

	return nil
}

// ReviewItemRequest holds the parameters for recording a review decision.
type ReviewItemRequest struct {
	ReviewerID uuid.UUID
	Decision   item.ReviewDecision
	Comments   string
}

// ReviewItem records a review decision and transitions the item accordingly.
// Enforces separation of duties: reviewer ≠ author.
func (s *ItemService) ReviewItem(ctx context.Context, orgID, itemID uuid.UUID, req ReviewItemRequest) error {
	existing, err := s.repo.GetByID(ctx, orgID, itemID)
	if err != nil {
		return err
	}

	if existing.Status != item.ItemStatusReview {
		return apperrors.NewConflict(fmt.Sprintf("item must be in REVIEW status to review, current: %s", existing.Status))
	}

	// Separation of duties: reviewer cannot be the author
	if existing.Approval.AuthorID == req.ReviewerID {
		return apperrors.NewForbidden("separation of duties violation: reviewer cannot be the item author")
	}

	// Record review
	now := time.Now().UTC()
	existing.Approval.ReviewerID = &req.ReviewerID
	existing.Approval.ReviewerDecision = req.Decision
	existing.Approval.ReviewedAt = &now

	// Transition based on decision
	var targetStatus item.ItemStatus
	switch req.Decision {
	case item.ReviewApproved:
		targetStatus = item.ItemStatusCalibration
	case item.ReviewRejected, item.ReviewRevision:
		targetStatus = item.ItemStatusDraft // Send back for revision
	default:
		return apperrors.NewBadRequest("invalid review decision")
	}

	if err := existing.TransitionTo(targetStatus, req.ReviewerID); err != nil {
		return err
	}

	if err := s.repo.Update(ctx, existing); err != nil {
		return err
	}

	s.publishAudit(ctx, "ITEM_REVIEWED", req.ReviewerID, "item", itemID, orgID, "TRANSITION", map[string]interface{}{
		"decision":  string(req.Decision),
		"to_status": string(targetStatus),
		"comments":  req.Comments,
	})

	return nil
}

// CalibrateItemRequest holds IRT calibration parameters.
type CalibrateItemRequest struct {
	PsychometricianID uuid.UUID
	Params            item.IRTParameters
}

// CalibrateItem sets the IRT parameters on an item after psychometric review.
// Enforces separation of duties: psychometrician ≠ author ≠ reviewer.
func (s *ItemService) CalibrateItem(ctx context.Context, orgID, itemID uuid.UUID, req CalibrateItemRequest) error {
	existing, err := s.repo.GetByID(ctx, orgID, itemID)
	if err != nil {
		return err
	}

	if existing.Status != item.ItemStatusCalibration {
		return apperrors.NewConflict(fmt.Sprintf("item must be in CALIBRATION status, current: %s", existing.Status))
	}

	// Separation of duties
	if existing.Approval.AuthorID == req.PsychometricianID {
		return apperrors.NewForbidden("separation of duties: psychometrician cannot be the author")
	}
	if existing.Approval.ReviewerID != nil && *existing.Approval.ReviewerID == req.PsychometricianID {
		return apperrors.NewForbidden("separation of duties: psychometrician cannot be the reviewer")
	}

	// Set IRT parameters (validates internally)
	if err := existing.SetIRTParameters(req.Params); err != nil {
		return err
	}

	// Record psychometrician
	existing.Approval.PsychometricianID = &req.PsychometricianID

	// Sign calibration
	calibData, _ := json.Marshal(req.Params)
	sig, err := s.signingSvc.Sign(calibData, "aegis-psychometrician-signing")
	if err != nil {
		s.logger.Warn("failed to sign calibration", zap.Error(err))
	} else {
		existing.Approval.PsychometricianSig = sig
	}

	// Transition to PILOT (or directly to ACTIVE if pilot is skipped)
	if err := existing.TransitionTo(item.ItemStatusPilot, req.PsychometricianID); err != nil {
		return err
	}

	if err := s.repo.Update(ctx, existing); err != nil {
		return err
	}

	// Record version
	newData, _ := json.Marshal(existing.IRTParams)
	s.recordVersion(ctx, existing.ID, existing.Version, item.ChangeTypeIRTUpdate, nil, newData, req.PsychometricianID, "IRT calibration")

	s.publishAudit(ctx, "ITEM_CALIBRATED", req.PsychometricianID, "item", itemID, orgID, "CALIBRATE", map[string]interface{}{
		"irt_a":       req.Params.A,
		"irt_b":       req.Params.B,
		"irt_c":       req.Params.C,
		"sample_size": req.Params.CalibrationSampleSize,
	})

	return nil
}

// ActivateItem transitions a PILOT item to ACTIVE status.
func (s *ItemService) ActivateItem(ctx context.Context, orgID, itemID, approverID uuid.UUID) error {
	existing, err := s.repo.GetByID(ctx, orgID, itemID)
	if err != nil {
		return err
	}

	// Separation of duties: approver ≠ author ≠ reviewer ≠ psychometrician
	existing.Approval.ApproverID = &approverID
	if err := existing.Approval.ValidateSeparationOfDuties(); err != nil {
		return apperrors.NewForbidden(err.Error())
	}

	// Final approval signature
	now := time.Now().UTC()
	existing.Approval.ApprovedAt = &now
	approvalData := fmt.Sprintf("%s:%s:%d", existing.ID, approverID, existing.Version)
	sig, err := s.signingSvc.Sign([]byte(approvalData), "aegis-approver-signing")
	if err == nil {
		existing.Approval.ApproverSignature = sig
	}

	if err := existing.TransitionTo(item.ItemStatusActive, approverID); err != nil {
		return err
	}

	if err := s.repo.Update(ctx, existing); err != nil {
		return err
	}

	s.publishAudit(ctx, "ITEM_ACTIVATED", approverID, "item", itemID, orgID, "TRANSITION", map[string]interface{}{
		"approval_chain": map[string]string{
			"author":          existing.Approval.AuthorID.String(),
			"reviewer":        uuidToString(existing.Approval.ReviewerID),
			"psychometrician": uuidToString(existing.Approval.PsychometricianID),
			"approver":        approverID.String(),
		},
	})

	return nil
}

// RetireItem transitions an ACTIVE or SUSPENDED item to RETIRED.
func (s *ItemService) RetireItem(ctx context.Context, orgID, itemID, actorID uuid.UUID, reason string) error {
	existing, err := s.repo.GetByID(ctx, orgID, itemID)
	if err != nil {
		return err
	}

	previousStatus := existing.Status
	if err := existing.TransitionTo(item.ItemStatusRetired, actorID); err != nil {
		return err
	}

	if err := s.repo.Update(ctx, existing); err != nil {
		return err
	}

	s.publishAudit(ctx, "ITEM_RETIRED", actorID, "item", itemID, orgID, "TRANSITION", map[string]interface{}{
		"from_status": string(previousStatus),
		"reason":      reason,
		"exposure":    existing.Exposure.ExposureCount,
	})

	return nil
}

// GetVersionHistory returns the complete version history of an item.
func (s *ItemService) GetVersionHistory(ctx context.Context, itemID uuid.UUID) ([]item.ItemVersion, error) {
	return s.repo.GetVersionHistory(ctx, itemID)
}

// ──────────────────────────────────────────────
//  Internal Helpers
// ──────────────────────────────────────────────

// recordVersion creates a version history entry with a digital signature.
func (s *ItemService) recordVersion(ctx context.Context, itemID uuid.UUID, version int, changeType item.ChangeType, prev, next json.RawMessage, changedBy uuid.UUID, reason string) {
	versionEntry := &item.ItemVersion{
		ID:           uuid.New(),
		ItemID:       itemID,
		Version:      version,
		ChangeType:   changeType,
		PreviousData: prev,
		NewData:      next,
		ChangedBy:    changedBy,
		ChangedAt:    time.Now().UTC(),
		ChangeReason: reason,
	}

	// Sign the version entry
	sigData := fmt.Sprintf("%s:%d:%s", itemID, version, string(next))
	sig, err := s.signingSvc.Sign([]byte(sigData), "aegis-version-signing")
	if err != nil {
		s.logger.Warn("failed to sign version entry", zap.Error(err))
		versionEntry.DigitalSig = []byte{} // Empty sig — logged but not fatal
	} else {
		versionEntry.DigitalSig = sig
	}

	if err := s.repo.CreateVersion(ctx, versionEntry); err != nil {
		s.logger.Error("failed to record version history",
			zap.String("item_id", itemID.String()),
			zap.Int("version", version),
			zap.Error(err),
		)
	}
}

// publishAudit publishes an audit event. Failures are logged but do not block the operation.
func (s *ItemService) publishAudit(ctx context.Context, eventType string, actorID uuid.UUID, resourceType string, resourceID, orgID uuid.UUID, action string, detail map[string]interface{}) {
	if s.auditSvc == nil {
		return
	}

	entry := &audit.Entry{
		EventTime:      time.Now().UTC(),
		EventType:      eventType,
		ActorID:        actorID,
		ActorType:      audit.ActorTypeUser,
		ResourceType:   resourceType,
		ResourceID:     resourceID,
		OrganizationID: orgID,
		Action:         action,
		Detail:         detail,
	}

	if err := s.auditSvc.Append(ctx, entry); err != nil {
		s.logger.Error("failed to publish audit event",
			zap.String("event_type", eventType),
			zap.Error(err),
		)
	}
}

// uuidToString safely converts a *uuid.UUID to string, returning "" for nil.
func uuidToString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}
