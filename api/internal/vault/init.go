package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	vapi "github.com/hashicorp/vault/api"
)

// ApproleCreds carries the role_id / secret_id pair read from the approle
// file mounted by vault-init. Phase 5 exposes this so the lifetime watcher
// can fall back to a fresh login if token renewal fails.
type ApproleCreds struct {
	RoleID   string `json:"role_id"`
	SecretID string `json:"secret_id"`
}

// TLSConfig carries the mTLS material for the api<->vault channel. Vault's
// listener runs tls_require_and_verify_client_cert, so the api presents
// ClientCert/ClientKey (chaining to CACert) on every call. All three paths are
// supplied by the operator via env (see cmd/api/main.go); an empty CACert
// leaves TLS unconfigured (plaintext — only for a non-TLS dev Vault).
type TLSConfig struct {
	CACert     string
	ClientCert string
	ClientKey  string
}

// LoginAppRole authenticates using credentials at approlePath (mounted from
// the vault-init state volume) and returns a logged-in *vapi.Client, the
// raw login *vapi.Secret (consumed by the lifetime watcher), and the
// ApproleCreds (kept for relogin fallback). The returned client carries the
// mTLS config, so every subsequent call (including relogin) reuses it.
func LoginAppRole(ctx context.Context, addr, approlePath string, tlsCfg TLSConfig) (*vapi.Client, *vapi.Secret, ApproleCreds, error) {
	cfg := vapi.DefaultConfig()
	cfg.Address = addr
	if tlsCfg.CACert != "" {
		if err := cfg.ConfigureTLS(&vapi.TLSConfig{
			CACert:     tlsCfg.CACert,
			ClientCert: tlsCfg.ClientCert,
			ClientKey:  tlsCfg.ClientKey,
		}); err != nil {
			return nil, nil, ApproleCreds{}, fmt.Errorf("configure vault mTLS: %w", err)
		}
	}
	cli, err := vapi.NewClient(cfg)
	if err != nil {
		return nil, nil, ApproleCreds{}, err
	}
	body, err := os.ReadFile(approlePath)
	if err != nil {
		return nil, nil, ApproleCreds{}, fmt.Errorf("read approle file: %w", err)
	}
	var creds ApproleCreds
	if err := json.Unmarshal(body, &creds); err != nil {
		return nil, nil, ApproleCreds{}, fmt.Errorf("parse approle file: %w", err)
	}
	res, err := cli.Logical().WriteWithContext(ctx, "auth/approle/login", map[string]any{
		"role_id":   creds.RoleID,
		"secret_id": creds.SecretID,
	})
	if err != nil {
		return nil, nil, ApproleCreds{}, fmt.Errorf("approle login: %w", err)
	}
	if res == nil || res.Auth == nil || res.Auth.ClientToken == "" {
		return nil, nil, ApproleCreds{}, fmt.Errorf("approle login: no token")
	}
	cli.SetToken(res.Auth.ClientToken)
	return cli, res, creds, nil
}
