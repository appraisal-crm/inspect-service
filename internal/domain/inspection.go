package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Status is the inspection lifecycle. Only two states, one direction.
type Status string

const (
	StatusScheduled Status = "scheduled"
	StatusCompleted Status = "completed"
)

// Inspection is the aggregate: one on-site visit for one request. It is created
// when a request is scheduled (Kafka consumer, step 9), filled in by the assigned
// inspector, and closed with Complete — which emits inspect.completed (step 8).
type Inspection struct {
	ID           uuid.UUID       `json:"id"`
	RequestID    uuid.UUID       `json:"request_id"`
	InspectorID  *uuid.UUID      `json:"inspector_id,omitempty"` // nil until an appraiser assigns one
	Status       Status          `json:"status"`
	Notes        *string         `json:"notes,omitempty"`
	PropertyData json.RawMessage `json:"property_data,omitempty" swaggertype:"object"` // free-form object data
	Photos       []Photo         `json:"photos,omitempty"`                             // loaded on demand, not a column
	ScheduledAt  time.Time       `json:"scheduled_at"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// Photo is one image attached to an inspection. The binary lives in S3; we keep
// only the object key here.
type Photo struct {
	ID           uuid.UUID `json:"id"`
	InspectionID uuid.UUID `json:"inspection_id"`
	S3Key        string    `json:"s3_key"`
	UploadedAt   time.Time `json:"uploaded_at"`
}
