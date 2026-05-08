-- 002_audit_password.up.sql
-- Plan 2 Phase 1: audit_log for security-relevant actions (initially: change_password).

CREATE TABLE IF NOT EXISTS audit_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    -- actor_id intentionally uses SQLite's implicit ON DELETE RESTRICT:
    -- audit linkage must not be silently broken if a user row is deleted.
    actor_id    INTEGER REFERENCES users(id),
    action      TEXT    NOT NULL,
    target      TEXT,
    outcome     TEXT    NOT NULL CHECK (outcome IN ('success','failure')),
    ts          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    details     TEXT
);

CREATE INDEX IF NOT EXISTS audit_log_ts_idx ON audit_log(ts);
CREATE INDEX IF NOT EXISTS audit_log_action_idx ON audit_log(action);
