CREATE TABLE apps_repos (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  cluster_id TEXT NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  url TEXT NOT NULL,
  added_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  added_by INTEGER,
  UNIQUE(cluster_id, name)
);

CREATE INDEX idx_apps_repos_cluster ON apps_repos(cluster_id);

CREATE TABLE apps_installs (
  id TEXT PRIMARY KEY,
  cluster_id TEXT NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
  chart TEXT NOT NULL,
  version TEXT NOT NULL,
  release_name TEXT NOT NULL,
  namespace TEXT NOT NULL,
  hostname TEXT,
  operation TEXT NOT NULL CHECK (operation IN ('install','upgrade','uninstall')),
  status TEXT NOT NULL CHECK (status IN ('running','succeeded','failed')),
  atomic INTEGER NOT NULL DEFAULT 1,
  values_hash TEXT,
  started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  finished_at TIMESTAMP,
  error_message TEXT,
  actor_id INTEGER,
  log_path TEXT
);

CREATE INDEX idx_apps_installs_cluster ON apps_installs(cluster_id);
CREATE INDEX idx_apps_installs_status ON apps_installs(status);
