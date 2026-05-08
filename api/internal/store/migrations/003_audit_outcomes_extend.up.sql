-- 003_audit_outcomes_extend.up.sql
-- Phase 2: extend Outcome CHECK to allow started | succeeded | failed.
-- SQLite has no DROP CHECK CONSTRAINT, so rename-recreate-copy.

ALTER TABLE audit_log RENAME TO _audit_log_old;

CREATE TABLE audit_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    -- actor_id intentionally uses SQLite's implicit ON DELETE RESTRICT:
    -- audit linkage must not be silently broken if a user row is deleted.
    actor_id    INTEGER REFERENCES users(id),
    action      TEXT    NOT NULL,
    target      TEXT,
    outcome     TEXT    NOT NULL CHECK (outcome IN ('success','failure','started','succeeded','failed')),
    ts          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    details     TEXT
);

INSERT INTO audit_log (id, actor_id, action, target, outcome, ts, details)
SELECT id, actor_id, action, target, outcome, ts, details FROM _audit_log_old;

DROP TABLE _audit_log_old;

CREATE INDEX IF NOT EXISTS audit_log_ts_idx ON audit_log(ts);
CREATE INDEX IF NOT EXISTS audit_log_action_idx ON audit_log(action);
