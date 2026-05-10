// Package proxmox provides direct Proxmox HTTP API helpers used by Bandolier
// outside of terraform — currently the wizard's "Test reachability" preflight
// that validates an operator-supplied set of credentials before the cluster
// is saved.
//
// This package intentionally duplicates the TLS-config helper in
// internal/telemetry/proxmox.go for now; both should move to a shared
// internal/proxmox/client.go in a future refactor.
package proxmox

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// TestRequest carries the operator-supplied wizard values to validate against
// a live Proxmox host. Fields mirror the proxmox section of the initialize
// request, plus the three storage names so we can verify each one independently.
type TestRequest struct {
	Endpoint        string
	TokenID         string
	TokenSecret     string
	Node            string
	Storage         string // VM disks
	ImageStorage    string
	SnippetsStorage string
	CABundle        string // optional PEM; empty → InsecureSkipVerify
}

// Check is one validation result. Status is always either "ok" or "fail";
// "skip" is reserved for future checks that may be conditional.
type Check struct {
	Name   string `json:"name"`
	Label  string `json:"label"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// TestResult bundles the per-check results plus an overall ok flag (true iff
// every check is "ok").
type TestResult struct {
	OK     bool    `json:"ok"`
	Checks []Check `json:"checks"`
}

// RunTests executes the validation suite against the given Proxmox host. It
// stops short-circuit on the first auth failure (no point checking storages
// against a host that won't accept the token) but otherwise runs all checks
// independently so the operator gets the full picture in one click.
func RunTests(ctx context.Context, req TestRequest) TestResult {
	client, clientErr := buildHTTPClient(req.CABundle)
	var checks []Check

	if clientErr != nil {
		return TestResult{
			OK: false,
			Checks: []Check{{
				Name: "tls_config", Label: "TLS configuration",
				Status: "fail", Detail: clientErr.Error(),
			}},
		}
	}

	// Compose endpoint base + auth header once.
	base := strings.TrimSuffix(req.Endpoint, "/")
	authHdr := fmt.Sprintf("PVEAPIToken=%s=%s", req.TokenID, req.TokenSecret)

	// 1. Endpoint reachable + token authenticates (combined).
	versionBody, vErr := apiGet(ctx, client, base+"/api2/json/version", authHdr)
	if vErr != nil {
		checks = append(checks, Check{
			Name: "proxmox_reachable", Label: "Proxmox API reachable + token authenticates",
			Status: "fail", Detail: vErr.Error(),
		})
		// Without a working endpoint there's nothing else to check.
		return TestResult{OK: false, Checks: checks}
	}
	checks = append(checks, Check{
		Name: "proxmox_reachable", Label: "Proxmox API reachable + token authenticates",
		Status: "ok", Detail: extractVersion(versionBody),
	})

	// 2. Node accessible. PathEscape the operator-supplied node name so a
	// crafted value like "pve/../../version" can't traverse to a different
	// API endpoint and silently produce a fake "ok" result. The operator is
	// authenticated and could bypass the check via the initialize endpoint
	// regardless, but a fake-OK from this preflight is also a correctness
	// bug (operator thinks their config is valid when it isn't).
	if _, err := apiGet(ctx, client, base+"/api2/json/nodes/"+url.PathEscape(req.Node)+"/status", authHdr); err != nil {
		checks = append(checks, Check{
			Name: "node_exists", Label: fmt.Sprintf("Node %q accessible", req.Node),
			Status: "fail", Detail: err.Error(),
		})
	} else {
		checks = append(checks, Check{
			Name: "node_exists", Label: fmt.Sprintf("Node %q accessible", req.Node),
			Status: "ok",
		})
	}

	// 3-5. Storage-content checks. Each fetches /storage/<name>/status which
	// includes the configured `content` list; we verify the required type is
	// present.
	checks = append(checks, checkStorageContent(ctx, client, base, authHdr, req.Node,
		"vm_disk_storage", "VM disk storage", req.Storage, "images"))
	checks = append(checks, checkStorageContent(ctx, client, base, authHdr, req.Node,
		"image_storage", "Image storage", req.ImageStorage, "iso"))
	checks = append(checks, checkStorageContent(ctx, client, base, authHdr, req.Node,
		"snippets_storage", "Snippets storage", req.SnippetsStorage, "snippets"))

	ok := true
	for _, c := range checks {
		if c.Status != "ok" {
			ok = false
			break
		}
	}
	return TestResult{OK: ok, Checks: checks}
}

// checkStorageContent returns an "ok" Check iff the named Proxmox storage
// exists on the node AND its `content` list includes the required type. On
// failure (404, missing content type, etc.) the Detail tells the operator
// the precise pvesm command to fix the most common case.
//
// Path segments (node + storage) are url.PathEscape'd to keep operator-
// supplied values from traversing the URL — see RunTests for the rationale.
func checkStorageContent(ctx context.Context, client *http.Client, base, authHdr, node,
	name, labelBase, storage, requiredContent string) Check {
	label := fmt.Sprintf("%s %q has %q content type", labelBase, storage, requiredContent)
	body, err := apiGet(ctx, client,
		base+"/api2/json/nodes/"+url.PathEscape(node)+"/storage/"+url.PathEscape(storage)+"/status", authHdr)
	if err != nil {
		return Check{Name: name, Label: label, Status: "fail", Detail: err.Error()}
	}
	contents, parseErr := extractStorageContents(body)
	if parseErr != nil {
		return Check{Name: name, Label: label, Status: "fail", Detail: parseErr.Error()}
	}
	if !contains(contents, requiredContent) {
		// Build the pvesm command without a leading comma when the existing
		// content list is empty (rare — newly-created dir storage with no
		// content types yet). With a leading comma, pvesm rejects the
		// command.
		existing := strings.Join(contents, ",")
		var fix string
		if existing == "" {
			fix = fmt.Sprintf("pvesm set %s --content %s", storage, requiredContent)
		} else {
			fix = fmt.Sprintf("pvesm set %s --content %s,%s", storage, existing, requiredContent)
		}
		return Check{
			Name: name, Label: label, Status: "fail",
			Detail: fmt.Sprintf("storage exists but its content list (%v) does not include %q. Enable with: %s",
				contents, requiredContent, fix),
		}
	}
	return Check{Name: name, Label: label, Status: "ok"}
}

// apiGet wraps a single authenticated GET against the Proxmox REST API. It
// returns the response body on 2xx and a descriptive error otherwise. The
// 5s timeout per request keeps the wizard responsive when Proxmox is slow
// or unreachable; the caller's context governs the overall budget.
//
// Note on error contents: a transport-level failure produces a *url.Error
// whose .Error() includes the full request URL but NOT the Authorization
// header (Go's net/http never reflects request headers in error strings).
// The token never appears in any error wrapped by this function. If a
// future logger ever surfaces these errors, that property still holds.
func apiGet(ctx context.Context, client *http.Client, url, authHdr string) ([]byte, error) {
	rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", authHdr)
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(res.Body)
	// 401 = bad token (wrong secret, expired, revoked) — never a permission
	// issue, so don't surface a permission hint that would misdirect the
	// operator. 403 = "Permission check failed" against a specific ACL path,
	// which permissionHint is shaped for.
	if res.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("HTTP 401 (token unauthorized — wrong/expired/revoked): %s", truncate(string(body), 200))
	}
	if res.StatusCode == http.StatusForbidden {
		if hint := permissionHint(string(body)); hint != "" {
			return nil, fmt.Errorf("HTTP 403: %s", hint)
		}
		return nil, fmt.Errorf("HTTP 403 (token missing permission): %s", truncate(string(body), 200))
	}
	if res.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("HTTP 404 (resource not found): %s", truncate(string(body), 200))
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", res.StatusCode, truncate(string(body), 200))
	}
	return body, nil
}

func extractVersion(body []byte) string {
	var resp struct {
		Data struct {
			Version string `json:"version"`
			Release string `json:"release"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "(version unparseable)"
	}
	if resp.Data.Release != "" {
		return "PVE " + resp.Data.Version + " (" + resp.Data.Release + ")"
	}
	if resp.Data.Version != "" {
		return "PVE " + resp.Data.Version
	}
	return "(no version field)"
}

