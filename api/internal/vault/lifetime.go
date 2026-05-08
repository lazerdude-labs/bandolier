package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	vapi "github.com/hashicorp/vault/api"
)

// TokenStatus snapshots the live state of the Vault client token: the
// remaining TTL (from a fresh lookup-self) and the wall-clock time of the
// last successful renew/relogin (cached on the Client). Used by /api/health
// so the Settings page can surface token health to the operator.
type TokenStatus struct {
	TTLSeconds  int64     `json:"ttl_seconds"`
	LastRenewed time.Time `json:"last_renewed"`
}

// AuditWriter is the narrow shape Phase 5 needs from the audit package.
// Defined here so vault doesn't import audit (cycle: audit ← store, and
// store is below vault in the dep graph).
type AuditWriter func(ctx context.Context, action string, details map[string]any)

// actionVaultTokenRenew mirrors audit.ActionVaultTokenRenew. We don't import
// audit from vault to keep the package boundary clean — the string value is
// the contract.
const actionVaultTokenRenew = "vault_token_renew"

// TokenStatus reads the current TTL via lookup-self and combines it with the
// cached LastRenewed timestamp. On lookup failure, returns zero TTL but the
// cached LastRenewed — so a transient Vault hiccup doesn't blank the UI row.
func (c *Client) TokenStatus(ctx context.Context) TokenStatus {
	c.mu.Lock()
	last := c.lastRenewed
	c.mu.Unlock()

	res, err := c.api.Auth().Token().LookupSelfWithContext(ctx)
	if err != nil || res == nil || res.Data == nil {
		return TokenStatus{TTLSeconds: 0, LastRenewed: last}
	}
	// Vault returns ttl as a json.Number; coerce safely.
	var ttl int64
	switch v := res.Data["ttl"].(type) {
	case json.Number:
		if n, err := v.Int64(); err == nil {
			ttl = n
		}
	case float64:
		ttl = int64(v)
	case int64:
		ttl = v
	}
	return TokenStatus{TTLSeconds: ttl, LastRenewed: last}
}

// markRenewed stamps the wall-clock time of a successful renewal/relogin.
func (c *Client) markRenewed() {
	c.mu.Lock()
	c.lastRenewed = time.Now().UTC()
	c.mu.Unlock()
}

// StartLifetimeWatcher kicks off a background goroutine that keeps the
// AppRole-issued client token renewed via Vault's LifetimeWatcher. If the
// watcher's DoneCh fires (renewal failed or token expired), the loop falls
// back to a fresh AppRole login using creds and updates the client token
// in place. Both successful renewals and re-logins emit a vault_token_renew
// audit row via auditWrite.
//
// The initial loginSecret is the *vapi.Secret returned by LoginAppRole — the
// watcher needs its lease metadata to schedule the first renewal. The
// pointer is updated in-place when relogin succeeds so the next watcher
// iteration uses the new lease.
func (c *Client) StartLifetimeWatcher(
	ctx context.Context,
	logger *slog.Logger,
	loginSecret *vapi.Secret,
	creds ApproleCreds,
	auditWrite AuditWriter,
) error {
	if loginSecret == nil || loginSecret.Auth == nil {
		return fmt.Errorf("vault watcher: login secret missing auth")
	}
	c.markRenewed()
	go c.runWatcherLoop(ctx, logger, loginSecret, creds, auditWrite)
	return nil
}

func (c *Client) runWatcherLoop(
	ctx context.Context,
	logger *slog.Logger,
	loginSecret *vapi.Secret,
	creds ApproleCreds,
	auditWrite AuditWriter,
) {
	for {
		watcher, err := c.api.NewLifetimeWatcher(&vapi.LifetimeWatcherInput{
			Secret: loginSecret,
		})
		if err != nil {
			logger.Error("vault watcher: NewLifetimeWatcher failed", "err", err)
			if !c.relogin(ctx, logger, loginSecret, creds, auditWrite) {
				return
			}
			continue
		}

		go watcher.Start()

		// Keep this watcher live for all renewal events; only DoneCh tears it
		// down. The Vault SDK's LifetimeWatcher is long-lived: RenewCh fires
		// on every renewal, not once. Re-arming on each RenewCh would spawn
		// a fresh goroutine per renewal cycle and may schedule renewals from
		// the wrong base TTL.
	watchLoop:
		for {
			select {
			case <-ctx.Done():
				watcher.Stop()
				return

			case renewal := <-watcher.RenewCh():
				c.markRenewed()
				details := map[string]any{"source": "renew"}
				if renewal != nil && renewal.Secret != nil && renewal.Secret.Auth != nil {
					details["lease_duration"] = renewal.Secret.Auth.LeaseDuration
				}
				auditWrite(ctx, actionVaultTokenRenew, details)
				logger.Info("vault watcher: renewal succeeded", "source", "renew")
				// Update loginSecret in-place so a future re-arm (after
				// DoneCh) uses the latest lease metadata.
				if renewal != nil && renewal.Secret != nil {
					*loginSecret = *renewal.Secret
				}

			case err := <-watcher.DoneCh():
				watcher.Stop()
				if err != nil {
					logger.Warn("vault watcher: renewal stopped, falling back to relogin", "err", err)
				} else {
					logger.Info("vault watcher: lease at end-of-life, relogin")
				}
				break watchLoop
			}
		}

		if !c.relogin(ctx, logger, loginSecret, creds, auditWrite) {
			return
		}
	}
}

// relogin performs a fresh AppRole login with exponential backoff. On
// success it swaps the client's token, marks renewed, emits an audit row,
// and updates *loginSecret in place. Returns true on success, false after
// 5 consecutive failures (caller should exit the watcher goroutine).
func (c *Client) relogin(
	ctx context.Context,
	logger *slog.Logger,
	loginSecret *vapi.Secret,
	creds ApproleCreds,
	auditWrite AuditWriter,
) bool {
	backoff := time.Second
	const maxBackoff = 60 * time.Second
	const maxAttempts = 5

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return false
		}
		res, err := c.api.Logical().WriteWithContext(ctx, "auth/approle/login", map[string]any{
			"role_id":   creds.RoleID,
			"secret_id": creds.SecretID,
		})
		if err == nil && res != nil && res.Auth != nil && res.Auth.ClientToken != "" {
			c.api.SetToken(res.Auth.ClientToken)
			c.markRenewed()
			auditWrite(ctx, actionVaultTokenRenew, map[string]any{
				"source":         "relogin",
				"lease_duration": res.Auth.LeaseDuration,
			})
			logger.Info("vault watcher: relogin succeeded", "attempt", attempt)
			*loginSecret = *res
			return true
		}
		logger.Warn("vault watcher: relogin failed", "attempt", attempt, "err", err)

		// Sleep with backoff, but bail out promptly if shutdown fires.
		select {
		case <-ctx.Done():
			return false
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
	logger.Error("vault watcher: giving up after relogin attempts", "attempts", maxAttempts)
	return false
}
