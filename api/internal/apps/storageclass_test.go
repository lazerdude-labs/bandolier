package apps

import "testing"

func TestParseStorageClassJSON(t *testing.T) {
	const sample = `{
	  "items": [
	    {
	      "metadata": {
	        "name": "longhorn",
	        "annotations": {"storageclass.kubernetes.io/is-default-class": "true"}
	      },
	      "provisioner": "driver.longhorn.io"
	    },
	    {
	      "metadata": {"name": "local-path"},
	      "provisioner": "rancher.io/local-path"
	    }
	  ]
	}`

	got, err := parseStorageClassJSON([]byte(sample))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 classes, got %d", len(got))
	}
	if got[0].Name != "longhorn" || got[0].Provisioner != "driver.longhorn.io" || !got[0].IsDefault {
		t.Errorf("class[0] wrong: %+v", got[0])
	}
	if got[1].Name != "local-path" || got[1].IsDefault {
		t.Errorf("class[1] wrong: %+v", got[1])
	}
}

func TestParseStorageClassJSONEmpty(t *testing.T) {
	got, err := parseStorageClassJSON(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d", len(got))
	}
}

func TestValidStorageClassName(t *testing.T) {
	valid := []string{"longhorn", "local-path", "nfs.csi.k8s.io", "a", "sc1"}
	for _, s := range valid {
		if !validStorageClassName(s) {
			t.Errorf("expected %q to be valid", s)
		}
	}
	// Reject empty, helm --set injection vectors, and shell metacharacters.
	invalid := []string{
		"",
		"longhorn,foo.admin.password=evil", // helm --set comma injection
		"longhorn=x",
		"Longhorn",       // uppercase
		"sc name",        // space
		"-leading",       // leading dash
		"trailing-",      // trailing dash
		"$(whoami)",      // shell meta
		"a;b",            // semicolon
		"line\nbreak",    // newline
	}
	for _, s := range invalid {
		if validStorageClassName(s) {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}
