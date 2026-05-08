package auth

import (
	"context"
	"errors"
)

const MinPasswordLength = 12

var ErrPasswordTooShort = errors.New("password must be at least 12 characters")

// ValidatePassword returns ErrPasswordTooShort when p is shorter than MinPasswordLength.
func ValidatePassword(p string) error {
	if len(p) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	return nil
}

// WithUserID returns a new context carrying the authenticated user id.
// Production code uses RequireSession middleware; tests use this helper directly.
func WithUserID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, userCtxKey, id)
}
