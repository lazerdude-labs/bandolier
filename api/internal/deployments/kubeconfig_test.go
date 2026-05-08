package deployments

import (
	"strings"
	"testing"
)

func TestRewriteServerAddress(t *testing.T) {
	in := "apiVersion: v1\nclusters:\n- cluster:\n    server: https://127.0.0.1:6443\n    certificate-authority-data: AAA\n  name: default\n"
	out := rewriteServerAddress(in, "192.0.2.21")
	if !strings.Contains(out, "https://192.0.2.21:6443") {
		t.Fatalf("rewrite failed: %s", out)
	}
	if strings.Contains(out, "127.0.0.1") {
		t.Fatalf("127.0.0.1 still present: %s", out)
	}
}

func TestRewriteServerAddress_AcceptsZeroAddr(t *testing.T) {
	in := "    server: https://0.0.0.0:6443"
	out := rewriteServerAddress(in, "192.0.2.21")
	if !strings.Contains(out, "https://192.0.2.21:6443") {
		t.Fatalf("rewrite failed: %s", out)
	}
}
