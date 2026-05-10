package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeProxmox stands up a small subset of the Proxmox REST API for tests.
// Behavior is configurable per-call via fields on the struct so each test
// can simulate the exact failure mode it cares about.
type fakeProxmox struct {
	tokenSecret string
	versionResp string // 200 body for GET /api2/json/version
	nodes       map[string]bool
	storages    map[string]string // name -> "iso,images,snippets" etc.
	failAuth    bool
}

func (f *fakeProxmox) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if f.failAuth || !strings.HasSuffix(auth, "="+f.tokenSecret) {
			http.Error(w, `{"errors":{"":"invalid token"}}`, http.StatusUnauthorized)
			return
		}
		switch {
		case r.URL.Path == "/api2/json/version":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(f.versionResp))
		case strings.HasPrefix(r.URL.Path, "/api2/json/nodes/") && strings.HasSuffix(r.URL.Path, "/status"):
			parts := strings.Split(r.URL.Path, "/")
			// /api2/json/nodes/<node>/status               (5 segments after split -> idx 4)
			// /api2/json/nodes/<node>/storage/<name>/status (7 segments)
			if len(parts) == 6 {
				node := parts[4]
				if !f.nodes[node] {
					http.Error(w, `{"errors":{"":"node not found"}}`, http.StatusNotFound)
					return
				}
				_, _ = fmt.Fprintf(w, `{"data":{"node":%q,"uptime":12345}}`, node)
				return
			}
			if len(parts) == 8 {
				node := parts[4]
				storage := parts[6]
				if !f.nodes[node] {
					http.Error(w, `{"errors":{"":"node not found"}}`, http.StatusNotFound)
					return
				}
				content, ok := f.storages[storage]
				if !ok {
					http.Error(w, `{"errors":{"":"storage not found"}}`, http.StatusNotFound)
					return
				}
				_, _ = fmt.Fprintf(w, `{"data":{"storage":%q,"content":%q,"type":"dir"}}`, storage, content)
				return
			}
			http.Error(w, `{"errors":{"":"unhandled path"}}`, http.StatusNotFound)
		default:
			http.Error(w, `{"errors":{"":"not found"}}`, http.StatusNotFound)
		}
	})
}

// newFake stands up a small subset of the Proxmox REST API for tests.
// httptest.NewTLSServer generates a self-signed cert; RunTests called with
// CABundle="" hits the InsecureSkipVerify branch in buildHTTPClient and
// accepts that cert. Any test that needs a strict-TLS path would have to
// thread the server's CA bundle through, which we don't currently exercise
// (the strict-TLS branch is covered by TestBuildTLSConfig_* unit tests).
func newFake(t *testing.T) (*fakeProxmox, *httptest.Server) {
	t.Helper()
	f := &fakeProxmox{
		tokenSecret: "00000000-0000-0000-0000-000000000000",
		versionResp: `{"data":{"version":"8.2.4","release":"8.2","repoid":"abc"}}`,
		nodes:       map[string]bool{"pve": true},
		storages: map[string]string{
			"local-lvm": "images,rootdir",
			"local":     "iso,vztmpl,backup",
			"cephfs":    "iso,vztmpl,backup,snippets",
		},
	}
	srv := httptest.NewTLSServer(f.handler())
	t.Cleanup(srv.Close)
	return f, srv
}

func TestRunTests_AllChecksPass(t *testing.T) {
	f, srv := newFake(t)
	got := RunTests(context.Background(), TestRequest{
		Endpoint:        srv.URL,
		TokenID:         "root@pam!t",
		TokenSecret:     f.tokenSecret,
		Node:            "pve",
		Storage:         "local-lvm",
		ImageStorage:    "local",
		SnippetsStorage: "cephfs",
	})
	if !got.OK {
		t.Fatalf("expected OK, got %s", mustJSON(got))
	}
	if len(got.Checks) != 5 {
		t.Fatalf("expected 5 checks, got %d: %s", len(got.Checks), mustJSON(got))
	}
	for _, c := range got.Checks {
		if c.Status != "ok" {
			t.Errorf("check %q expected ok, got %s (%s)", c.Name, c.Status, c.Detail)
		}
	}
	// Version detail should mention 8.2
	if !strings.Contains(got.Checks[0].Detail, "8.2") {
		t.Errorf("expected version detail to include 8.2, got %q", got.Checks[0].Detail)
	}
}

func TestRunTests_BadToken_ShortCircuits(t *testing.T) {
	_, srv := newFake(t)
	got := RunTests(context.Background(), TestRequest{
		Endpoint:        srv.URL,
		TokenID:         "root@pam!t",
		TokenSecret:     "wrong-secret",
		Node:            "pve",
		Storage:         "local-lvm",
		ImageStorage:    "local",
		SnippetsStorage: "cephfs",
	})
	if got.OK {
		t.Fatal("expected OK=false on bad token")
	}
	// On endpoint/auth failure we short-circuit; only one check returned.
	if len(got.Checks) != 1 {
		t.Fatalf("expected short-circuit (1 check), got %d: %s", len(got.Checks), mustJSON(got))
	}
	if got.Checks[0].Name != "proxmox_reachable" || got.Checks[0].Status != "fail" {
		t.Fatalf("unexpected first check: %+v", got.Checks[0])
	}
}

