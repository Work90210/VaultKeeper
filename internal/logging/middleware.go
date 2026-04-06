package logging

import (
	"io"
	"log/slog"
	"net/http"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

func NewLogger(appEnv string, level slog.Leveler, writer io.Writer) *slog.Logger {
	options := &slog.HandlerOptions{Level: level}
	if appEnv == "production" {
		return slog.New(slog.NewJSONHandler(writer, options))
	}
	return slog.New(slog.NewTextHandler(writer, options))
}

func Middleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}

			requestID := chimiddleware.GetReqID(r.Context())
			if requestID == "" {
				requestID = uuid.NewString()
			}
			w.Header().Set("X-Request-ID", requestID)

			requestLogger := logger.With("request_id", requestID)
			startedAt := time.Now()
			requestLogger.Info("request.started",
				"method", r.Method,
				"path", r.URL.Path,
				"query", RedactMap(queryFields(r)),
				"headers", RedactMap(headerFields(r)),
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
			)

			recorder := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(recorder, r)

			requestLogger.Info("request.completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status_code", recorder.statusCode,
				"bytes_written", recorder.bytesWritten,
				"duration_ms", time.Since(startedAt).Milliseconds(),
			)
		})
	}
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(body []byte) (int, error) {
	written, err := r.ResponseWriter.Write(body)
	r.bytesWritten += written
	return written, err
}

func queryFields(r *http.Request) map[string]any {
	values := r.URL.Query()
	if len(values) == 0 {
		return nil
	}

	fields := make(map[string]any, len(values))
	for key, vals := range values {
		if len(vals) == 1 {
			fields[key] = vals[0]
			continue
		}
		items := make([]any, 0, len(vals))
		for _, entry := range vals {
			items = append(items, entry)
		}
		fields[key] = items
	}
	return fields
}

func headerFields(r *http.Request) map[string]any {
	if len(r.Header) == 0 {
		return nil
	}

	fields := make(map[string]any, len(r.Header))
	for key, vals := range r.Header {
		if len(vals) == 1 {
			fields[key] = vals[0]
			continue
		}
		items := make([]any, 0, len(vals))
		for _, entry := range vals {
			items = append(items, entry)
		}
		fields[key] = items
	}
	return fields
}
