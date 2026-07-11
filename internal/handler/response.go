package handler

import (
	"net/http"

	"github.com/appraisal-crm/inspect-service/internal/httputil"
)

// Local aliases so handlers read cleanly and Swagger sees a handler-package type.
type errorResponse = httputil.ErrorResponse

func respondError(w http.ResponseWriter, status int, message string) {
	httputil.RespondError(w, status, message)
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	httputil.RespondJSON(w, status, data)
}
