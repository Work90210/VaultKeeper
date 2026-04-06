package httputil

import (
	"encoding/json"
	"net/http"
)

type envelope struct {
	Data  any `json:"data"`
	Error any `json:"error"`
	Meta  any `json:"meta"`
}

type paginationMeta struct {
	Total      int    `json:"total"`
	NextCursor string `json:"next_cursor"`
	HasMore    bool   `json:"has_more"`
}

func RespondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{
		Data:  data,
		Error: nil,
		Meta:  nil,
	})
}

func RespondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{
		Data:  nil,
		Error: message,
		Meta:  nil,
	})
}

func RespondPaginated(w http.ResponseWriter, status int, data any, total int, cursor string, hasMore bool) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{
		Data:  data,
		Error: nil,
		Meta: paginationMeta{
			Total:      total,
			NextCursor: cursor,
			HasMore:    hasMore,
		},
	})
}
