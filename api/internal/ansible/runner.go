package ansible

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// Event is a parsed ansible-runner event.
type Event struct {
	UUID   string         `json:"uuid"`
	Event  string         `json:"event"`
	Data   map[string]any `json:"event_data"`
	Stdout string         `json:"stdout"`
}

// Runner runs ansible-runner against a private_data_dir layout it builds itself.
type Runner struct {
	binary string // "ansible-runner"
	root   string // private_data_dir
}

func New(binary, root string) *Runner {
	if binary == "" {
		binary = "ansible-runner"
	}
	return &Runner{binary: binary, root: root}
}

// Prepare creates the private_data_dir layout from a playbook source dir,
// inventory string, and extra-vars JSON. Layout matches ansible-runner's
// expectations: project/, inventory/, env/extravars.
func (r *Runner) Prepare(playbookSrcDir, inventory string, extraVars map[string]any) error {
	for _, p := range []string{"project", "inventory", "env"} {
		if err := os.MkdirAll(filepath.Join(r.root, p), 0o755); err != nil {
			return err
		}
	}
	if err := copyDir(playbookSrcDir, filepath.Join(r.root, "project")); err != nil {
		return fmt.Errorf("copy project: %w", err)
	}
	if err := os.WriteFile(filepath.Join(r.root, "inventory", "hosts.ini"), []byte(inventory), 0o600); err != nil {
		return err
	}
	body, err := json.Marshal(extraVars)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(r.root, "env", "extravars"), body, 0o600); err != nil {
		return err
	}
	return nil
}

// Run executes ansible-runner with --json events. Calls onEvent for each event
// and returns the exit code from the playbook.
func (r *Runner) Run(ctx context.Context, playbookFile string, onEvent func(Event), stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, r.binary,
		"run", r.root,
		"--json",
		"-p", playbookFile)
	cmd.Stdout = io.MultiWriter(stdout, &eventTeeWriter{OnEvent: onEvent})
	cmd.Stderr = stderr
	return cmd.Run()
}

type eventTeeWriter struct {
	OnEvent func(Event)
	buf     []byte
}

func (w *eventTeeWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		idx := bytesIndexByte(w.buf, '\n')
		if idx < 0 {
			break
		}
		line := w.buf[:idx]
		w.buf = w.buf[idx+1:]
		var e Event
		if err := json.Unmarshal(line, &e); err == nil && e.Event != "" {
			w.OnEvent(e)
		}
	}
	return len(p), nil
}

func bytesIndexByte(b []byte, c byte) int {
	for i, v := range b {
		if v == c {
			return i
		}
	}
	return -1
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
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, body, info.Mode())
	})
}

// scanLines is a stand-in helper if needed elsewhere.
var _ = bufio.ScanLines

// TestEventWriter is exported for tests only.
type TestEventWriter = eventTeeWriter
