package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid token")

// JWT issues and verifies HS256 tokens.
type JWT struct {
	secret []byte
	ttl    time.Duration
}

func NewJWT(secret string, ttl time.Duration) (*JWT, error) {
	if secret == "" {
		return nil, errors.New("jwt secret is empty")
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("jwt ttl must be positive, got %s", ttl)
	}
	return &JWT{secret: []byte(secret), ttl: ttl}, nil
}

// Issue returns a signed token with the user id as subject.
func (j *JWT) Issue(userID string, now time.Time) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(j.ttl)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secret)
}

// Verify checks the signature and expiration.
func (j *JWT) Verify(token string) error {
	_, err := jwt.Parse(token,
		func(*jwt.Token) (any, error) { return j.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	return nil
}
