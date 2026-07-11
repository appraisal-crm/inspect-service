package httputil

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse is the single JSON error shape across the API: {"error": "..."}.
type ErrorResponse struct {
	Error string `json:"error"`
}

func RespondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

func RespondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
