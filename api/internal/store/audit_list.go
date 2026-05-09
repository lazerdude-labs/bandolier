package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type AuditFilter struct {
	Action  string     // exact match; empty = any
	Outcome string     // exact match; empty = any
	ActorID int64      // 0 = any
	Since   *time.Time // nil = any
	Limit   int        // <= 0 defaults to 50; capped at 200
}

type AuditRow struct {
	ID      int64
	ActorID *int64
	Action  string
	Target  *string
	Outcome string
	TS      time.Time
	Details *string
}

func (s *Store) ListAuditEntries(ctx context.Context, f AuditFilter) ([]AuditRow, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	clauses := []string{"1=1"}
	args := []any{}
	if f.Action != "" {
		clauses = append(clauses, "action = ?")
		args = append(args, f.Action)
	}
	if f.Outcome != "" {
		clauses = append(clauses, "outcome = ?")
		args = append(args, f.Outcome)
	}
	if f.ActorID != 0 {
		clauses = append(clauses, "actor_id = ?")
		args = append(args, f.ActorID)
	}
	if f.Since != nil {
		clauses = append(clauses, "ts >= ?")
		args = append(args, f.Since.UTC())
	}
	args = append(args, limit)

	q := fmt.Sprintf(
		`SELECT id, actor_id, action, target, outcome, ts, details
		 FROM audit_log WHERE %s ORDER BY ts DESC LIMIT ?`,
		strings.Join(clauses, " AND "),
	)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list audit: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]AuditRow, 0, limit)
	for rows.Next() {
		var r AuditRow
		var actor *int64
		var target, details *string
		if err := rows.Scan(&r.ID, &actor, &r.Action, &target, &r.Outcome, &r.TS, &details); err != nil {
			return nil, err
		}
		r.ActorID = actor
		r.Target = target
		r.Details = details
		out = append(out, r)
	}
	return out, rows.Err()
}
