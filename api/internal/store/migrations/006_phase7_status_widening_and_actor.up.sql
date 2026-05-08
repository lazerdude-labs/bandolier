-- 006_phase7_status_widening_and_actor.up.sql
-- Phase 7: widen audit_log.outcome CHECK to include 'cancelled' (cancellable
-- long-running operations) and add deployments.actor_id so the Activity view
-- can attribute deploy/destroy/upgrade rows back to the operator.
--
-- SQLite has no DROP CHECK CONSTRAINT, so tables with CHECK columns are
-- rebuilt via the standard rename-recreate-copy pattern (same approach as
-- migration 003). deployments has no CHECK on status, so a plain ALTER TABLE
-- suffices for the actor_id addition. apps_installs.status also gets widened
-- to allow 'cancelled' so the apps cancel handler can land its terminal row.

ALTER TABLE audit_log RENAME TO _audit_log_old;

CREATE TABLE audit_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    -- actor_id intentionally uses SQLite's implicit ON DELETE RESTRICT:
    -- audit linkage must not be silently broken if a user row is deleted.
    actor_id    INTEGER REFERENCES users(id),
    action      TEXT    NOT NULL,
    target      TEXT,
    outcome     TEXT    NOT NULL CHECK (outcome IN ('success','failure','started','succeeded','failed','cancelled')),
    ts          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    details     TEXT
);

INSERT INTO audit_log (id, actor_id, action, target, outcome, ts, details)
SELECT id, actor_id, action, target, outcome, ts, details FROM _audit_log_old;

DROP TABLE _audit_log_old;

CREATE INDEX IF NOT EXISTS audit_log_ts_idx ON audit_log(ts);
CREATE INDEX IF NOT EXISTS audit_log_action_idx ON audit_log(action);

ALTER TABLE deployments ADD COLUMN actor_id INTEGER REFERENCES users(id);

-- apps_installs status widening — same rename-recreate dance.
ALTER TABLE apps_installs RENAME TO _apps_installs_old;

CREATE TABLE apps_installs (
  id TEXT PRIMARY KEY,
  cluster_id TEXT NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
  chart TEXT NOT NULL,
  version TEXT NOT NULL,
  release_name TEXT NOT NULL,
  namespace TEXT NOT NULL,
  hostname TEXT,
  operation TEXT NOT NULL CHECK (operation IN ('install','upgrade','uninstall')),
  status TEXT NOT NULL CHECK (status IN ('running','succeeded','failed','cancelled')),
  atomic INTEGER NOT NULL DEFAULT 1,
  values_hash TEXT,
  started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  finished_at TIMESTAMP,
  error_message TEXT,
  actor_id INTEGER,
  log_path TEXT,
  hostname_unclaimed INTEGER NOT NULL DEFAULT 0
);

INSERT INTO apps_installs (
  id, cluster_id, chart, version, release_name, namespace, hostname,
  operation, status, atomic, values_hash, started_at, finished_at,
  error_message, actor_id, log_path, hostname_unclaimed
)
SELECT id, cluster_id, chart, version, release_name, namespace, hostname,
       operation, status, atomic, values_hash, started_at, finished_at,
       error_message, actor_id, log_path, hostname_unclaimed
FROM _apps_installs_old;

DROP TABLE _apps_installs_old;

CREATE INDEX IF NOT EXISTS idx_apps_installs_cluster ON apps_installs(cluster_id);
CREATE INDEX IF NOT EXISTS idx_apps_installs_status ON apps_installs(status);
