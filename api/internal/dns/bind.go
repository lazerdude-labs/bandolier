// api/internal/dns/bind.go
package dns

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Bind is the production BIND9 provider. Uses nsupdate (RFC 2136) signed
// with TSIG. The binary lives at /usr/bin/nsupdate (installed by Dockerfile
// via bind9-dnsutils).
type Bind struct {
	cfg Config
}

func NewBind(cfg Config) *Bind { return &Bind{cfg: cfg} }

func (b *Bind) Upsert(ctx context.Context, r Record) error {
	if r.TTL == 0 {
		r.TTL = 300
	}
	r.Name = normalizeName(r.Name, b.cfg.Zone)
	script := buildUpsertScript(b.cfg.Server, b.cfg.Zone, r)
	return b.runScript(ctx, script)
}

func (b *Bind) Delete(ctx context.Context, name, recordType string) error {
	name = normalizeName(name, b.cfg.Zone)
	script := buildDeleteScript(b.cfg.Server, b.cfg.Zone, name, recordType)
	return b.runScript(ctx, script)
}

func (b *Bind) Healthy(ctx context.Context) error {
	// Send an empty zone-show to verify reachability + TSIG auth.
	script := fmt.Sprintf("server %s %s\nzone %s\nshow\nsend\n",
		stripPort(b.cfg.Server), portOf(b.cfg.Server), b.cfg.Zone)
	return b.runScript(ctx, script)
}

func (b *Bind) runScript(ctx context.Context, script string) error {
	args := []string{}
	// TSIG is passed via a temp key file (mode 0600) instead of -y on argv.
	// Argv is world-visible via /proc/<pid>/cmdline, ps, and audit logs;
	// a key file keeps the secret off the process list.
	if b.cfg.TSIGName != "" && b.cfg.TSIGSecret != "" {
		kf, err := os.CreateTemp("", "tsig-*.key")
		if err != nil {
			return fmt.Errorf("temp key: %w", err)
		}
		keyFile := kf.Name()
		defer os.Remove(keyFile)
		// BIND key file format:
		//   key "<name>" {
		//       algorithm hmac-sha256;
		//       secret "<base64-secret>";
		//   };
		keyBody := fmt.Sprintf("key \"%s\" {\n\talgorithm hmac-sha256;\n\tsecret \"%s\";\n};\n", b.cfg.TSIGName, b.cfg.TSIGSecret)
		if _, err := kf.WriteString(keyBody); err != nil {
			kf.Close()
			return fmt.Errorf("write key: %w", err)
		}
		if err := kf.Chmod(0o600); err != nil {
			kf.Close()
			return fmt.Errorf("chmod key: %w", err)
		}
		kf.Close()
		args = append(args, "-k", keyFile)
	}
	// Hard timeout — nsupdate can hang waiting for a response.
	tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(tctx, "nsupdate", args...)
	cmd.Stdin = strings.NewReader(script)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nsupdate: %w: %s", err, out.String())
	}
	return nil
}

// normalizeName ensures a name is fully-qualified relative to the zone and
// terminated with a trailing dot (RFC 1035). Idempotent.
func normalizeName(name, zone string) string {
	name = strings.TrimSuffix(name, ".")
	zone = strings.TrimSuffix(zone, ".")
	if !strings.HasSuffix(name, "."+zone) && name != zone {
		name = name + "." + zone
	}
	return name + "."
}

// buildUpsertScript writes nsupdate stdin for an idempotent upsert: delete
// any existing records of the same name+type, then add the new one. This is
// the canonical "set this record" pattern with nsupdate.
func buildUpsertScript(server, zone string, r Record) string {
	host, port := parseServer(server)
	var b strings.Builder
	fmt.Fprintf(&b, "server %s %s\n", host, port)
	fmt.Fprintf(&b, "zone %s\n", zone)
	fmt.Fprintf(&b, "update delete %s %s\n", r.Name, r.Type)
	fmt.Fprintf(&b, "update add %s %d %s %s\n", r.Name, r.TTL, r.Type, r.Data)
	fmt.Fprintf(&b, "send\n")
	return b.String()
}

func buildDeleteScript(server, zone, name, recordType string) string {
	host, port := parseServer(server)
	var b strings.Builder
	fmt.Fprintf(&b, "server %s %s\n", host, port)
	fmt.Fprintf(&b, "zone %s\n", zone)
	fmt.Fprintf(&b, "update delete %s %s\n", name, recordType)
	fmt.Fprintf(&b, "send\n")
	return b.String()
}

// parseServer splits "host:port" into (host, port). Defaults to port 53.
func parseServer(s string) (host, port string) {
	if idx := strings.LastIndex(s, ":"); idx > 0 {
		return s[:idx], s[idx+1:]
	}
	return s, "53"
}

func stripPort(s string) string { h, _ := parseServer(s); return h }
func portOf(s string) string    { _, p := parseServer(s); return p }
