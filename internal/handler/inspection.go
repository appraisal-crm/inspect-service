package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/appraisal-crm/inspect-service/internal/domain"
	"github.com/appraisal-crm/inspect-service/internal/middleware"
	"github.com/appraisal-crm/inspect-service/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type inspectionHandler struct {
	svc service.InspectionService
}

func newInspectionHandler(svc service.InspectionService) *inspectionHandler {
	return &inspectionHandler{svc: svc}
}

// canAccess reports whether the caller may see or modify this inspection.
// Appraiser/admin see everything; an inspector is limited to their own.
func canAccess(ctx context.Context, insp *domain.Inspection) bool {
	if middleware.HasRole(ctx, "appraiser") || middleware.HasRole(ctx, "admin") {
		return true
	}
	if middleware.HasRole(ctx, "inspector") {
		uid, ok := middleware.UserIDFromContext(ctx)
		return ok && insp.InspectorID != nil && *insp.InspectorID == uid
	}
	return false
}

// List godoc
// @Summary     List inspections
// @Description Inspectors get inspections assigned to them. Appraiser/admin get all (paginated) or filter by inspector_id.
// @Tags        inspections
// @Produce     json
// @Security    BearerAuth
// @Param       inspector_id query string false "Filter by inspector ID (appraiser/admin only)"
// @Param       page  query int false "Page number (default 1)"
// @Param       limit query int false "Page size (default 20, max 100)"
// @Success     200 {array}  domain.Inspection
// @Success     200 {object} listAllResponse
// @Failure     400 {object} errorResponse
// @Failure     401 {object} errorResponse
// @Failure     500 {object} errorResponse
// @Router      /inspections [get]
func (h *inspectionHandler) List(w http.ResponseWriter, r *http.Request) {
	// Inspector without a broader role: only their own assignments.
	if middleware.HasRole(r.Context(), "inspector") &&
		!middleware.HasRole(r.Context(), "appraiser") &&
		!middleware.HasRole(r.Context(), "admin") {
		uid, _ := middleware.UserIDFromContext(r.Context())
		inspections, err := h.svc.ListByInspectorID(r.Context(), uid)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to list inspections")
			return
		}
		respondJSON(w, http.StatusOK, inspections)
		return
	}

	if raw := r.URL.Query().Get("inspector_id"); raw != "" {
		inspectorID, err := uuid.Parse(raw)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid inspector_id query param")
			return
		}
		inspections, err := h.svc.ListByInspectorID(r.Context(), inspectorID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to list inspections")
			return
		}
		respondJSON(w, http.StatusOK, inspections)
		return
	}

	page := parseIntParam(r.URL.Query().Get("page"), 1)
	limit := parseIntParam(r.URL.Query().Get("limit"), 20)
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	inspections, err := h.svc.ListAll(r.Context(), limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list inspections")
		return
	}
	respondJSON(w, http.StatusOK, listAllResponse{Data: inspections, Page: page, Limit: limit})
}

// GetByID godoc
// @Summary     Get inspection by ID
// @Tags        inspections
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Inspection ID"
// @Success     200 {object} domain.Inspection
// @Failure     400 {object} errorResponse
// @Failure     401 {object} errorResponse
// @Failure     403 {object} errorResponse
// @Failure     404 {object} errorResponse
// @Router      /inspections/{id} [get]
func (h *inspectionHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	insp, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		writeServiceError(w, err, "failed to get inspection")
		return
	}
	if !canAccess(r.Context(), insp) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	respondJSON(w, http.StatusOK, insp)
}

// Update godoc
// @Summary     Update inspection fields
// @Description Inspector fills notes/property_data on their own inspection; assigning inspector_id is appraiser/admin only.
// @Tags        inspections
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Inspection ID"
// @Param       body body updateInspectionDTO true "Fields to update"
// @Success     200 {object} domain.Inspection
// @Failure     400 {object} errorResponse
// @Failure     401 {object} errorResponse
// @Failure     403 {object} errorResponse
// @Failure     404 {object} errorResponse
// @Failure     409 {object} errorResponse
// @Failure     500 {object} errorResponse
// @Router      /inspections/{id} [patch]
func (h *inspectionHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var dto updateInspectionDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validate.Struct(dto); err != nil {
		respondError(w, http.StatusBadRequest, firstValidationError(err))
		return
	}
	if dto.PropertyData != nil && !json.Valid(dto.PropertyData) {
		respondError(w, http.StatusBadRequest, "property_data must be valid JSON")
		return
	}

	// Only appraiser/admin may assign an inspector.
	if dto.InspectorID != nil && !middleware.HasRole(r.Context(), "appraiser") && !middleware.HasRole(r.Context(), "admin") {
		respondError(w, http.StatusForbidden, "only appraiser or admin can assign an inspector")
		return
	}

	insp, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		writeServiceError(w, err, "failed to update inspection")
		return
	}
	if !canAccess(r.Context(), insp) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	updated, err := h.svc.Update(r.Context(), id, service.UpdateInput{
		InspectorID:  dto.InspectorID,
		Notes:        dto.Notes,
		PropertyData: dto.PropertyData,
	})
	if err != nil {
		writeServiceError(w, err, "failed to update inspection")
		return
	}
	respondJSON(w, http.StatusOK, updated)
}

// Complete godoc
// @Summary     Complete an inspection
// @Description Marks the inspection completed and emits inspect.completed. Inspector only.
// @Tags        inspections
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Inspection ID"
// @Success     200 {object} domain.Inspection
// @Failure     400 {object} errorResponse
// @Failure     401 {object} errorResponse
// @Failure     403 {object} errorResponse
// @Failure     404 {object} errorResponse
// @Failure     409 {object} errorResponse
// @Failure     422 {object} errorResponse
// @Failure     500 {object} errorResponse
// @Router      /inspections/{id}/complete [post]
func (h *inspectionHandler) Complete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	insp, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		writeServiceError(w, err, "failed to complete inspection")
		return
	}
	if !canAccess(r.Context(), insp) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	completed, err := h.svc.Complete(r.Context(), id)
	if err != nil {
		writeServiceError(w, err, "failed to complete inspection")
		return
	}
	respondJSON(w, http.StatusOK, completed)
}

func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

// writeServiceError maps domain errors to HTTP status codes — the one place
// where domain vocabulary becomes HTTP.
func writeServiceError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		respondError(w, http.StatusNotFound, "inspection not found")
	case errors.Is(err, domain.ErrConflict):
		respondError(w, http.StatusConflict, "inspection was modified concurrently, please retry")
	case errors.Is(err, domain.ErrInvalidStatus):
		respondError(w, http.StatusUnprocessableEntity, "invalid status transition")
	default:
		respondError(w, http.StatusInternalServerError, fallback)
	}
}

func parseIntParam(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
