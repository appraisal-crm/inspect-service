package repository

import (
	"context"
	"errors"
	"time"

	"github.com/appraisal-crm/inspect-service/internal/domain"
	"github.com/google/uuid"
)

// Repository-level errors. The service maps these to domain errors, which the
// handler maps to HTTP codes.
var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("concurrent modification conflict")
)

// InspectionRepository is the persistence contract. The service depends on this
// interface, so tests can swap a mock for real Postgres.
type InspectionRepository interface {
	// Create inserts an inspection. Idempotent on request_id: a second insert for
	// the same request is a no-op (created=false), so a redelivered event never
	// doubles the row.
	Create(ctx context.Context, insp *domain.Inspection) (created bool, err error)

	GetByID(ctx context.Context, id uuid.UUID) (*domain.Inspection, error)

	// Update changes inspector_id/notes/property_data only (never status).
	// prevUpdatedAt is the optimistic-lock guard.
	Update(ctx context.Context, insp *domain.Inspection, prevUpdatedAt time.Time) error

	// Complete flips status scheduled→completed and writes the event to the outbox
	// in the SAME transaction.
	Complete(ctx context.Context, id uuid.UUID, completedAt, updatedAt time.Time, event domain.EventEnvelope) error

	AddPhoto(ctx context.Context, photo *domain.Photo) error

	ListByInspectorID(ctx context.Context, inspectorID uuid.UUID) ([]*domain.Inspection, error)
	ListAll(ctx context.Context, limit, offset int) ([]*domain.Inspection, error)
}
