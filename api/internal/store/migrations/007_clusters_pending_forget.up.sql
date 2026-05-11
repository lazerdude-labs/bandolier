-- 007_clusters_pending_forget.up.sql
-- v0.1.7: add clusters.pending_forget so Forget can safely cascade through a
-- Destroy on live clusters. The DELETE handler sets pending_forget = 1 when
-- the operator requests cascade=destroy against a `ready` or `degraded`
-- cluster; the deployments executor's runDestroy success path reads it on
-- completion and invokes the existing Forget path (Vault purge + DB row
-- drop). Destroy failures clear the flag so the operator can investigate
-- without auto-forgetting a partially-destroyed cluster.
--
-- Plain ALTER TABLE is sufficient — no CHECK constraint on this column, so
-- the rebuild-via-rename dance from migration 003/006 isn't needed.

ALTER TABLE clusters ADD COLUMN pending_forget INTEGER NOT NULL DEFAULT 0;
