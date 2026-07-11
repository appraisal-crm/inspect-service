package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/appraisal-crm/inspect-service/internal/domain"
	"github.com/appraisal-crm/inspect-service/internal/repository"
	"github.com/google/uuid"
)

type inspectionService struct {
	repo repository.InspectionRepository
}

func NewInspectionService(repo repository.InspectionRepository) InspectionService {
	return &inspectionService{repo: repo}
}

func (s *inspectionService) CreateFromRequest(ctx context.Context, requestID uuid.UUID) (bool, error) {
	now := time.Now()
	insp := &domain.Inspection{
		ID:          uuid.New(),
		RequestID:   requestID,
		Status:      domain.StatusScheduled,
		ScheduledAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	created, err := s.repo.Create(ctx, insp)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create inspection", "error", err, "request_id", requestID)
		return false, err
	}
	if created {
		slog.InfoContext(ctx, "inspection created", "inspection_id", insp.ID, "request_id", requestID)
	} else {
		slog.InfoContext(ctx, "inspection already exists for request, skipping", "request_id", requestID)
	}
	return created, nil
}

func (s *inspectionService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Inspection, error) {
	insp, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, domain.ErrNotFound
		}
		slog.ErrorContext(ctx, "failed to get inspection", "error", err, "inspection_id", id)
		return nil, err
	}
	return insp, nil
}

func (s *inspectionService) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*domain.Inspection, error) {
	insp, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	// Patch only the fields the caller provided.
	if in.InspectorID != nil {
		insp.InspectorID = in.InspectorID
	}
	if in.Notes != nil {
		insp.Notes = in.Notes
	}
	if in.PropertyData != nil {
		insp.PropertyData = in.PropertyData
	}

	prevUpdatedAt := insp.UpdatedAt
	insp.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, insp, prevUpdatedAt); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, domain.ErrNotFound
		}
		if errors.Is(err, repository.ErrConflict) {
			slog.WarnContext(ctx, "concurrent update detected", "inspection_id", id)
			return nil, domain.ErrConflict
		}
		slog.ErrorContext(ctx, "failed to update inspection", "error", err, "inspection_id", id)
		return nil, err
	}
	slog.InfoContext(ctx, "inspection updated", "inspection_id", id)
	return insp, nil
}

func (s *inspectionService) Complete(ctx context.Context, id uuid.UUID) (*domain.Inspection, error) {
	insp, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	// State machine: only scheduled → completed is allowed.
	if insp.Status != domain.StatusScheduled {
		slog.WarnContext(ctx, "cannot complete inspection in its current status", "inspection_id", id, "status", insp.Status)
		return nil, domain.ErrInvalidStatus
	}

	completedAt := time.Now()
	insp.Status = domain.StatusCompleted
	insp.CompletedAt = &completedAt
	insp.UpdatedAt = completedAt
	event := domain.NewInspectCompletedEvent(*insp, len(insp.Photos))

	if err := s.repo.Complete(ctx, id, completedAt, completedAt, event); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, domain.ErrNotFound
		}
		if errors.Is(err, repository.ErrConflict) {
			slog.WarnContext(ctx, "concurrent completion detected", "inspection_id", id)
			return nil, domain.ErrConflict
		}
		slog.ErrorContext(ctx, "failed to complete inspection", "error", err, "inspection_id", id)
		return nil, err
	}

	slog.InfoContext(ctx, "inspection completed", "inspection_id", id, "request_id", insp.RequestID)
	return insp, nil
}

func (s *inspectionService) ListByInspectorID(ctx context.Context, inspectorID uuid.UUID) ([]*domain.Inspection, error) {
	inspections, err := s.repo.ListByInspectorID(ctx, inspectorID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to list inspections", "error", err, "inspector_id", inspectorID)
		return nil, err
	}
	return inspections, nil
}

func (s *inspectionService) ListAll(ctx context.Context, limit, offset int) ([]*domain.Inspection, error) {
	inspections, err := s.repo.ListAll(ctx, limit, offset)
	if err != nil {
		slog.ErrorContext(ctx, "failed to list all inspections", "error", err)
		return nil, err
	}
	return inspections, nil
}
