package profile

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrProfileNotFound = errors.New("profile not found")

type Repository interface {
	GetByUserID(ctx context.Context, userID string) (Profile, error)
	Upsert(ctx context.Context, p Profile) (Profile, error)
}

type PGRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *PGRepository {
	return &PGRepository{pool: pool}
}

func (r *PGRepository) GetByUserID(ctx context.Context, userID string) (Profile, error) {
	var p Profile
	err := r.pool.QueryRow(ctx,
		`SELECT user_id, COALESCE(display_name, ''), COALESCE(avatar_url, ''), bio, timezone, updated_at
		 FROM user_profiles WHERE user_id = $1`,
		userID,
	).Scan(&p.UserID, &p.DisplayName, &p.AvatarURL, &p.Bio, &p.Timezone, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Profile{}, ErrProfileNotFound
		}
		return Profile{}, fmt.Errorf("get profile: %w", err)
	}
	return p, nil
}

func (r *PGRepository) Upsert(ctx context.Context, p Profile) (Profile, error) {
	var result Profile
	err := r.pool.QueryRow(ctx,
		`INSERT INTO user_profiles (user_id, display_name, avatar_url, bio, timezone)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (user_id) DO UPDATE
		 SET display_name = COALESCE(EXCLUDED.display_name, user_profiles.display_name),
		     avatar_url = COALESCE(EXCLUDED.avatar_url, user_profiles.avatar_url),
		     bio = COALESCE(EXCLUDED.bio, user_profiles.bio),
		     timezone = COALESCE(EXCLUDED.timezone, user_profiles.timezone),
		     updated_at = now()
		 RETURNING user_id, COALESCE(display_name, ''), COALESCE(avatar_url, ''), bio, timezone, updated_at`,
		p.UserID, nilIfEmpty(p.DisplayName), nilIfEmpty(p.AvatarURL), p.Bio, p.Timezone,
	).Scan(&result.UserID, &result.DisplayName, &result.AvatarURL, &result.Bio, &result.Timezone, &result.UpdatedAt)
	if err != nil {
		return Profile{}, fmt.Errorf("upsert profile: %w", err)
	}
	return result, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
