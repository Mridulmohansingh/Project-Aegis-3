package handler

import (
	"context"

	"github.com/google/uuid"

	"github.com/aegis-platform/aegis/internal/domain/item"
	"github.com/aegis-platform/aegis/internal/service"
)

// ItemServiceAdapter adapts service.ItemService to handler.ItemService.
type ItemServiceAdapter struct {
	svc *service.ItemService
}

// NewItemServiceAdapter creates a new ItemServiceAdapter.
func NewItemServiceAdapter(svc *service.ItemService) *ItemServiceAdapter {
	return &ItemServiceAdapter{svc: svc}
}

func (a *ItemServiceAdapter) CreateItem(ctx interface{}, req CreateItemRequest) (*item.Item, error) {
	c, ok := ctx.(context.Context)
	if !ok {
		c = context.Background()
	}

	subjID, _ := uuid.Parse(req.SubjectID)
	chapID, _ := uuid.Parse(req.ChapterID)
	topID, _ := uuid.Parse(req.TopicID)

	var subTopUUID *uuid.UUID
	if req.SubTopicID != nil {
		if id, err := uuid.Parse(*req.SubTopicID); err == nil {
			subTopUUID = &id
		}
	}

	var loUUID *uuid.UUID
	if req.LearningOutcomeID != nil {
		if id, err := uuid.Parse(*req.LearningOutcomeID); err == nil {
			loUUID = &id
		}
	}

	diffLevel := item.DifficultyMedium
	if req.DifficultyLevel != nil {
		diffLevel = item.DifficultyLevel(*req.DifficultyLevel)
	}

	cogLevel := item.CognitiveLevelApply
	if req.CognitiveLevel != nil {
		cogLevel = item.CognitiveLevel(*req.CognitiveLevel)
	}

	estTime := 120
	if req.EstimatedTimeSecs != nil {
		estTime = *req.EstimatedTimeSecs
	}

	lang := "en"
	if req.PrimaryLanguage != nil {
		lang = *req.PrimaryLanguage
	}

	return a.svc.CreateItem(c, service.CreateItemRequest{
		ExternalID:        req.ExternalID,
		ItemType:          item.ItemType(req.ItemType),
		OrganizationID:    uuid.New(),
		SubjectID:         subjID,
		ChapterID:         chapID,
		TopicID:           topID,
		SubTopicID:        subTopUUID,
		LearningOutcomeID: loUUID,
		Content:           req.Content,
		MarkingScheme:     req.MarkingScheme,
		DifficultyLevel:   diffLevel,
		CognitiveLevel:    cogLevel,
		EstimatedTimeSecs: estTime,
		PrimaryLanguage:   lang,
		Tags:              req.Tags,
		AuthorID:          uuid.New(),
	})
}

func (a *ItemServiceAdapter) GetItem(ctx interface{}, orgID, itemID uuid.UUID) (*item.Item, error) {
	c, ok := ctx.(context.Context)
	if !ok {
		c = context.Background()
	}
	return a.svc.GetItem(c, orgID, itemID)
}

func (a *ItemServiceAdapter) UpdateItem(ctx interface{}, orgID, itemID uuid.UUID, req UpdateItemRequest) (*item.Item, error) {
	c, ok := ctx.(context.Context)
	if !ok {
		c = context.Background()
	}

	var diffLevel *item.DifficultyLevel
	if req.DifficultyLevel != nil {
		dl := item.DifficultyLevel(*req.DifficultyLevel)
		diffLevel = &dl
	}

	var cogLevel *item.CognitiveLevel
	if req.CognitiveLevel != nil {
		cl := item.CognitiveLevel(*req.CognitiveLevel)
		cogLevel = &cl
	}

	return a.svc.UpdateItem(c, orgID, itemID, service.UpdateItemRequest{
		Content:           req.Content,
		MarkingScheme:     req.MarkingScheme,
		DifficultyLevel:   diffLevel,
		CognitiveLevel:    cogLevel,
		EstimatedTimeSecs: req.EstimatedTimeSecs,
		Tags:              req.Tags,
		ExpectedVersion:   req.Version,
		ActorID:           uuid.New(),
	})
}

func (a *ItemServiceAdapter) DeleteItem(ctx interface{}, orgID, itemID uuid.UUID, actorID uuid.UUID) error {
	c, ok := ctx.(context.Context)
	if !ok {
		c = context.Background()
	}
	return a.svc.DeleteItem(c, orgID, itemID, actorID)
}

func (a *ItemServiceAdapter) ListItems(ctx interface{}, filter item.ItemFilter, cursor string, limit int) ([]*item.Item, string, error) {
	c, ok := ctx.(context.Context)
	if !ok {
		c = context.Background()
	}
	return a.svc.ListItems(c, filter, cursor, limit)
}

func (a *ItemServiceAdapter) SubmitForReview(ctx interface{}, orgID, itemID uuid.UUID, actorID uuid.UUID) error {
	c, ok := ctx.(context.Context)
	if !ok {
		c = context.Background()
	}
	return a.svc.SubmitForReview(c, orgID, itemID, actorID)
}

func (a *ItemServiceAdapter) ReviewItem(ctx interface{}, orgID, itemID uuid.UUID, req ReviewItemRequest) error {
	c, ok := ctx.(context.Context)
	if !ok {
		c = context.Background()
	}
	revID, _ := uuid.Parse(req.ReviewerID)
	return a.svc.ReviewItem(c, orgID, itemID, service.ReviewItemRequest{
		ReviewerID: revID,
		Decision:   item.ReviewDecision(req.Decision),
		Comments:   req.Comments,
	})
}

func (a *ItemServiceAdapter) CalibrateItem(ctx interface{}, orgID, itemID uuid.UUID, req CalibrateItemRequest) error {
	c, ok := ctx.(context.Context)
	if !ok {
		c = context.Background()
	}
	psychID, _ := uuid.Parse(req.PsychometricianID)
	return a.svc.CalibrateItem(c, orgID, itemID, service.CalibrateItemRequest{
		PsychometricianID: psychID,
		Params: item.IRTParameters{
			A:                     req.A,
			B:                     req.B,
			C:                     req.C,
			SEA:                   req.SEA,
			SEB:                   req.SEB,
			SEC:                   req.SEC,
			CalibrationSampleSize: req.CalibrationSample,
		},
	})
}
