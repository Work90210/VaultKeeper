package server

import (
	"net/http"

	"github.com/vaultkeeper/vaultkeeper/internal/httputil"
)

func RespondJSON(w http.ResponseWriter, status int, data any) {
	httputil.RespondJSON(w, status, data)
}

func RespondError(w http.ResponseWriter, status int, message string) {
	httputil.RespondError(w, status, message)
}

func RespondPaginated(w http.ResponseWriter, status int, data any, total int, cursor string, hasMore bool) {
	httputil.RespondPaginated(w, status, data, total, cursor, hasMore)
}
