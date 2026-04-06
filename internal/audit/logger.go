package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthEvent struct {
	Action      string
	ActorUserID string
	IPAddress   string
	UserAgent   string
	Detail      map[string]string
}

type Logger struct {
	pool *pgxpool.Pool
}

func NewLogger(pool *pgxpool.Pool) *Logger {
	return &Logger{pool: pool}
}

func (l *Logger) LogAccessDenied(ctx context.Context, userID, endpoint, requiredRole, actualRole string, ip string) {
	_ = l.LogAuthEvent(ctx, AuthEvent{
		Action:      "access_denied",
		ActorUserID: userID,
		IPAddress:   ip,
		Detail: map[string]string{
			"endpoint":      endpoint,
			"required_role": requiredRole,
			"actual_role":   actualRole,
		},
	})
}

func (l *Logger) LogAuthEvent(ctx context.Context, event AuthEvent) error {
	detail, err := json.Marshal(event.Detail)
	if err != nil {
		return fmt.Errorf("marshal auth event detail: %w", err)
	}

	tx, err := l.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin auth audit tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Serialize writers to prevent hash chain forks
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(847213)`); err != nil {
		return fmt.Errorf("lock auth audit chain: %w", err)
	}

	previousHash, err := getLastHash(ctx, tx)
	if err != nil {
		return fmt.Errorf("get last auth audit hash: %w", err)
	}

	hashValue := computeHash(event.Action, event.ActorUserID, string(detail), previousHash, time.Now().UTC())

	_, err = tx.Exec(ctx,
		`INSERT INTO auth_audit_log (action, actor_user_id, ip_address, user_agent, detail, hash_value, previous_hash)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		event.Action, event.ActorUserID, event.IPAddress, event.UserAgent, detail, hashValue, previousHash,
	)
	if err != nil {
		return fmt.Errorf("insert auth audit log: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit auth audit log: %w", err)
	}

	return nil
}

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func getLastHash(ctx context.Context, q queryRower) (string, error) {
	var hash string
	err := q.QueryRow(ctx,
		`SELECT hash_value FROM auth_audit_log ORDER BY created_at DESC, id DESC LIMIT 1`,
	).Scan(&hash)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("query last hash: %w", err)
	}
	return hash, nil
}

func computeHash(action, actor, detail, previousHash string, ts time.Time) string {
	data := fmt.Sprintf("%s|%s|%s|%s|%d", action, actor, detail, previousHash, ts.UnixNano())
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}
