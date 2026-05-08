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
		`SELECT id, name, profile, status, created_at, updated_at FROM clusters WHERE id = ?`, id).
		Scan(&c.ID, &c.Name, &c.Profile, &c.Status, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

func (s *Store) ListClusters(ctx context.Context) ([]Cluster, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, profile, status, created_at, updated_at FROM clusters ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Cluster
	for rows.Next() {
		var c Cluster
		if err := rows.Scan(&c.ID, &c.Name, &c.Profile, &c.Status, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
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

// ListReadyClusters returns the IDs of all clusters in the `ready` state.
// Used by the wildcard cert renewal goroutine to know which clusters to scan.
func (s *Store) ListReadyClusters(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM clusters WHERE status = 'ready'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
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
