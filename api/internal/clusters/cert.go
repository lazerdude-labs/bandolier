package clusters

import (
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/lazerdude-labs/bandolier/api/internal/vault"
)

// CertBundle is a wildcard TLS bundle issued by Vault PKI: leaf cert, key, the
// chain of issuing CAs (intermediate first, root last — the order Vault
// returns), and the leaf's NotAfter expressed as wall-clock time.
type CertBundle struct {
	Certificate string
	PrivateKey  string
	CAChain     []string
	ExpiresAt   time.Time
}

// IssueWildcardCert asks Vault PKI to issue a wildcard cert for *.fqdn (with
// the bare fqdn as a SAN) using the provided role. The role's max_ttl/policy
// must allow an 8760h (1y) lease.
//
// Vault returns the leaf in `certificate`, the key in `private_key`, the chain
// in `ca_chain`, and the leaf's NotAfter as a Unix timestamp in `expiration`.
// We're defensive about the chain shape (string vs []any vs []string returned
// by the SDK depending on backend version) and the expiration shape (int64 vs
// float64 — JSON unmarshaling into map[string]any tends to give float64).
func IssueWildcardCert(ctx context.Context, v *vault.Client, fqdn, pkiRole string) (*CertBundle, error) {
	if fqdn == "" {
		return nil, fmt.Errorf("issue wildcard: fqdn is empty")
	}
	if pkiRole == "" {
		return nil, fmt.Errorf("issue wildcard: pki role is empty")
	}
	resp, err := v.WriteRaw(ctx, "pki/issue/"+pkiRole, map[string]any{
		"common_name": "*." + fqdn,
		"alt_names":   fqdn,
		"ttl":         "8760h",
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("issue wildcard: empty response")
	}

	cert, _ := resp["certificate"].(string)
	key, _ := resp["private_key"].(string)
	if cert == "" || key == "" {
		return nil, fmt.Errorf("issue wildcard: missing certificate or private_key in response")
	}

	var chain []string
	switch v := resp["ca_chain"].(type) {
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				chain = append(chain, s)
			}
		}
	case []string:
		chain = append(chain, v...)
	case string:
		if v != "" {
			chain = append(chain, v)
		}
	}

	var expiresAt time.Time
	switch v := resp["expiration"].(type) {
	case float64:
		expiresAt = time.Unix(int64(v), 0).UTC()
	case int64:
		expiresAt = time.Unix(v, 0).UTC()
	case int:
		expiresAt = time.Unix(int64(v), 0).UTC()
	}

	return &CertBundle{
		Certificate: cert,
		PrivateKey:  key,
		CAChain:     chain,
		ExpiresAt:   expiresAt,
	}, nil
}

// PersistWildcardCert writes the issued bundle to Vault KV at
// clusters/<id>/wildcard_cert so subsequent deploys / Traefik reinstalls can
// re-push without re-issuing. The chain is stored as a single concatenated
// string (PEM blocks separated by newlines) — kubectl/Traefik don't care about
// boundaries, only that the bytes parse as PEM.
func PersistWildcardCert(ctx context.Context, v *vault.Client, clusterID string, b *CertBundle) error {
	if b == nil {
		return fmt.Errorf("persist wildcard: nil bundle")
	}
	chainJoined := ""
	for i, c := range b.CAChain {
		if i > 0 {
			chainJoined += "\n"
		}
		chainJoined += c
	}
	return v.Put(ctx, "clusters/"+clusterID+"/wildcard_cert", map[string]any{
		"certificate": b.Certificate,
		"private_key": b.PrivateKey,
		"ca_chain":    chainJoined,
		"expires_at":  b.ExpiresAt.Format(time.RFC3339),
	})
}

