package apps

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lazerdude-labs/bandolier/api/internal/store"
)

// ErrNotFound is returned when a repo or install row does not exist.
var ErrNotFound = errors.New("apps: not found")

// Store wraps the underlying store.Store and exposes CRUD for apps_repos and
// apps_installs. It is intentionally narrow — handlers/executor compose this
// with the Helm wrapper rather than reaching into *sql.DB directly.
type Store struct {
	s *store.Store
}

func NewStore(s *store.Store) *Store { return &Store{s: s} }

func (a *Store) CreateRepo(ctx context.Context, clusterID, name, url string, addedBy *int64) (int64, error) {
	res, err := a.s.DB().ExecContext(ctx,
		`INSERT INTO apps_repos (cluster_id, name, url, added_by) VALUES (?,?,?,?)`,
		clusterID, name, url, addedBy)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (a *Store) ListRepos(ctx context.Context, clusterID string) ([]Repo, error) {
	rows, err := a.s.DB().QueryContext(ctx,
		`SELECT id, cluster_id, name, url, added_at, added_by
		   FROM apps_repos WHERE cluster_id = ? ORDER BY name`, clusterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Repo
	for rows.Next() {
		var r Repo
		var addedBy sql.NullInt64
		if err := rows.Scan(&r.ID, &r.ClusterID, &r.Name, &r.URL, &r.AddedAt, &addedBy); err != nil {
			return nil, err
		}
		if addedBy.Valid {
			r.AddedBy = &addedBy.Int64
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (a *Store) DeleteRepo(ctx context.Context, clusterID, name string) error {
	res, err := a.s.DB().ExecContext(ctx,
		`DELETE FROM apps_repos WHERE cluster_id = ? AND name = ?`, clusterID, name)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (a *Store) CreateInstall(ctx context.Context, in *Install) error {
	atomic := 0
	if in.Atomic {
		atomic = 1
	}
	_, err := a.s.DB().ExecContext(ctx,
		`INSERT INTO apps_installs (id, cluster_id, chart, version, release_name, namespace, hostname, operation, status, atomic, values_hash, actor_id)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		in.ID, in.ClusterID, in.Chart, in.Version, in.ReleaseName, in.Namespace,
		in.Hostname, in.Operation, in.Status, atomic, in.ValuesHash, in.ActorID)
	return err
}

func (a *Store) GetInstall(ctx context.Context, id string) (*Install, error) {
	row := a.s.DB().QueryRowContext(ctx,
		`SELECT id, cluster_id, chart, version, release_name, namespace, hostname,
		        operation, status, atomic, values_hash, started_at, finished_at, error_message, actor_id, hostname_unclaimed
		   FROM apps_installs WHERE id = ?`, id)
	var in Install
	var hostname, valuesHash, errMsg sql.NullString
	var finishedAt sql.NullTime
	var actorID sql.NullInt64
	var atomic int
	var unclaimedInt int
	if err := row.Scan(&in.ID, &in.ClusterID, &in.Chart, &in.Version, &in.ReleaseName, &in.Namespace,
		&hostname, &in.Operation, &in.Status, &atomic, &valuesHash, &in.StartedAt, &finishedAt, &errMsg, &actorID, &unclaimedInt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	in.Atomic = atomic == 1
	if hostname.Valid {
		in.Hostname = &hostname.String
	}
	if valuesHash.Valid {
		in.ValuesHash = &valuesHash.String
	}
	if finishedAt.Valid {
		t := finishedAt.Time
		in.FinishedAt = &t
	}
	if errMsg.Valid {
		in.ErrorMessage = &errMsg.String
	}
	if actorID.Valid {
		v := actorID.Int64
		in.ActorID = &v
	}
	in.HostnameUnclaimed = unclaimedInt == 1
	return &in, nil
}

func (a *Store) ListInstallsForCluster(ctx context.Context, clusterID string, limit int) ([]Install, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := a.s.DB().QueryContext(ctx,
		`SELECT id, cluster_id, chart, version, release_name, namespace, hostname,
		        operation, status, atomic, values_hash, started_at, finished_at, error_message, actor_id, hostname_unclaimed
		   FROM apps_installs WHERE cluster_id = ? ORDER BY started_at DESC LIMIT ?`, clusterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Install
	for rows.Next() {
		var in Install
		var hostname, valuesHash, errMsg sql.NullString
		var finishedAt sql.NullTime
		var actorID sql.NullInt64
		var atomic int
		var unclaimedInt int
		if err := rows.Scan(&in.ID, &in.ClusterID, &in.Chart, &in.Version, &in.ReleaseName, &in.Namespace,
			&hostname, &in.Operation, &in.Status, &atomic, &valuesHash, &in.StartedAt, &finishedAt, &errMsg, &actorID, &unclaimedInt); err != nil {
			return nil, err
		}
		in.Atomic = atomic == 1
		if hostname.Valid {
			in.Hostname = &hostname.String
		}
		if valuesHash.Valid {
			in.ValuesHash = &valuesHash.String
		}
		if finishedAt.Valid {
			t := finishedAt.Time
			in.FinishedAt = &t
		}
		if errMsg.Valid {
			in.ErrorMessage = &errMsg.String
		}
		if actorID.Valid {
			v := actorID.Int64
			in.ActorID = &v
		}
		in.HostnameUnclaimed = unclaimedInt == 1
		out = append(out, in)
	}
	return out, rows.Err()
}

// MarkHostnameUnclaimed flips the hostname_unclaimed flag on the install row.
// Called by the executor's post-helm ingress probe when the requested
// hostname does not appear on any Ingress / IngressRoute in the namespace —
// signals to the UI that the chart likely uses a different value path and
// the operator should override via the Advanced ▸ Hostname value path field.
func (a *Store) MarkHostnameUnclaimed(ctx context.Context, id string, unclaimed bool) error {
	v := 0
	if unclaimed {
		v = 1
	}
	_, err := a.s.DB().ExecContext(ctx,
		`UPDATE apps_installs SET hostname_unclaimed = ? WHERE id = ?`, v, id)
	return err
}

func (a *Store) FinishInstall(ctx context.Context, id, status, errMsg string) error {
	now := time.Now().UTC()
	var em sql.NullString
	if errMsg != "" {
		em = sql.NullString{String: errMsg, Valid: true}
	}
	_, err := a.s.DB().ExecContext(ctx,
		`UPDATE apps_installs SET status = ?, finished_at = ?, error_message = ? WHERE id = ?`,
		status, now, em, id)
	return err
}
