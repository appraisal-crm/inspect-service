package handler

import (
	"encoding/json"

	"github.com/appraisal-crm/inspect-service/internal/domain"
	"github.com/google/uuid"
)

// updateInspectionDTO — all fields optional; only the present ones are patched.
type updateInspectionDTO struct {
	InspectorID  *uuid.UUID      `json:"inspector_id"`
	Notes        *string         `json:"notes"         validate:"omitempty,max=5000"`
	PropertyData json.RawMessage `json:"property_data" swaggertype:"object"`
}

// listAllResponse is the paginated envelope for appraiser/admin list-all.
type listAllResponse struct {
	Data  []*domain.Inspection `json:"data"`
	Page  int                  `json:"page"`
	Limit int                  `json:"limit"`
}
