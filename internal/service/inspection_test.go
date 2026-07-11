package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/appraisal-crm/inspect-service/internal/domain"
	"github.com/appraisal-crm/inspect-service/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRepo implements repository.InspectionRepository. Each test wires only the
// func fields it needs; unset ones stay nil and simply aren't called.
type mockRepo struct {
	createFn   func(ctx context.Context, insp *domain.Inspection) (bool, error)
	getFn      func(ctx context.Context, id uuid.UUID) (*domain.Inspection, error)
	updateFn   func(ctx context.Context, insp *domain.Inspection, prev time.Time) error
	completeFn func(ctx context.Context, id uuid.UUID, completedAt, updatedAt time.Time, ev domain.EventEnvelope) error
}

func (m *mockRepo) Create(ctx context.Context, insp *domain.Inspection) (bool, error) {
	return m.createFn(ctx, insp)
}
func (m *mockRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Inspection, error) {
	return m.getFn(ctx, id)
}
func (m *mockRepo) Update(ctx context.Context, insp *domain.Inspection, prev time.Time) error {
	return m.updateFn(ctx, insp, prev)
}
func (m *mockRepo) Complete(ctx context.Context, id uuid.UUID, completedAt, updatedAt time.Time, ev domain.EventEnvelope) error {
	return m.completeFn(ctx, id, completedAt, updatedAt, ev)
}
func (m *mockRepo) AddPhoto(ctx context.Context, p *domain.Photo) error { return nil }
func (m *mockRepo) ListByInspectorID(ctx context.Context, id uuid.UUID) ([]*domain.Inspection, error) {
	return nil, nil
}
func (m *mockRepo) ListAll(ctx context.Context, limit, offset int) ([]*domain.Inspection, error) {
	return nil, nil
}

// stubStorage satisfies storage.PhotoStorage without any I/O.
type stubStorage struct{}

func (stubStorage) KeyFor(id uuid.UUID, filename string) string { return "key/" + filename }
func (stubStorage) PresignUpload(_ context.Context, key string) (string, error) {
	return "https://upload/" + key, nil
}

func TestCreateFromRequest(t *testing.T) {
	tests := []struct {
		name        string
		repoCreated bool
		repoErr     error
		wantCreated bool
		wantErr     bool
	}{
		{name: "new inspection", repoCreated: true, wantCreated: true},
		{name: "duplicate request is a no-op", repoCreated: false, wantCreated: false},
		{name: "repo error propagates", repoErr: errors.New("boom"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepo{createFn: func(_ context.Context, insp *domain.Inspection) (bool, error) {
				assert.Equal(t, domain.StatusScheduled, insp.Status) // always born scheduled
				return tt.repoCreated, tt.repoErr
			}}
			svc := NewInspectionService(repo, stubStorage{})
			created, err := svc.CreateFromRequest(context.Background(), uuid.New())
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantCreated, created)
		})
	}
}

func TestComplete(t *testing.T) {
	id := uuid.New()
	tests := []struct {
		name       string
		current    domain.Status
		getErr     error
		completeFn func(t *testing.T) func(context.Context, uuid.UUID, time.Time, time.Time, domain.EventEnvelope) error
		wantErr    error
	}{
		{
			name:    "scheduled completes and emits event",
			current: domain.StatusScheduled,
			completeFn: func(t *testing.T) func(context.Context, uuid.UUID, time.Time, time.Time, domain.EventEnvelope) error {
				return func(_ context.Context, _ uuid.UUID, _, _ time.Time, ev domain.EventEnvelope) error {
					assert.Equal(t, domain.EventTypeInspectCompleted, ev.EventType)
					return nil
				}
			},
		},
		{
			name:    "already completed is an invalid transition",
			current: domain.StatusCompleted,
			wantErr: domain.ErrInvalidStatus,
		},
		{
			name:    "not found",
			getErr:  repository.ErrNotFound,
			wantErr: domain.ErrNotFound,
		},
		{
			name:    "repo conflict maps to domain conflict",
			current: domain.StatusScheduled,
			completeFn: func(t *testing.T) func(context.Context, uuid.UUID, time.Time, time.Time, domain.EventEnvelope) error {
				return func(_ context.Context, _ uuid.UUID, _, _ time.Time, _ domain.EventEnvelope) error {
					return repository.ErrConflict
				}
			},
			wantErr: domain.ErrConflict,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepo{
				getFn: func(_ context.Context, _ uuid.UUID) (*domain.Inspection, error) {
					if tt.getErr != nil {
						return nil, tt.getErr
					}
					return &domain.Inspection{ID: id, RequestID: uuid.New(), Status: tt.current}, nil
				},
			}
			if tt.completeFn != nil {
				repo.completeFn = tt.completeFn(t)
			}
			svc := NewInspectionService(repo, stubStorage{})
			out, err := svc.Complete(context.Background(), id)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, domain.StatusCompleted, out.Status)
			require.NotNil(t, out.CompletedAt)
		})
	}
}

