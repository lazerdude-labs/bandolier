package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Session struct {
	ID        string
	UserID    int64
	ExpiresAt time.Time
}

func (s *Store) CreateSession(ctx context.Context, id string, userID int64, ttlSeconds int) error {
	expires := time.Now().UTC().Add(time.Duration(ttlSeconds) * time.Second)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)`,
		id, userID, expires)
	return err
}

func (s *Store) GetSession(ctx context.Context, id string) (*Session, error) {
	sess := &Session{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, expires_at FROM sessions
		 WHERE id = ? AND expires_at > CURRENT_TIMESTAMP`, id).
		Scan(&sess.ID, &sess.UserID, &sess.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return sess, err
}

func (s *Store) DeleteSession(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	return err
}