// extractStorageContents parses the comma-separated `content` field from a
// /storage/<name>/status response. Proxmox returns it as a single string like
// "iso,vztmpl,backup,snippets".
func extractStorageContents(body []byte) ([]string, error) {
	var resp struct {
		Data struct {
			Content string `json:"content"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse storage status: %w", err)
	}
	parts := strings.Split(resp.Data.Content, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out, nil
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// permissionHintRe matches the Proxmox 403 body pattern
// `Permission check failed (<path>, <priv1>|<priv2>|...)` and captures the
// ACL path and the missing privilege list. Proxmox emits this in the response
// body for any token-vs-ACL mismatch; the path is what we use to recommend
// the right pveum command.
var permissionHintRe = regexp.MustCompile(`Permission check failed \(([^,]+),\s*([^)]+)\)`)

// permPathRe / permPrivRe define the strict shape we accept for captured
// values. Anything outside these classes is rejected and we fall back to
// the raw-body error path. Reason: the captured values get embedded into a
// suggested shell command in the operator-facing detail field. Even though
// React text-renders the detail (no XSS) and JSON-marshals it (no header
// injection), an attacker-controlled-or-misconfigured Proxmox could craft a
// 403 body whose captured "privs" group includes plausible-looking but
// malicious shell snippets that the operator might copy-paste. Limiting
// captures to ACL path / privilege identifiers makes that vector untenable.
//
// Length caps are belt-and-suspenders against pathological inputs (e.g. a
// 5MB body where the entire body falls inside one regex group); apiGet
// already reads bodies into memory unbounded so this is more about not
// emitting a multi-line error than DoS.
var (
	permPathRe = regexp.MustCompile(`^/[a-zA-Z0-9/_.-]{1,128}$`)
	permPrivRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9.|_-]{0,127}$`)
)

