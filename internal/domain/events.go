package domain

import (
	"time"

	"github.com/google/uuid"
)

// TopicInspectEvents carries every domain event produced by inspect-service.
// One topic per producing service; events are told apart by EventType (ADR-007).
const TopicInspectEvents = "inspect.events"

// EventTypeInspectCompleted is emitted when an inspection is completed.
const EventTypeInspectCompleted = "inspect.completed"

// EventVersion is the envelope schema version. Additive changes keep it; a
// breaking change bumps it.
const EventVersion = 1

// EventEnvelope is the JSON wire format for every event. RequestID is the
// correlation/message key so consumers stay ordered per request; Data holds the
// event-type-specific payload.
type EventEnvelope struct {
	EventID    uuid.UUID `json:"event_id"`
	EventType  string    `json:"event_type"`
	Version    int       `json:"version"`
	OccurredAt time.Time `json:"occurred_at"`
	RequestID  uuid.UUID `json:"request_id"`
	Data       any       `json:"data"`
}

// InspectCompletedData is the payload for EventTypeInspectCompleted. It carries
// enough for request-service to advance the request and notification-service to
// notify the client.
type InspectCompletedData struct {
	InspectionID uuid.UUID  `json:"inspection_id"`
	InspectorID  *uuid.UUID `json:"inspector_id,omitempty"`
	CompletedAt  time.Time  `json:"completed_at"`
	PhotoCount   int        `json:"photo_count"`
}

// NewInspectCompletedEvent builds the envelope for a finished inspection.
func NewInspectCompletedEvent(insp Inspection, photoCount int) EventEnvelope {
	completedAt := insp.UpdatedAt
	if insp.CompletedAt != nil {
		completedAt = *insp.CompletedAt
	}
	return EventEnvelope{
		EventID:    uuid.New(),
		EventType:  EventTypeInspectCompleted,
		Version:    EventVersion,
		OccurredAt: time.Now().UTC(),
		RequestID:  insp.RequestID,
		Data: InspectCompletedData{
			InspectionID: insp.ID,
			InspectorID:  insp.InspectorID,
			CompletedAt:  completedAt,
			PhotoCount:   photoCount,
		},
	}
}
