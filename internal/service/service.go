package service

import (
	"context"
	"encoding/json"

	"github.com/appraisal-crm/inspect-service/internal/domain"
	"github.com/google/uuid"
)

// UpdateInput carries the mutable fields. Nil pointers / nil JSON mean "leave as
// is", so a caller can patch one field without clobbering the others.
type UpdateInput struct {
	InspectorID  *uuid.UUID
	Notes        *string
	PropertyData json.RawMessage
}

// InspectionService is the business-logic contract used by the HTTP handlers and
// the Kafka consumer.
type InspectionService interface {
	// CreateFromRequest is driven by the request.status_changed consumer (step 9)
	// when a request enters inspection_scheduled. Idempotent per request.
	CreateFromRequest(ctx context.Context, requestID uuid.UUID) (created bool, err error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Inspection, error)
	Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*domain.Inspection, error)
	Complete(ctx context.Context, id uuid.UUID) (*domain.Inspection, error)
	ListByInspectorID(ctx context.Context, inspectorID uuid.UUID) ([]*domain.Inspection, error)
	ListAll(ctx context.Context, limit, offset int) ([]*domain.Inspection, error)
}
