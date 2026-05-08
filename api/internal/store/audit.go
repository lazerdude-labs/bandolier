package store

import (
	"context"
	"fmt"
)

type AuditEntry struct {
	ActorID int64
	Action  string
	Target  string
	Outcome string
	Details *string
}

func (s *Store) InsertAuditEntry(ctx context.Context, e AuditEntry) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO audit_log(actor_id, action, target, outcome, details)
		VALUES(?, ?, ?, ?, ?)`,
		nullableInt64(e.ActorID), e.Action, nullableString(e.Target), e.Outcome, nullableStringPtr(e.Details),
	)
	if err != nil {
		return 0, fmt.Errorf("insert audit: %w", err)
	}
	return res.LastInsertId()
}

func (s *Store) CountAuditEntries(ctx context.Context, action string, outcome string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM audit_log WHERE action = ? AND outcome = ?`, action, outcome,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count audit: %w", err)
	}
	return n, nil
}