func TestUpdateAppliesFields(t *testing.T) {
	id := uuid.New()
	newInspector := uuid.New()
	notes := "cracked wall"
	repo := &mockRepo{
		getFn: func(_ context.Context, _ uuid.UUID) (*domain.Inspection, error) {
			return &domain.Inspection{ID: id, Status: domain.StatusScheduled}, nil
		},
		updateFn: func(_ context.Context, _ *domain.Inspection, _ time.Time) error { return nil },
	}
	svc := NewInspectionService(repo, stubStorage{})
	out, err := svc.Update(context.Background(), id, UpdateInput{
		InspectorID:  &newInspector,
		Notes:        &notes,
		PropertyData: json.RawMessage(`{"rooms":3}`),
	})
	require.NoError(t, err)
	assert.Equal(t, newInspector, *out.InspectorID)
	assert.Equal(t, notes, *out.Notes)
	assert.JSONEq(t, `{"rooms":3}`, string(out.PropertyData))
}

func TestUpdateConflict(t *testing.T) {
	repo := &mockRepo{
		getFn: func(_ context.Context, _ uuid.UUID) (*domain.Inspection, error) {
			return &domain.Inspection{ID: uuid.New(), Status: domain.StatusScheduled}, nil
		},
		updateFn: func(_ context.Context, _ *domain.Inspection, _ time.Time) error {
			return repository.ErrConflict
		},
	}
	svc := NewInspectionService(repo, stubStorage{})
	_, err := svc.Update(context.Background(), uuid.New(), UpdateInput{})
	require.ErrorIs(t, err, domain.ErrConflict)
}

func TestAddPhotoRejectsCompleted(t *testing.T) {
	repo := &mockRepo{
		getFn: func(_ context.Context, _ uuid.UUID) (*domain.Inspection, error) {
			return &domain.Inspection{ID: uuid.New(), Status: domain.StatusCompleted}, nil
		},
	}
	svc := NewInspectionService(repo, stubStorage{})
	_, err := svc.AddPhoto(context.Background(), uuid.New(), "photo.jpg")
	require.ErrorIs(t, err, domain.ErrInvalidStatus)
}

func TestAddPhotoSuccess(t *testing.T) {
	var savedKey string
	repo := &mockRepo{
		getFn: func(_ context.Context, _ uuid.UUID) (*domain.Inspection, error) {
			return &domain.Inspection{ID: uuid.New(), Status: domain.StatusScheduled}, nil
		},
	}
	// Override AddPhoto to capture the stored key via a local closure repo.
	capRepo := &captureRepo{mockRepo: repo, onAdd: func(p *domain.Photo) { savedKey = p.S3Key }}
	svc := NewInspectionService(capRepo, stubStorage{})
	res, err := svc.AddPhoto(context.Background(), uuid.New(), "roof.jpg")
	require.NoError(t, err)
	assert.Equal(t, "key/roof.jpg", savedKey)
	assert.Equal(t, "https://upload/key/roof.jpg", res.UploadURL)
}

// captureRepo wraps mockRepo to observe AddPhoto without changing the shared mock.
type captureRepo struct {
	*mockRepo
	onAdd func(*domain.Photo)
}

func (c *captureRepo) AddPhoto(ctx context.Context, p *domain.Photo) error {
	c.onAdd(p)
	return nil
}