func TestRunTests_NodeMissing(t *testing.T) {
	f, srv := newFake(t)
	got := RunTests(context.Background(), TestRequest{
		Endpoint:        srv.URL,
		TokenID:         "root@pam!t",
		TokenSecret:     f.tokenSecret,
		Node:            "does-not-exist",
		Storage:         "local-lvm",
		ImageStorage:    "local",
		SnippetsStorage: "cephfs",
	})
	if got.OK {
		t.Fatal("expected OK=false on missing node")
	}
	// First check should still pass (token works); node check should fail.
	if got.Checks[0].Status != "ok" {
		t.Errorf("proxmox_reachable should pass even with bad node, got %s", got.Checks[0].Status)
	}
	if got.Checks[1].Name != "node_exists" || got.Checks[1].Status != "fail" {
		t.Fatalf("expected node_exists to fail, got %+v", got.Checks[1])
	}
}

func TestRunTests_SnippetsContentMissing(t *testing.T) {
	f, srv := newFake(t)
	// Point snippets at "local" which has only iso,vztmpl,backup — no snippets
	got := RunTests(context.Background(), TestRequest{
		Endpoint:        srv.URL,
		TokenID:         "root@pam!t",
		TokenSecret:     f.tokenSecret,
		Node:            "pve",
		Storage:         "local-lvm",
		ImageStorage:    "local",
		SnippetsStorage: "local",
	})
	if got.OK {
		t.Fatal("expected OK=false when snippets not enabled")
	}
	// Find the snippets check
	var snippets *Check
	for i := range got.Checks {
		if got.Checks[i].Name == "snippets_storage" {
			snippets = &got.Checks[i]
		}
	}
	if snippets == nil {
		t.Fatal("snippets_storage check missing from result")
	}
	if snippets.Status != "fail" {
		t.Fatalf("expected snippets_storage to fail, got %s (%s)", snippets.Status, snippets.Detail)
	}
	// Detail should include the pvesm fix command
	if !strings.Contains(snippets.Detail, "pvesm set local --content") {
		t.Errorf("expected fix-command hint in detail, got %q", snippets.Detail)
	}
}

func TestRunTests_AllStoragesMissing(t *testing.T) {
	f, srv := newFake(t)
	got := RunTests(context.Background(), TestRequest{
		Endpoint:        srv.URL,
		TokenID:         "root@pam!t",
		TokenSecret:     f.tokenSecret,
		Node:            "pve",
		Storage:         "nonexistent-vm",
		ImageStorage:    "nonexistent-img",
		SnippetsStorage: "nonexistent-snip",
	})
	if got.OK {
		t.Fatal("expected OK=false")
	}
	storageChecks := []string{"vm_disk_storage", "image_storage", "snippets_storage"}
	for _, name := range storageChecks {
		found := false
		for _, c := range got.Checks {
			if c.Name == name {
				found = true
				if c.Status != "fail" {
					t.Errorf("%s expected fail, got %s", name, c.Status)
				}
			}
		}
		if !found {
			t.Errorf("%s check missing from result", name)
		}
	}
}

func TestBuildTLSConfig_Empty(t *testing.T) {
	cfg, err := buildTLSConfig("")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !cfg.InsecureSkipVerify {
		t.Fatal("empty CA bundle should yield InsecureSkipVerify=true")
	}
}

func TestBuildTLSConfig_RejectsNonCertBlock(t *testing.T) {
	// A PEM block with valid base64 body but a non-CERTIFICATE type. The
	// body is base64("hello") so pem.Decode succeeds and we reach the
	// type-check branch we want to exercise.
	priv := "-----BEGIN PRIVATE KEY-----\naGVsbG8=\n-----END PRIVATE KEY-----\n"
	_, err := buildTLSConfig(priv)
	if err == nil {
		t.Fatal("expected error for non-CERTIFICATE PEM block")
	}
	if !strings.Contains(err.Error(), "non-certificate") {
		t.Errorf("expected non-certificate error, got %v", err)
	}
}

func TestBuildTLSConfig_RejectsMalformedPEM(t *testing.T) {
	_, err := buildTLSConfig("not-pem-at-all")
	if err == nil {
		t.Fatal("expected error for malformed PEM")
	}
	if !strings.Contains(err.Error(), "malformed CA bundle") {
		t.Errorf("expected malformed-bundle error, got %v", err)
	}
}

