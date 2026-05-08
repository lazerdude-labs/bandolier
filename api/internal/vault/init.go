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

// LoginAppRole authenticates using credentials at approlePath (mounted from
// the vault-init state volume) and returns a logged-in *vapi.Client, the
// raw login *vapi.Secret (consumed by the lifetime watcher), and the
// ApproleCreds (kept for relogin fallback).
func LoginAppRole(ctx context.Context, addr, approlePath string) (*vapi.Client, *vapi.Secret, ApproleCreds, error) {
	cfg := vapi.DefaultConfig()
	cfg.Address = addr
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
