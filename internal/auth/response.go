package auth

import (
	"encoding/json"
	"net/http"
)

func respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data":  nil,
		"error": message,
		"meta":  nil,
	})
}
