package terraform_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lazerdude-labs/bandolier/api/internal/terraform"
)

func TestInitMinimalModule(t *testing.T) {
	tfBin, err := exec.LookPath("terraform")
	if err != nil {
		t.Skip("terraform not in PATH")
	}

	src := t.TempDir()
	module := []byte(`variable "x" {
  type    = string
  default = ""
}

output "got" {
  value = var.x
}
`)
	if err := os.WriteFile(filepath.Join(src, "main.tf"), module, 0o644); err != nil {
		t.Fatal(err)
	}

	work := t.TempDir()
	r, err := terraform.New(work, tfBin)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.CopyModule(src); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteVars(map[string]any{"x": "hello"}); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := r.Init(context.Background(), &stdout, &stderr); err != nil {
		t.Fatalf("init: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}
}
