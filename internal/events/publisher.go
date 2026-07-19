// Package events provides Kafka-based event publishing for AEGIS.
//
// Events are published for cross-service communication and audit purposes.
// All events are serialized as JSON with a standard envelope containing
// event metadata, correlation IDs, and the domain payload.
//
// Events are published asynchronously with at-least-once delivery guarantees.
// Consumers must be idempotent.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ──────────────────────────────────────────────
//  Event Types
// ──────────────────────────────────────────────

// Topic constants for Kafka topics.
const (
	TopicItemEvents      = "aegis.items"
	TopicExamEvents      = "aegis.exams"
	TopicResponseEvents  = "aegis.responses"
	TopicScoringEvents   = "aegis.scoring"
	TopicAuditEvents     = "aegis.audit"
	TopicNotifications   = "aegis.notifications"
)

// EventType identifies the type of domain event.
type EventType string

const (
	// Item lifecycle events
	EventItemCreated     EventType = "ITEM_CREATED"
	EventItemUpdated     EventType = "ITEM_UPDATED"
	EventItemSubmitted   EventType = "ITEM_SUBMITTED_FOR_REVIEW"
	EventItemReviewed    EventType = "ITEM_REVIEWED"
	EventItemCalibrated  EventType = "ITEM_CALIBRATED"
	EventItemActivated   EventType = "ITEM_ACTIVATED"
	EventItemRetired     EventType = "ITEM_RETIRED"
	EventItemFlagged     EventType = "ITEM_FLAGGED_FOR_REVIEW"

	// Exam lifecycle events
	EventExamCreated         EventType = "EXAM_CREATED"
	EventExamConfigured      EventType = "EXAM_CONFIGURED"
	EventPapersGenerated     EventType = "PAPERS_GENERATED"
	EventExamScheduled       EventType = "EXAM_SCHEDULED"
	EventExamStarted         EventType = "EXAM_STARTED"
	EventExamCompleted       EventType = "EXAM_COMPLETED"

	// Session events
	EventSessionInitialized  EventType = "SESSION_INITIALIZED"
	EventSessionStarted      EventType = "SESSION_STARTED"
	EventSessionCompleted    EventType = "SESSION_COMPLETED"
	EventSessionTimedOut     EventType = "SESSION_TIMED_OUT"
	EventSessionTerminated   EventType = "SESSION_TERMINATED"

	// Scoring events
	EventScoringStarted      EventType = "SCORING_STARTED"
	EventScoringCompleted    EventType = "SCORING_COMPLETED"
	EventResultsPublished    EventType = "RESULTS_PUBLISHED"

	// Analysis events
	EventAnalysisCompleted   EventType = "ANALYSIS_COMPLETED"
	EventDIFDetected         EventType = "DIF_DETECTED"
	EventPersonFitFlagged    EventType = "PERSON_FIT_FLAGGED"
)

// ──────────────────────────────────────────────
//  Event Envelope
// ──────────────────────────────────────────────

// Event is the standard envelope for all domain events.
type Event struct {
	// ID is a unique event identifier for deduplication.
	ID string `json:"id"`
	// Type identifies the event type.
	Type EventType `json:"type"`
	// Source identifies the originating service.
	Source string `json:"source"`
	// Time is the event timestamp (ISO 8601).
	Time string `json:"time"`
	// CorrelationID links related events across services.
	CorrelationID string `json:"correlation_id"`
	// OrganizationID scopes the event to a tenant.
	OrganizationID string `json:"organization_id"`
	// ActorID identifies who triggered the event.
	ActorID string `json:"actor_id"`
	// Data contains the event-specific payload.
	Data json.RawMessage `json:"data"`
}

// NewEvent creates a new event with standard metadata.
func NewEvent(eventType EventType, source string, orgID, actorID uuid.UUID, data interface{}) (*Event, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshaling event data: %w", err)
	}

	return &Event{
		ID:             uuid.New().String(),
		Type:           eventType,
		Source:         source,
		Time:           time.Now().UTC().Format(time.RFC3339Nano),
		CorrelationID:  uuid.New().String(),
		OrganizationID: orgID.String(),
		ActorID:        actorID.String(),
		Data:           payload,
	}, nil
}

// ──────────────────────────────────────────────
//  Publisher Interface
// ──────────────────────────────────────────────