func TestPermissionHint(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string // substring that must appear; "" means hint should be empty
	}{
		{
			name: "storage path → suggests PVEDatastoreAdmin",
			body: `Permission check failed (/storage/local-lvm, Datastore.Audit|Datastore.AllocateSpace)`,
			want: "PVEDatastoreAdmin",
		},
		{
			name: "storage path → includes pveum acl modify",
			body: `Permission check failed (/storage/local, Datastore.Audit)`,
			want: "pveum acl modify /storage/local",
		},
		{
			name: "node path → suggests PVEAuditor for preflight",
			body: `Permission check failed (/nodes/pve, Sys.Audit)`,
			want: "PVEAuditor",
		},
		{
			name: "node path → mentions PVEVMAdmin for deploy",
			body: `Permission check failed (/nodes/pve, Sys.Audit)`,
			want: "PVEVMAdmin",
		},
		{
			name: "unrecognized path → still surfaces a generic hint",
			body: `Permission check failed (/access/groups, Sys.Audit)`,
			want: "/access/groups",
		},
		{
			name: "non-permission body returns empty",
			body: `{"errors":{"":"some other failure"}}`,
			want: "",
		},
		{
			// The captured `privs` group falls outside permPrivRe — must not
			// be reflected. Defends against a Proxmox-or-MITM emitting a
			// crafted body that injects a misleading pveum command into the
			// hint that the operator might copy-paste.
			name: "shell-injection-in-privs is rejected",
			body: "Permission check failed (/storage/local, X; rm -rf /)",
			want: "",
		},
		{
			// Captured `path` outside permPathRe (contains spaces) — reject.
			name: "weird path is rejected",
			body: "Permission check failed (/storage/local with spaces, Datastore.Audit)",
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := permissionHint(tc.body)
			if tc.want == "" {
				if got != "" {
					t.Errorf("expected empty hint, got %q", got)
				}
				return
			}
			if !strings.Contains(got, tc.want) {
				t.Errorf("expected hint to contain %q, got %q", tc.want, got)
			}
		})
	}
}

// TestRunTests_PermissionDeniedSurfacesFix wires a fake Proxmox that returns
// the real-world 403 body the bpg provider/Proxmox spits out when the token
// lacks Datastore.Audit on a storage. The detail should mention the role
// fix, not the raw permission body.
func TestRunTests_PermissionDeniedSurfacesFix(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/version" {
			_, _ = w.Write([]byte(`{"data":{"version":"8.2.4","release":"8.2"}}`))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api2/json/nodes/pve/storage/") {
			http.Error(w, `Permission check failed (/storage/local-lvm, Datastore.Audit|Datastore.AllocateSpace)`, http.StatusForbidden)
			return
		}
		// Node /status: pretend ok so we reach storage checks.
		if strings.HasSuffix(r.URL.Path, "/status") {
			_, _ = w.Write([]byte(`{"data":{"node":"pve"}}`))
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	got := RunTests(context.Background(), TestRequest{
		Endpoint:        srv.URL,
		TokenID:         "root@pam!t",
		TokenSecret:     "any",
		Node:            "pve",
		Storage:         "local-lvm",
		ImageStorage:    "local",
		SnippetsStorage: "cephfs",
	})
	if got.OK {
		t.Fatal("expected OK=false")
	}
	var storage *Check
	for i := range got.Checks {
		if got.Checks[i].Name == "vm_disk_storage" {
			storage = &got.Checks[i]
		}
	}
	if storage == nil {
		t.Fatal("vm_disk_storage check missing")
	}
	if !strings.Contains(storage.Detail, "PVEDatastoreAdmin") {
		t.Errorf("expected PVEDatastoreAdmin role in detail, got %q", storage.Detail)
	}
	if !strings.Contains(storage.Detail, "pveum acl modify /storage/local-lvm") {
		t.Errorf("expected pveum acl modify command in detail, got %q", storage.Detail)
	}
}

// TestRunTests_401_DoesNotSuggestACLFix wires a fake Proxmox that returns
// a 401 with a body that happens to also mention "Permission check failed".
// 401 = bad token (wrong/expired/revoked); a permission-hint that suggests
// `pveum acl modify` would actively misdirect the operator. The detail
// must NOT recommend a role grant in this case.
func TestRunTests_401_DoesNotSuggestACLFix(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 401 even on /version so RunTests fails on the auth check.
		http.Error(w, `{"errors":{"":"invalid token"}} Permission check failed (/storage/local, Datastore.Audit)`, http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)
	got := RunTests(context.Background(), TestRequest{
		Endpoint:        srv.URL,
		TokenID:         "root@pam!t",
		TokenSecret:     "any",
		Node:            "pve",
		Storage:         "local-lvm",
		ImageStorage:    "local",
		SnippetsStorage: "cephfs",
	})
	if got.OK {
		t.Fatal("expected OK=false on 401")
	}
	if !strings.Contains(got.Checks[0].Detail, "401") {
		t.Errorf("expected detail to flag 401, got %q", got.Checks[0].Detail)
	}
	if strings.Contains(got.Checks[0].Detail, "pveum acl modify") {
		t.Errorf("401 detail must not suggest pveum acl fix; got %q", got.Checks[0].Detail)
	}
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// Compile-time check that we don't accidentally drop tls.Config field that
// production needs.
var _ = (*tls.Config)(nil)
