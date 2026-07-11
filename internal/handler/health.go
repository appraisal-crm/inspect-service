package handler

import (
	"net/http"

	"github.com/appraisal-crm/inspect-service/internal/httputil"
)

// Health godoc
// @Summary     Health check
// @Tags        system
// @Produce     json
// @Success     200 {object} map[string]string
// @Router      /health [get]
func Health(w http.ResponseWriter, r *http.Request) {
	httputil.RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
