package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

var ErrDuplicateName = errors.New("cluster name already exists")

type Cluster struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Profile   string    `json:"profile"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// PendingForget is the cascade-delete latch. When true, the executor's
	// runDestroy success path invokes the Forget orchestrator (Vault purge +
	// DB row drop) after the cluster transitions to `destroyed`. Set by the
	// DELETE /api/clusters/{id}?cascade=destroy handler against live
	// clusters; cleared on destroy failure so the operator decides what to
	// do with a partially-destroyed cluster.
	PendingForget bool `json:"pending_forget"`
}

func (s *Store) CreateCluster(ctx context.Context, c *Cluster) error {
	now := time.Now().UTC()
	c.CreatedAt = now
	c.UpdatedAt = now
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO clusters (id, name, profile, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.Profile, c.Status, c.CreatedAt, c.UpdatedAt)
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed: clusters.name") {
		return ErrDuplicateName
	}
	return err
}

func (s *Store) GetCluster(ctx context.Context, id string) (*Cluster, error) {
	c := &Cluster{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, profile, status, created_at, updated_at, pending_forget FROM clusters WHERE id = ?`, id).
		Scan(&c.ID, &c.Name, &c.Profile, &c.Status, &c.CreatedAt, &c.UpdatedAt, &c.PendingForget)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

func (s *Store) ListClusters(ctx context.Context) ([]Cluster, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, profile, status, created_at, updated_at, pending_forget FROM clusters ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Cluster
	for rows.Next() {
		var c Cluster
		if err := rows.Scan(&c.ID, &c.Name, &c.Profile, &c.Status, &c.CreatedAt, &c.UpdatedAt, &c.PendingForget); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetPendingForget flips the cascade-delete latch on a cluster. Used by the
// DELETE /api/clusters/{id}?cascade=destroy handler to mark a live cluster
// for forget-after-destroy. Idempotent; setting to the current value is a
// no-op.
func (s *Store) SetPendingForget(ctx context.Context, id string, val bool) error {
	v := 0
	if val {
		v = 1
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE clusters SET pending_forget = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		v, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdateClusterStatus(ctx context.Context, id, status string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE clusters SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteCluster removes the cluster row. Schema FKs declare ON DELETE CASCADE
// against clusters(id) for deployments / apps_repos / apps_installs, so those
// rows go with it. Audit log rows are intentionally left intact (they target
// the cluster id by string for after-the-fact accountability).
func (s *Store) DeleteCluster(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM clusters WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListReadyClusters returns the IDs of all clusters in the `ready` state.
// Used by the wildcard cert renewal goroutine to know which clusters to scan.
func (s *Store) ListReadyClusters(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM clusters WHERE status = 'ready'`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
