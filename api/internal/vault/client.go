package vault

import (
	"context"
	"fmt"
	"sync"
	"time"

	vapi "github.com/hashicorp/vault/api"
)

type Client struct {
	api         *vapi.Client
	mount       string
	mu          sync.Mutex
	lastRenewed time.Time
}

func NewClient(api *vapi.Client, mount string) *Client {
	return &Client{api: api, mount: mount}
}

func (c *Client) Put(ctx context.Context, path string, data map[string]any) error {
	_, err := c.api.KVv2(c.mount).Put(ctx, path, data)
	if err != nil {
		return fmt.Errorf("vault put %s: %w", path, err)
	}
	return nil
}

func (c *Client) Get(ctx context.Context, path string) (map[string]any, error) {
	res, err := c.api.KVv2(c.mount).Get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("vault get %s: %w", path, err)
	}
	if res == nil || res.Data == nil {
		return nil, fmt.Errorf("vault get %s: no data", path)
	}
	return res.Data, nil
}

func (c *Client) Delete(ctx context.Context, path string) error {
	if err := c.api.KVv2(c.mount).DeleteMetadata(ctx, path); err != nil {
		return fmt.Errorf("vault delete %s: %w", path, err)
	}
	return nil
}

// WriteRaw writes to a Vault logical path, bypassing the bandolier KV mount
// helpers. Required for non-KV engines such as PKI's pki/issue/<role>, where
// the path is interpreted directly by Vault rather than the KV mount.
func (c *Client) WriteRaw(ctx context.Context, path string, data map[string]any) (map[string]any, error) {
	s, err := c.api.Logical().WriteWithContext(ctx, path, data)
	if err != nil {
		return nil, fmt.Errorf("vault write %s: %w", path, err)
	}
	if s == nil || s.Data == nil {
		return nil, nil
	}
	return s.Data, nil
}

// HealthInfo carries Vault status fields exposed by the API for the
// settings/health page. All fields besides Sealed/Initialized are
// best-effort and may be empty if the Vault SDK doesn't populate them.
type HealthInfo struct {
	Sealed      bool   `json:"sealed"`
	Initialized bool   `json:"initialized"`
	Version     string `json:"version,omitempty"`
	ClusterName string `json:"cluster_name,omitempty"`
	Type        string `json:"type,omitempty"`        // shamir, transit, awskms…
	AuthMethod  string `json:"auth_method,omitempty"` // AppRole — how this client logs in
}

// Sealed queries the Vault seal-status endpoint and reports whether Vault is
// currently sealed. Returns (true, nil) when sealed, (false, nil) when
// unsealed, and (true, err) when the query itself fails — callers should treat
// an unknown state as sealed for safety.
func (c *Client) Sealed(ctx context.Context) (bool, error) {
	status, err := c.api.Sys().SealStatusWithContext(ctx)
	if err != nil {
		return true, fmt.Errorf("vault seal-status: %w", err)
	}
	return status.Sealed, nil
}

// Health returns a richer status snapshot used by the /api/health endpoint.
// On error, returns a fail-safe HealthInfo (sealed=true, all other fields
// empty) plus the error — so the handler can log + fall through.
func (c *Client) Health(ctx context.Context) (*HealthInfo, error) {
	status, err := c.api.Sys().SealStatusWithContext(ctx)
	if err != nil {
		return &HealthInfo{Sealed: true, AuthMethod: "approle"}, fmt.Errorf("vault seal-status: %w", err)
	}
	return &HealthInfo{
		Sealed:      status.Sealed,
		Initialized: status.Initialized,
		Version:     status.Version,
		ClusterName: status.ClusterName,
		Type:        status.Type,
		AuthMethod:  "approle",
	}, nil
}
