package homelab

import (
	"strings"
	"testing"
)

func TestCatalogHasRocky9(t *testing.T) {
	d, ok := Catalog["rocky9"]
	if !ok {
		t.Fatal("Catalog missing rocky9 entry")
	}
	if d.ID != "rocky9" {
		t.Errorf("ID: got %q want %q", d.ID, "rocky9")
	}
	if d.Label == "" {
		t.Error("Label is empty")
	}
	if len(d.URLs) == 0 {
		t.Fatal("URLs is empty")
	}
	for i, u := range d.URLs {
		if !strings.HasPrefix(u, "https://") {
			t.Errorf("URLs[%d] not https: %q", i, u)
		}
		if !strings.HasSuffix(u, ".qcow2") {
			t.Errorf("URLs[%d] not qcow2: %q", i, u)
		}
	}
	if len(d.SHA256) != 64 {
		t.Errorf("SHA256 length: got %d want 64", len(d.SHA256))
	}
	if d.FileName == "" {
		t.Error("FileName is empty")
	}
}

func TestResolveCatalogEntry(t *testing.T) {
	d, err := ResolveImage("rocky9", "", "")
	if err != nil {
		t.Fatalf("ResolveImage(rocky9): %v", err)
	}
	if d.ID != "rocky9" {
		t.Errorf("ID: got %q want %q", d.ID, "rocky9")
	}
	if d.SHA256 == "" {
		t.Error("SHA256 empty after resolve")
	}
	if len(d.URLs) == 0 {
		t.Error("URLs empty after resolve")
	}
}

func TestResolveCustomURL(t *testing.T) {
	url := "https://example.com/path/to/myimage.qcow2"
	sha := strings.Repeat("a", 64)
	d, err := ResolveImage("", url, sha)
	if err != nil {
		t.Fatalf("ResolveImage(custom): %v", err)
	}
	if len(d.URLs) != 1 || d.URLs[0] != url {
		t.Errorf("URLs: got %v want [%q]", d.URLs, url)
	}
	if d.SHA256 != sha {
		t.Errorf("SHA256: got %q want %q", d.SHA256, sha)
	}
	// Proxmox rejects .qcow2 under content_type=iso; FileName is rewritten to .img.
	if d.FileName != "myimage.img" {
		t.Errorf("FileName: got %q want %q", d.FileName, "myimage.img")
	}
}

func TestProxmoxSafeFileNameRewritesDiskImageExtensions(t *testing.T) {
	cases := map[string]string{
		"foo.qcow2":       "foo.img",
		"bar.qcow":        "bar.img",
		"baz.raw":         "baz.img",
		"already.img":     "already.img",
		"unknown.iso":     "unknown.iso",
		"no-extension":    "no-extension",
		"deep.path.qcow2": "deep.path.img",
	}
	for in, want := range cases {
		if got := proxmoxSafeFileName(in); got != want {
			t.Errorf("proxmoxSafeFileName(%q): got %q want %q", in, got, want)
		}
	}
}

func TestResolveRejectsNoArgs(t *testing.T) {
	if _, err := ResolveImage("", "", ""); err == nil {
		t.Error("expected error for empty args, got nil")
	}
}

func TestResolveRejectsBoth(t *testing.T) {
	sha := strings.Repeat("b", 64)
	if _, err := ResolveImage("rocky9", "https://example.com/foo.qcow2", sha); err == nil {
		t.Error("expected error when both distroID and customURL set, got nil")
	}
}

func TestResolveRejectsCustomURLWithoutSHA(t *testing.T) {
	if _, err := ResolveImage("", "https://example.com/foo.qcow2", ""); err == nil {
		t.Error("expected error for customURL without sha, got nil")
	}
}

func TestResolveRejectsUnknownDistro(t *testing.T) {
	if _, err := ResolveImage("freebsd99", "", ""); err == nil {
		t.Error("expected error for unknown distroID, got nil")
	}
}
