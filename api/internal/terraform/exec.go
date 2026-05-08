package terraform

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hashicorp/terraform-exec/tfexec"
)

// Runner wraps tfexec for one cluster's working directory.
type Runner struct {
	workdir string
	tf      *tfexec.Terraform
}

func New(workdir, tfBinary string) (*Runner, error) {
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return nil, err
	}
	tf, err := tfexec.NewTerraform(workdir, tfBinary)
	if err != nil {
		return nil, fmt.Errorf("new terraform: %w", err)
	}
	return &Runner{workdir: workdir, tf: tf}, nil
}

// statePreserve lists workdir entries we never delete during a CopyModule
// sync — they hold real terraform state that must outlive a re-deploy.
var statePreserve = map[string]bool{
	".terraform":               true, // wiped by Init() later, but not here
	".terraform.lock.hcl":      true, // ditto
	"terraform.tfstate":        true, // REAL state, must persist
	"terraform.tfstate.backup": true,
	"terraform.tfvars.json":    true, // rewritten each deploy by WriteVars
}

// CopyModule makes the workdir match the source module dir.
//
// Files/dirs in workdir that don't exist in src are removed (orphan cleanup),
// EXCEPT for entries in statePreserve. This prevents leftover .tf files from
// previous module versions (e.g. an old backend.tf) from polluting init.
func (r *Runner) CopyModule(src string) error {
	// 1. Remove orphans at the top level of workdir.
	entries, err := os.ReadDir(r.workdir)
	if err != nil {
		return fmt.Errorf("read workdir: %w", err)
	}
	for _, e := range entries {
		if statePreserve[e.Name()] {
			continue
		}
		// Skip entries also in statePreserve via path skip list.
		if _, err := os.Stat(filepath.Join(src, e.Name())); err == nil {
			// Source has this entry; copyDir will overwrite.
			continue
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat src %s: %w", e.Name(), err)
		}
		// Source does NOT have this entry → delete from workdir.
		if err := os.RemoveAll(filepath.Join(r.workdir, e.Name())); err != nil {
			return fmt.Errorf("remove orphan %s: %w", e.Name(), err)
		}
	}
	// 2. Copy everything from src (filtered by pathsToSkip).
	return copyDir(src, r.workdir)
}

// WriteVars writes a terraform.tfvars.json file with arbitrary values.
func (r *Runner) WriteVars(vars map[string]any) error {
	body, err := json.Marshal(vars)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.workdir, "terraform.tfvars.json"), body, 0o600)
}

// Init wraps tf init.
//
// Removes any cached .terraform/ directory first so a re-init never inherits
// stale backend config from a previous module that lived in this workdir.
// This is safe because the actual state lives in workdir/terraform.tfstate
// (local backend), which is preserved.
func (r *Runner) Init(ctx context.Context, stdout, stderr io.Writer) error {
	if err := os.RemoveAll(filepath.Join(r.workdir, ".terraform")); err != nil {
		return fmt.Errorf("clean .terraform: %w", err)
	}
	if err := os.Remove(filepath.Join(r.workdir, ".terraform.lock.hcl")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clean .terraform.lock.hcl: %w", err)
	}
	r.tf.SetStdout(stdout)
	r.tf.SetStderr(stderr)
	return r.tf.Init(ctx, tfexec.Upgrade(false))
}

// Apply wraps tf apply -auto-approve.
func (r *Runner) Apply(ctx context.Context, stdout, stderr io.Writer) error {
	r.tf.SetStdout(stdout)
	r.tf.SetStderr(stderr)
	return r.tf.Apply(ctx)
}

// Destroy wraps tf destroy -auto-approve.
func (r *Runner) Destroy(ctx context.Context, stdout, stderr io.Writer) error {
	r.tf.SetStdout(stdout)
	r.tf.SetStderr(stderr)
	return r.tf.Destroy(ctx)
}

// Output returns the parsed terraform outputs.
func (r *Runner) Output(ctx context.Context) (map[string]tfexec.OutputMeta, error) {
	return r.tf.Output(ctx)
}

// pathsToSkip lists module-source paths that must never be copied into a
// per-cluster workdir. These are runtime artifacts that, if imported, would
// cause terraform to think it's already initialized with a previous backend
// or to overwrite real state.
var pathsToSkip = map[string]bool{
	".terraform":          true,
	".terraform.lock.hcl": true,
	"terraform.tfstate":   true,
	"terraform.tfstate.backup": true,
	"crash.log":           true,
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if rel == "." {
			return nil
		}
		// Skip terraform runtime artifacts at any level.
		head := rel
		if i := filepath.Separator; i != 0 {
			if idx := indexSep(rel); idx >= 0 {
				head = rel[:idx]
			}
		}
		if pathsToSkip[head] {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, body, info.Mode())
	})
}

func indexSep(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == filepath.Separator {
			return i
		}
	}
	return -1
}