// permissionHint converts a Proxmox 403 body into an actionable single-line
// hint with the exact pveum command to fix the missing ACL. Returns "" when
// the body doesn't match the recognized pattern OR when captured values
// fall outside the strict permPathRe / permPrivRe shapes; callers fall back
// to the raw body in that case.
//
// The recommended role is chosen by ACL path prefix:
//   - /storage/* → PVEDatastoreAdmin (covers Datastore.Audit + AllocateSpace
//     + AllocateTemplate, all of which the wizard's storage-content checks
//     and Bandolier's terraform need).
//   - /nodes/*   → PVEAuditor for the read-only preflight check; deploy
//     requires PVEVMAdmin which we mention but don't lead with.
//   - other      → no role recommendation; the raw privilege list is shown.
//
// We deliberately don't substitute the operator's actual token ID into the
// command — the wizard knows it but baking it into log lines or surfaced
// errors invites it leaking via screenshots/CI logs. The operator can
// substitute their own.
func permissionHint(body string) string {
	m := permissionHintRe.FindStringSubmatch(body)
	if m == nil {
		return ""
	}
	path := strings.TrimSpace(m[1])
	privs := strings.TrimSpace(m[2])
	if !permPathRe.MatchString(path) || !permPrivRe.MatchString(privs) {
		return ""
	}
	switch {
	case strings.HasPrefix(path, "/storage/"):
		return fmt.Sprintf("token missing %s on %s. Fix on Proxmox: `pveum acl modify %s --tokens '<your-token-id>' --roles PVEDatastoreAdmin --propagate 1` (or grant %s individually).",
			"PVEDatastoreAdmin", path, path, privs)
	case strings.HasPrefix(path, "/nodes/"):
		return fmt.Sprintf("token missing privilege on %s (needs %s). Fix on Proxmox: `pveum acl modify %s --tokens '<your-token-id>' --roles PVEAuditor --propagate 1` for the preflight; deploy also needs PVEVMAdmin on this path.",
			path, privs, path)
	default:
		return fmt.Sprintf("token missing privilege on %s (needs %s). Grant via `pveum acl modify %s --tokens '<your-token-id>' --roles <Role> --propagate 1`.",
			path, privs, path)
	}
}

// buildHTTPClient mirrors internal/telemetry/proxmox.go's helper. Empty
// CABundle yields InsecureSkipVerify (accepted homelab risk; documented in
// THREAT_MODEL.md). A bundle is parsed strictly: every block must be a
// CERTIFICATE, otherwise we error rather than silently trust extra material.
func buildHTTPClient(caBundle string) (*http.Client, error) {
	tlsCfg, err := buildTLSConfig(caBundle)
	if err != nil {
		return nil, err
	}
	return &http.Client{Transport: &http.Transport{TLSClientConfig: tlsCfg}}, nil
}

func buildTLSConfig(caBundle string) (*tls.Config, error) {
	if caBundle == "" {
		return &tls.Config{InsecureSkipVerify: true}, nil //nolint:gosec // see THREAT_MODEL.md
	}
	pool := x509.NewCertPool()
	rest := []byte(caBundle)
	found := 0
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("CA bundle has non-certificate block type %q", block.Type)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse cert in CA bundle: %w", err)
		}
		pool.AddCert(cert)
		found++
	}
	if found == 0 {
		return nil, fmt.Errorf("malformed CA bundle PEM")
	}
	return &tls.Config{RootCAs: pool}, nil
}
