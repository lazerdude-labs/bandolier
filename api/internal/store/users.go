package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type User struct {
	ID           int64
	PasswordHash string
}

var ErrNotFound = errors.New("not found")

func (s *Store) CreateUser(ctx context.Context, hash string) (*User, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (password_hash) VALUES (?)`, hash)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &User{ID: id, PasswordHash: hash}, nil
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (*User, error) {
	u := &User{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, password_hash FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) UpdateUserPassword(ctx context.Context, id int64, hash string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, hash, id)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
