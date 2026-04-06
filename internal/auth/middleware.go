package auth

import "context"

type JWTClaims struct {
	Subject    string
	Email      string
	SystemRole string
	Roles      []string
}

type JWTValidator interface {
	ValidateToken(ctx context.Context, rawToken string) (JWTClaims, error)
}
