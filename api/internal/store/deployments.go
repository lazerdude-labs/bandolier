package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type Deployment struct {
	ID           string         `json:"id"`
	ClusterID    string         `json:"cluster_id"`
	Operation    string         `json:"operation"`
	Status       string         `json:"status"`
	StartedAt    sql.NullTime   `json:"-"`
	FinishedAt   sql.NullTime   `json:"-"`
	ErrorMessage sql.NullString `json:"-"`
	LogPath      sql.NullString `json:"-"`
	ActorID      *int64         `json:"actor_id,omitempty"`
}

// MarshalJSON emits snake_case fields and unwraps sql.Null* wrappers
// so the API contract matches what the UI expects.
func (d Deployment) MarshalJSON() ([]byte, error) {
	type wire struct {
		ID           string  `json:"id"`
		ClusterID    string  `json:"cluster_id"`
		Operation    string  `json:"operation"`
		Status       string  `json:"status"`
		StartedAt    *string `json:"started_at"`
		FinishedAt   *string `json:"finished_at"`
		ErrorMessage *string `json:"error_message"`
		LogPath      *string `json:"log_path"`
		ActorID      *int64  `json:"actor_id"`
	}
	w := wire{
		ID:        d.ID,
		ClusterID: d.ClusterID,
		Operation: d.Operation,
		Status:    d.Status,
		ActorID:   d.ActorID,
	}
	if d.StartedAt.Valid {
		s := d.StartedAt.Time.UTC().Format(time.RFC3339)
		w.StartedAt = &s
	}
	if d.FinishedAt.Valid {
		s := d.FinishedAt.Time.UTC().Format(time.RFC3339)
		w.FinishedAt = &s
	}
	if d.ErrorMessage.Valid && d.ErrorMessage.String != "" {
		w.ErrorMessage = &d.ErrorMessage.String
	}
	if d.LogPath.Valid && d.LogPath.String != "" {
		w.LogPath = &d.LogPath.String
	}
	return json.Marshal(w)
}

func (s *Store) CreateDeployment(ctx context.Context, d *Deployment) error {
	d.StartedAt.Time = time.Now().UTC()
	d.StartedAt.Valid = true
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO deployments (id, cluster_id, operation, status, started_at, actor_id) VALUES (?, ?, ?, ?, ?, ?)`,
		d.ID, d.ClusterID, d.Operation, d.Status, d.StartedAt.Time, d.ActorID)
	return err
}

func (s *Store) FinishDeployment(ctx context.Context, id, status, errMsg string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`UPDATE deployments SET status = ?, finished_at = ?, error_message = ? WHERE id = ?`,
		status, now, errMsg, id)
	return err
}

// ListDeploymentsForCluster returns the most recent `limit` deployments for a
// cluster, ordered newest first. limit ≤ 0 disables the limit.
func (s *Store) ListDeploymentsForCluster(ctx context.Context, clusterID string, limit int) ([]Deployment, error) {
	q := `SELECT id, cluster_id, operation, status, started_at, finished_at, error_message, log_path, actor_id
	      FROM deployments WHERE cluster_id = ? ORDER BY started_at DESC`
	args := []any{clusterID}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]Deployment, 0)
	for rows.Next() {
		var d Deployment
		var actorID sql.NullInt64
		if err := rows.Scan(&d.ID, &d.ClusterID, &d.Operation, &d.Status, &d.StartedAt, &d.FinishedAt, &d.ErrorMessage, &d.LogPath, &actorID); err != nil {
			return nil, err
		}
		if actorID.Valid {
			v := actorID.Int64
			d.ActorID = &v
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ListRunningDeployments returns all deployments currently marked `running`.
// Used at startup to detect orphaned deployments left behind by an api restart.
func (s *Store) ListRunningDeployments(ctx context.Context) ([]Deployment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, cluster_id, operation, status, started_at, finished_at, error_message, log_path, actor_id
		 FROM deployments WHERE status = 'running' ORDER BY started_at ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]Deployment, 0)
	for rows.Next() {
		var d Deployment
		var actorID sql.NullInt64
		if err := rows.Scan(&d.ID, &d.ClusterID, &d.Operation, &d.Status, &d.StartedAt, &d.FinishedAt, &d.ErrorMessage, &d.LogPath, &actorID); err != nil {
			return nil, err
		}
		if actorID.Valid {
			v := actorID.Int64
			d.ActorID = &v
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) GetDeployment(ctx context.Context, id string) (*Deployment, error) {
	d := &Deployment{}
	var actorID sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT id, cluster_id, operation, status, started_at, finished_at, error_message, log_path, actor_id FROM deployments WHERE id = ?`, id).
		Scan(&d.ID, &d.ClusterID, &d.Operation, &d.Status, &d.StartedAt, &d.FinishedAt, &d.ErrorMessage, &d.LogPath, &actorID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if actorID.Valid {
		v := actorID.Int64
		d.ActorID = &v
	}
	return d, err
}
