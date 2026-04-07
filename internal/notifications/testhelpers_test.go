package notifications

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// pgxRow satisfies pgx.Row for mock QueryRow results.
type pgxRow struct {
	scanFunc func(dest ...any) error
}

func (r *pgxRow) Scan(dest ...any) error {
	return r.scanFunc(dest...)
}

// pgxRows satisfies pgx.Rows for mock Query results.
type pgxRows struct {
	data    [][]any
	idx     int
	closed  bool
	scanErr error
	iterErr error
}

func (r *pgxRows) Close()                                      {}
func (r *pgxRows) Err() error                                  { return r.iterErr }
func (r *pgxRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("") }
func (r *pgxRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *pgxRows) RawValues() [][]byte                          { return nil }
func (r *pgxRows) Conn() *pgx.Conn                             { return nil }

func (r *pgxRows) Next() bool {
	if r.closed || r.idx >= len(r.data) {
		return false
	}
	r.idx++
	return r.idx <= len(r.data)
}

func (r *pgxRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	row := r.data[r.idx-1]
	for i, d := range dest {
		if i >= len(row) {
			break
		}
		switch ptr := d.(type) {
		case *string:
			*ptr = row[i].(string)
		case *int:
			*ptr = row[i].(int)
		case *bool:
			*ptr = row[i].(bool)
		case *uuid.UUID:
			*ptr = row[i].(uuid.UUID)
		case **uuid.UUID:
			switch v := row[i].(type) {
			case *uuid.UUID:
				*ptr = v
			case uuid.UUID:
				*ptr = &v
			default:
				*ptr = nil
			}
		case *time.Time:
			*ptr = row[i].(time.Time)
		}
	}
	return nil
}

func (r *pgxRows) Values() ([]any, error) {
	if r.idx <= 0 || r.idx > len(r.data) {
		return nil, fmt.Errorf("no current row")
	}
	return r.data[r.idx-1], nil
}

// noopLogger returns a logger that discards all output.
func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(nopWriter{}, nil))
}

type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }

// noopEmailer returns an EmailSender that does nothing (empty host).
func noopEmailer() *EmailSender {
	return NewEmailSender("", 0, "", "", "", noopLogger())
}