// PushCertSecret renders a kubernetes.io/tls Secret manifest for the wildcard
// bundle and applies it to the cluster as `bandolier-wildcard-tls` in
// kube-system. Pipes through `kubectl apply -f -` against the supplied
// kubeconfig. Idempotent — re-runs replace the existing secret data.
//
// kubectl is used (rather than client-go) because the deploy goroutine already
// has a temp kubeconfig path on disk from the helm factory, and the rest of
// the deploy steps shell out the same way (helm, ansible, terraform).
func PushCertSecret(ctx context.Context, kubeconfigPath string, b *CertBundle) error {
	if b == nil {
		return fmt.Errorf("push cert secret: nil bundle")
	}
	if kubeconfigPath == "" {
		return fmt.Errorf("push cert secret: empty kubeconfig path")
	}
	crt := base64.StdEncoding.EncodeToString([]byte(b.Certificate))
	key := base64.StdEncoding.EncodeToString([]byte(b.PrivateKey))
	manifest := strings.Join([]string{
		"apiVersion: v1",
		"kind: Secret",
		"metadata:",
		"  name: bandolier-wildcard-tls",
		"  namespace: kube-system",
		"type: kubernetes.io/tls",
		"data:",
		"  tls.crt: " + crt,
		"  tls.key: " + key,
		"",
	}, "\n")

	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl apply secret: %w: %s", err, string(out))
	}
	return nil
}

// shouldRenew is the renewal trigger: re-issue when within 7 days of expiry.
func shouldRenew(expires, now time.Time) bool {
	return expires.Sub(now) < 7*24*time.Hour
}

// ListReadyClusterer is the narrow store surface RenewLoop needs. The
// production *store.Store satisfies this via the new ListReadyClusters method.
type ListReadyClusterer interface {
	ListReadyClusters(ctx context.Context) ([]string, error)
}

// HelmKubeconfigOnly is what RenewLoop needs from a Helm — just the kubeconfig
// path, so it can pass it to PushCertSecret. Avoids importing apps (cycle).
type HelmKubeconfigOnly interface {
	KubeconfigFile() string
}

// HelmFactoryLike mirrors apps.HelmFactory shape but returns the narrow
// HelmKubeconfigOnly interface, avoiding a cycle on the apps package.
type HelmFactoryLike interface {
	For(ctx context.Context, clusterID string) (HelmKubeconfigOnly, func(), error)
}

// RenewLoop watches all `ready` clusters' wildcard certs hourly and re-issues
// at the 7-day expiry threshold. Blocks until ctx is canceled.
//
// auditWrite is passed in to avoid a circular import on the audit package.
func RenewLoop(
	ctx context.Context,
	store ListReadyClusterer,
	v *vault.Client,
	helmFactory HelmFactoryLike,
	auditWrite func(ctx context.Context, action, target string, details map[string]any),
) {
	tick := time.NewTicker(time.Hour)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			renewOnce(ctx, store, v, helmFactory, auditWrite)
		}
	}
}

func renewOnce(
	ctx context.Context,
	store ListReadyClusterer,
	v *vault.Client,
	helmFactory HelmFactoryLike,
	auditWrite func(ctx context.Context, action, target string, details map[string]any),
) {
	clusters, err := store.ListReadyClusters(ctx)
	if err != nil {
		return
	}
	for _, cid := range clusters {
		data, err := v.Get(ctx, "clusters/"+cid+"/wildcard_cert")
		if err != nil || data == nil {
			continue
		}
		expStr, _ := data["expires_at"].(string)
		expires, err := time.Parse(time.RFC3339, expStr)
		if err != nil {
			continue
		}
		if !shouldRenew(expires, time.Now().UTC()) {
			continue
		}
		netData, _ := v.Get(ctx, vault.Paths{}.Network(cid))
		fqdn, _ := netData["fqdn"].(string)
		tlsData, _ := v.Get(ctx, "clusters/"+cid+"/tls")
		pkiRole, _ := tlsData["pki_role"].(string)
		bundle, err := IssueWildcardCert(ctx, v, fqdn, pkiRole)
		if err != nil {
			auditWrite(ctx, "cluster_cert_renew", cid, map[string]any{"error": err.Error(), "outcome": "failed"})
			continue
		}
		if err := PersistWildcardCert(ctx, v, cid, bundle); err != nil {
			continue
		}
		helm, done, err := helmFactory.For(ctx, cid)
		if err != nil {
			continue
		}
		_ = PushCertSecret(ctx, helm.KubeconfigFile(), bundle)
		done()
		auditWrite(ctx, "cluster_cert_renew", cid, map[string]any{
			"old_expires_at": expStr,
			"new_expires_at": bundle.ExpiresAt.Format(time.RFC3339),
		})
	}
}
