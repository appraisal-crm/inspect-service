package storage

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// PhotoStorage abstracts object storage for inspection photos. The real backend
// is S3 (Yandex Cloud); StubStorage stands in until the SDK/credentials are wired.
// Hiding it behind an interface means the service never changes when we swap it.
type PhotoStorage interface {
	// KeyFor builds the object key for a photo of an inspection.
	KeyFor(inspectionID uuid.UUID, filename string) string
	// PresignUpload returns a URL the client can PUT the image bytes to.
	PresignUpload(ctx context.Context, key string) (string, error)
}

// StubStorage produces deterministic keys and a fake upload URL. No network I/O.
type StubStorage struct {
	endpoint string
	bucket   string
}

func NewStubStorage(endpoint, bucket string) *StubStorage {
	return &StubStorage{endpoint: endpoint, bucket: bucket}
}

// KeyFor namespaces objects per inspection and adds a random prefix so two files
// with the same name never collide: inspections/<id>/<random>-<filename>.
func (s *StubStorage) KeyFor(inspectionID uuid.UUID, filename string) string {
	return fmt.Sprintf("inspections/%s/%s-%s", inspectionID, uuid.NewString(), filename)
}

func (s *StubStorage) PresignUpload(_ context.Context, key string) (string, error) {
	return fmt.Sprintf("%s/%s/%s?stub-presigned=true", s.endpoint, s.bucket, key), nil
}