// Publisher defines the event publishing interface.
// Implementations may use Kafka, NATS, RabbitMQ, or in-memory channels.
type Publisher interface {
	// Publish sends an event to the specified topic.
	// The key is used for partition assignment (e.g., exam ID ensures ordering per exam).
	Publish(ctx context.Context, topic string, key string, event *Event) error

	// PublishBatch sends multiple events to the specified topic atomically.
	PublishBatch(ctx context.Context, topic string, events []KeyedEvent) error

	// Close gracefully shuts down the publisher, flushing pending messages.
	Close() error
}

// KeyedEvent pairs an event with its partition key.
type KeyedEvent struct {
	Key   string
	Event *Event
}

// ──────────────────────────────────────────────
//  In-Memory Publisher (Development/Testing)
// ──────────────────────────────────────────────

// InMemoryPublisher implements Publisher using in-memory channels.
// FOR DEVELOPMENT AND TESTING ONLY.
type InMemoryPublisher struct {
	events map[string][]*Event // topic → events
	logger *zap.Logger
}

// NewInMemoryPublisher creates a new in-memory event publisher.
func NewInMemoryPublisher(logger *zap.Logger) *InMemoryPublisher {
	return &InMemoryPublisher{
		events: make(map[string][]*Event),
		logger: logger.With(zap.String("component", "in_memory_publisher")),
	}
}

// Publish stores an event in memory.
func (p *InMemoryPublisher) Publish(ctx context.Context, topic string, key string, event *Event) error {
	p.events[topic] = append(p.events[topic], event)
	p.logger.Debug("event published (in-memory)",
		zap.String("topic", topic),
		zap.String("key", key),
		zap.String("event_type", string(event.Type)),
		zap.String("event_id", event.ID),
	)
	return nil
}

// PublishBatch stores multiple events in memory.
func (p *InMemoryPublisher) PublishBatch(ctx context.Context, topic string, events []KeyedEvent) error {
	for _, ke := range events {
		if err := p.Publish(ctx, topic, ke.Key, ke.Event); err != nil {
			return err
		}
	}
	return nil
}

// Close is a no-op for in-memory publisher.
func (p *InMemoryPublisher) Close() error {
	return nil
}

// GetEvents returns all events published to a topic (for testing).
func (p *InMemoryPublisher) GetEvents(topic string) []*Event {
	return p.events[topic]
}

// ClearEvents removes all stored events (for testing).
func (p *InMemoryPublisher) ClearEvents() {
	p.events = make(map[string][]*Event)
}

// ──────────────────────────────────────────────
//  Event Payloads
// ──────────────────────────────────────────────

// ItemEventData is the payload for item-related events.
type ItemEventData struct {
	ItemID      string `json:"item_id"`
	ExternalID  string `json:"external_id"`
	Status      string `json:"status"`
	PrevStatus  string `json:"prev_status,omitempty"`
	SubjectID   string `json:"subject_id"`
	ChangeType  string `json:"change_type,omitempty"`
	Version     int    `json:"version"`
}

// ExamEventData is the payload for exam-related events.
type ExamEventData struct {
	ExamID     string `json:"exam_id"`
	ExamCode   string `json:"exam_code"`
	Status     string `json:"status"`
	PrevStatus string `json:"prev_status,omitempty"`
	FormCount  int    `json:"form_count,omitempty"`
}

// SessionEventData is the payload for session-related events.
type SessionEventData struct {
	SessionID   string `json:"session_id"`
	ExamID      string `json:"exam_id"`
	CandidateID string `json:"candidate_id"`
	Status      string `json:"status"`
	Responses   int    `json:"responses,omitempty"`
	DurationSec int    `json:"duration_sec,omitempty"`
}

// ScoringEventData is the payload for scoring-related events.
type ScoringEventData struct {
	ExamID          string  `json:"exam_id"`
	CandidatesScored int   `json:"candidates_scored"`
	MeanScore       float64 `json:"mean_score,omitempty"`
	ScoringVersion  string  `json:"scoring_version"`
}

// AnalysisEventData is the payload for analysis-related events.
type AnalysisEventData struct {
	ExamID          string   `json:"exam_id"`
	ItemsFlagged    int      `json:"items_flagged"`
	DIFDetected     int      `json:"dif_detected"`
	PersonFitFlags  int      `json:"person_fit_flags"`
	FlaggedItemIDs  []string `json:"flagged_item_ids,omitempty"`
}
