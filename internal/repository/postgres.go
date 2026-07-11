package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/appraisal-crm/inspect-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRepository returns the interface, not the concrete type — callers
// depend on the contract only.
func NewPostgresRepository(db *pgxpool.Pool) InspectionRepository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Create(ctx context.Context, insp *domain.Inspection) (bool, error) {
	// ON CONFLICT (request_id): one inspection per request. A redelivered creation
	// event matches nothing new, so RowsAffected() is 0 → created=false.
	query := `
		INSERT INTO inspections (id, request_id, inspector_id, status, notes, property_data, scheduled_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (request_id) DO NOTHING
	`
	tag, err := r.db.Exec(ctx, query,
		insp.ID, insp.RequestID, insp.InspectorID, insp.Status, insp.Notes,
		insp.PropertyData, insp.ScheduledAt, insp.CreatedAt, insp.UpdatedAt,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *postgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Inspection, error) {
	query := `
		SELECT id, request_id, inspector_id, status, notes, property_data, scheduled_at, completed_at, created_at, updated_at
		FROM inspections
		WHERE id = $1
	`
	var insp domain.Inspection
	err := r.db.QueryRow(ctx, query, id).Scan(
		&insp.ID, &insp.RequestID, &insp.InspectorID, &insp.Status, &insp.Notes,
		&insp.PropertyData, &insp.ScheduledAt, &insp.CompletedAt, &insp.CreatedAt, &insp.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	photos, err := r.listPhotos(ctx, id)
	if err != nil {
		return nil, err
	}
	insp.Photos = photos
	return &insp, nil
}

func (r *postgresRepository) listPhotos(ctx context.Context, inspectionID uuid.UUID) ([]domain.Photo, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, inspection_id, s3_key, uploaded_at
		FROM inspection_photos
		WHERE inspection_id = $1
		ORDER BY uploaded_at
	`, inspectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	photos := make([]domain.Photo, 0)
	for rows.Next() {
		var p domain.Photo
		if err := rows.Scan(&p.ID, &p.InspectionID, &p.S3Key, &p.UploadedAt); err != nil {
			return nil, err
		}
		photos = append(photos, p)
	}
	return photos, rows.Err()
}

// Update sets inspector_id, notes and property_data. It never touches status —
// status changes go through Complete only. Optimistic lock: updated_at must match.
func (r *postgresRepository) Update(ctx context.Context, insp *domain.Inspection, prevUpdatedAt time.Time) error {
	query := `
		UPDATE inspections
		SET inspector_id = $1, notes = $2, property_data = $3, updated_at = $4
		WHERE id = $5 AND updated_at = $6
	`
	tag, err := r.db.Exec(ctx, query,
		insp.InspectorID, insp.Notes, insp.PropertyData, insp.UpdatedAt, insp.ID, prevUpdatedAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return r.notFoundOrConflict(ctx, insp.ID)
	}
	return nil
}

func (r *postgresRepository) Complete(ctx context.Context, id uuid.UUID, completedAt, updatedAt time.Time, event domain.EventEnvelope) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) // no-op after a successful Commit

	// CAS guard: only a scheduled inspection can be completed. 0 rows → gone or
	// already completed; the tx rolls back, so no outbox row is written.
	tag, err := tx.Exec(ctx, `
		UPDATE inspections
		SET status = $1, completed_at = $2, updated_at = $3
		WHERE id = $4 AND status = $5
	`, domain.StatusCompleted, completedAt, updatedAt, id, domain.StatusScheduled)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM inspections WHERE id = $1)", id).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		return ErrConflict
	}

	if err := insertOutbox(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// insertOutbox writes the event into the outbox within the caller's tx, so the
// event and the state change commit atomically. aggregate_id = request_id keeps
// inspect.events ordered per request.
func insertOutbox(ctx context.Context, tx pgx.Tx, event domain.EventEnvelope) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO outbox (event_id, topic, event_type, aggregate_id, payload)
		VALUES ($1, $2, $3, $4, $5)
	`, event.EventID, domain.TopicInspectEvents, event.EventType, event.RequestID, payload)
	return err
}

func (r *postgresRepository) AddPhoto(ctx context.Context, photo *domain.Photo) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO inspection_photos (id, inspection_id, s3_key, uploaded_at)
		VALUES ($1, $2, $3, $4)
	`, photo.ID, photo.InspectionID, photo.S3Key, photo.UploadedAt)
	return err
}

func (r *postgresRepository) ListByInspectorID(ctx context.Context, inspectorID uuid.UUID) ([]*domain.Inspection, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, request_id, inspector_id, status, notes, property_data, scheduled_at, completed_at, created_at, updated_at
		FROM inspections
		WHERE inspector_id = $1
		ORDER BY scheduled_at DESC
	`, inspectorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanInspections(rows)
}

func (r *postgresRepository) ListAll(ctx context.Context, limit, offset int) ([]*domain.Inspection, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, request_id, inspector_id, status, notes, property_data, scheduled_at, completed_at, created_at, updated_at
		FROM inspections
		ORDER BY scheduled_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanInspections(rows)
}

// scanInspections reads list rows (without photos — lists stay light).
func scanInspections(rows pgx.Rows) ([]*domain.Inspection, error) {
	inspections := make([]*domain.Inspection, 0)
	for rows.Next() {
		var insp domain.Inspection
		if err := rows.Scan(
			&insp.ID, &insp.RequestID, &insp.InspectorID, &insp.Status, &insp.Notes,
			&insp.PropertyData, &insp.ScheduledAt, &insp.CompletedAt, &insp.CreatedAt, &insp.UpdatedAt,
		); err != nil {
			return nil, err
		}
		inspections = append(inspections, &insp)
	}
	return inspections, rows.Err()
}

// notFoundOrConflict disambiguates a 0-row UPDATE: missing row → ErrNotFound,
// otherwise a concurrent modification → ErrConflict.
func (r *postgresRepository) notFoundOrConflict(ctx context.Context, id uuid.UUID) error {
	var exists bool
	if err := r.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM inspections WHERE id = $1)", id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return ErrConflict
}
