package deployments

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lazerdude-labs/bandolier/api/internal/ansible"
	"github.com/lazerdude-labs/bandolier/api/internal/apps"
	"github.com/lazerdude-labs/bandolier/api/internal/audit"
	"github.com/lazerdude-labs/bandolier/api/internal/auth"
	"github.com/lazerdude-labs/bandolier/api/internal/clusters"
	"github.com/lazerdude-labs/bandolier/api/internal/dns"
	"github.com/lazerdude-labs/bandolier/api/internal/profiles"
	"github.com/lazerdude-labs/bandolier/api/internal/store"
	"github.com/lazerdude-labs/bandolier/api/internal/terraform"
	"github.com/lazerdude-labs/bandolier/api/internal/vault"
)

type Executor struct {
	Store        *store.Store
	Vault        *vault.Client
	Hub          *Hub
	Mutex        *ClusterMutex
	AppsHelm     apps.HelmFactory
	Registry     *profiles.Registry
	TfStateRoot  string // /var/lib/bandolier/tf-state
	LogRoot      string // /var/lib/bandolier/logs
	TerraformBin string // path to terraform binary
	AnsibleBin   string // "ansible-runner"

	// cancelMap tracks the cancel func for each in-flight deployment so the
	// HTTP cancel handler can signal the goroutine to stop. Entries are
	// added by the spawn site (Deploy/Destroy/Upgrade) and removed by the
	// goroutine's defer regardless of how it terminates.
	cancelMu  sync.Mutex
	cancelMap map[string]context.CancelFunc
}

func NewDeploymentID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// RecoverOrphanedDeployments finds deployments left in `running` state from
// a previous api process (killed by a container restart, crash, etc.), marks
// them `failed`, and flips their cluster to `error`. Idempotent: safe to call
// at every api startup.
func (e *Executor) RecoverOrphanedDeployments(ctx context.Context) error {
	orphans, err := e.Store.ListRunningDeployments(ctx)
	if err != nil {
		return fmt.Errorf("list running deployments: %w", err)
	}
	for _, d := range orphans {
		_ = e.Store.FinishDeployment(ctx, d.ID, "failed", "api restart orphaned deployment")
		_ = e.Store.UpdateClusterStatus(ctx, d.ClusterID, string(clusters.StatusError))
	}
	return nil
}

// Deploy runs the full pipeline for the given cluster. It returns the
// deployment ID immediately and runs the work in a goroutine.
func (e *Executor) Deploy(ctx context.Context, clusterID string) (string, error) {
	cluster, err := e.Store.GetCluster(ctx, clusterID)
	if err != nil {
		return "", err
	}
	if err := clusters.CanTransition(clusters.Status(cluster.Status), clusters.StatusDeploying); err != nil {
		return "", err
	}
	prof, err := e.Registry.Get(cluster.Profile)
	if err != nil {
		return "", err
	}
	if !e.Mutex.TryLock(clusterID) {
		return "", fmt.Errorf("deployment in progress")
	}

	depID := NewDeploymentID()
	logPath := filepath.Join(e.LogRoot, depID+".log")
	_ = os.MkdirAll(e.LogRoot, 0o755)
	logFile, err := os.Create(logPath)
	if err != nil {
		e.Mutex.Unlock(clusterID)
		return "", err
	}

	actorID := actorIDFromCtx(ctx)
	if err := e.Store.CreateDeployment(ctx, &store.Deployment{
		ID: depID, ClusterID: clusterID, Operation: "deploy", Status: "running",
		ActorID: actorIDPtr(actorID),
	}); err != nil {
		_ = logFile.Close()
		e.Mutex.Unlock(clusterID)
		return "", err
	}
	if err := e.Store.UpdateClusterStatus(ctx, clusterID, "deploying"); err != nil {
		_ = logFile.Close()
		e.Mutex.Unlock(clusterID)
		return "", err
	}

	_, _ = audit.Write(ctx, e.Store, audit.Entry{
		ActorID: actorID,
		Action:  string(audit.ActionClusterDeploy),
		Target:  clusterID,
		Outcome: audit.OutcomeStarted,
	})

	runCtx, cancelRun := context.WithCancel(context.Background())
	e.register(depID, cancelRun)
	go e.run(runCtx, depID, clusterID, prof, logFile, actorID)
	return depID, nil
}

func (e *Executor) run(ctx context.Context, depID, clusterID string, prof profiles.Profile, logFile *os.File, actorID int64) {
	defer logFile.Close()
	defer e.deregister(depID)
	defer e.Mutex.Unlock(clusterID)

	publish := func(ev Event) {
		e.Hub.Publish(depID, ev)
	}
	stepStart := func(name string) { publish(Event{Type: EventStepStart, Step: name}) }
	stepEnd := func(name string, exit int) { publish(Event{Type: EventStepEnd, Step: name, Exit: exit}) }

	logWriter := teeWriter{file: logFile, hub: e.Hub, depID: depID}

	fail := func(msg string) {
		// ctx is already cancelled (or done) by the time we reach a fail
		// path, so use a fresh background ctx for the terminal store/audit
		// writes. finishStatus distinguishes "user cancelled mid-run" from
		// "real failure" so the audit row + cluster status reflect intent.
		bg := context.Background()
		status, outcome := finishStatus(ctx.Err(), true)
		_ = e.Store.FinishDeployment(bg, depID, status, msg)
		_ = e.Store.UpdateClusterStatus(bg, clusterID, string(clusters.StatusError))
		_, _ = audit.Write(bg, e.Store, audit.Entry{
			ActorID: actorID,
			Action:  string(audit.ActionClusterDeploy),
			Target:  clusterID,
			Outcome: outcome,
			Details: map[string]any{"error": msg},
		})
		publish(Event{Type: EventDeploymentComplete, Status: status, Text: msg})
	}

	// 1. Terraform
	work := filepath.Join(e.TfStateRoot, "clusters", clusterID)
	tf, err := terraform.New(work, e.TerraformBin)
	if err != nil {
		fail("terraform setup: " + err.Error())
		return
	}
	if err := tf.CopyModule(prof.TerraformModuleDir()); err != nil {
		fail("copy module: " + err.Error())
		return
	}
	tfVars, err := prof.BuildTfvars(ctx, clusterID, e.Vault)
	if err != nil {
		fail("build tfvars: " + err.Error())
		return
	}
	if err := tf.WriteVars(tfVars); err != nil {
		fail("write tfvars: " + err.Error())
		return
	}
	stepStart("terraform.init")
	if err := tf.Init(ctx, &logWriter, &logWriter); err != nil {
		stepEnd("terraform.init", 1)
		fail("tf init: " + err.Error())
		return
	}
	stepEnd("terraform.init", 0)
	stepStart("terraform.apply")
	if err := tf.Apply(ctx, &logWriter, &logWriter); err != nil {
		stepEnd("terraform.apply", 1)
		fail("tf apply: " + err.Error())
		return
	}
	stepEnd("terraform.apply", 0)

	tfOut, err := tf.Output(ctx)
	if err != nil {
		fail("tf output: " + err.Error())
		return
	}
	tfOutMap := map[string]any{}
	for k, v := range tfOut {
		var s string
		if err := json.Unmarshal(v.Value, &s); err == nil {
			tfOutMap[k] = s
		} else {
			// non-string output (number, list, etc.) — keep raw JSON for completeness
			tfOutMap[k] = string(v.Value)
		}
	}

	// 2. Wait for SSH (60s, 2s interval)
	stepStart("wait_for_ssh")
	if err := waitForSSH(ctx, tfOutMap, 60*time.Second, 2*time.Second); err != nil {
		stepEnd("wait_for_ssh", 1)
		fail("wait ssh: " + err.Error())
		return
	}
	stepEnd("wait_for_ssh", 0)

	// 3. Ansible
	stepStart("ansible")
	runDir, err := os.MkdirTemp(work, "ansible-run-")
	if err != nil {
		stepEnd("ansible", 1)
		fail("create ansible run dir: " + err.Error())
		return
	}
	defer os.RemoveAll(runDir)
	inv, err := prof.BuildInventory(ctx, clusterID, tfOutMap, runDir, e.Vault)
	if err != nil {
		stepEnd("ansible", 1)
		fail("inventory: " + err.Error())
		return
	}
	extra, err := prof.BuildExtraVars(ctx, clusterID, e.Vault)
	if err != nil {
		stepEnd("ansible", 1)
		fail("extra vars: " + err.Error())
		return
	}
	runner := ansible.New(e.AnsibleBin, runDir)
	if err := runner.Prepare(prof.AnsiblePlaybookDir(), inv, extra); err != nil {
		stepEnd("ansible", 1)
		fail("ansible prepare: " + err.Error())
		return
	}
	onEvent := func(ev ansible.Event) {
		publish(Event{Type: EventAnsible, Data: ev})
	}
	if err := runner.Run(ctx, prof.AnsiblePlaybookFile(), onEvent, &logWriter, &logWriter); err != nil {
		stepEnd("ansible", 1)
		fail("ansible run: " + err.Error())
		return
	}
	stepEnd("ansible", 0)

	// 4. Pull kubeconfig — best-effort. Failure does NOT flip the cluster to
	// error; k3s itself is up, only the convenience-fetch failed. The UI's
	// Connection card will show "available shortly" + a Retrieve button.
	sshKeyPath := filepath.Join(runDir, "ssh_key")
	masterIP, _ := tfOutMap["master_ip"].(string)
	if err := FetchKubeconfig(ctx, e.Vault, clusterID, sshKeyPath, "ansible", masterIP); err != nil {
		publish(Event{Type: EventLog, Text: "kubeconfig retrieval failed: " + err.Error() + "\n"})
	} else {
		publish(Event{Type: EventLog, Text: "kubeconfig stored at vault://clusters/" + clusterID + "/kubeconfig\n"})
	}

	// Phase 7: pull k3s join token — best-effort. Failure logs but does NOT
	// flip the cluster to error; the UI's Connection card will offer a
	// Retrieve button.
	publish(Event{Type: EventStepStart, Step: "join_token"})
	if err := FetchJoinToken(ctx, e.Vault, clusterID, sshKeyPath, "ansible", masterIP); err != nil {
		publish(Event{Type: EventLog, Text: "join token retrieval failed: " + err.Error() + "\n"})
		publish(Event{Type: EventStepEnd, Step: "join_token", Exit: 1})
	} else {
		publish(Event{Type: EventLog, Text: "join token stored at vault://clusters/" + clusterID + "/join_token\n"})
		publish(Event{Type: EventStepEnd, Step: "join_token", Exit: 0})
	}

	// Phase 4: write wildcard DNS record so the cluster's apps resolve
	// without per-install DNS work.
	publish(Event{Type: EventStepStart, Step: "dns.write_wildcard"})
	if err := writeWildcardDNS(ctx, e.Vault, clusterID, masterIP); err != nil {
		publish(Event{Type: EventStepEnd, Step: "dns.write_wildcard", Exit: 1})
		fail("dns wildcard: " + err.Error())
		return
	}
	publish(Event{Type: EventStepEnd, Step: "dns.write_wildcard", Exit: 0})

	// Phase 4: issue wildcard TLS cert via Vault PKI; write to Vault and
	// push to the cluster as a kube-system Secret for Traefik to use.
	publish(Event{Type: EventStepStart, Step: "tls.issue_wildcard"})
	if err := issueWildcardTLS(ctx, e.Vault, e.AppsHelm, clusterID); err != nil {
		publish(Event{Type: EventStepEnd, Step: "tls.issue_wildcard", Exit: 1})
		fail("tls wildcard: " + err.Error())
		return
	}
	publish(Event{Type: EventStepEnd, Step: "tls.issue_wildcard", Exit: 0})

	// Phase 3: Install Traefik via Helm. Cluster only flips to ready when
	// ingress is healthy. Failure here fails the deploy.
	publish(Event{Type: EventStepStart, Step: "helm.install_traefik"})
	if err := installTraefik(ctx, e.AppsHelm, e.Vault, clusterID, &logWriter, &logWriter); err != nil {
		publish(Event{Type: EventStepEnd, Step: "helm.install_traefik", Exit: 1})
		fail("traefik install: " + err.Error())
		return
	}
	publish(Event{Type: EventStepEnd, Step: "helm.install_traefik", Exit: 0})

	// 5. Done
	_ = e.Store.FinishDeployment(ctx, depID, "succeeded", "")
	_ = e.Store.UpdateClusterStatus(ctx, clusterID, "ready")
	_, _ = audit.Write(ctx, e.Store, audit.Entry{
		ActorID: actorID,
		Action:  string(audit.ActionClusterDeploy),
		Target:  clusterID,
		Outcome: audit.OutcomeSucceeded,
	})
	publish(Event{Type: EventDeploymentComplete, Status: "succeeded"})
}

// Destroy kicks off a terraform destroy for the given cluster. It returns the
// deployment ID immediately and runs the work in a goroutine.
func (e *Executor) Destroy(ctx context.Context, clusterID string) (string, error) {
	cluster, err := e.Store.GetCluster(ctx, clusterID)
	if err != nil {
		return "", err
	}
	if err := clusters.CanTransition(clusters.Status(cluster.Status), clusters.StatusDestroying); err != nil {
		return "", clusters.ErrInvalidTransition
	}
	prof, err := e.Registry.Get(cluster.Profile)
	if err != nil {
		return "", err
	}
	if !e.Mutex.TryLock(clusterID) {
		return "", clusters.ErrLocked
	}

	depID := NewDeploymentID()
	logPath := filepath.Join(e.LogRoot, depID+".log")
	_ = os.MkdirAll(e.LogRoot, 0o755)
	logFile, err := os.Create(logPath)
	if err != nil {
		e.Mutex.Unlock(clusterID)
		return "", err
	}

	actorID := actorIDFromCtx(ctx)
	if err := e.Store.CreateDeployment(ctx, &store.Deployment{
		ID: depID, ClusterID: clusterID, Operation: "destroy", Status: "running",
		ActorID: actorIDPtr(actorID),
	}); err != nil {
		_ = logFile.Close()
		e.Mutex.Unlock(clusterID)
		return "", err
	}
	if err := e.Store.UpdateClusterStatus(ctx, clusterID, string(clusters.StatusDestroying)); err != nil {
		_ = logFile.Close()
		e.Mutex.Unlock(clusterID)
		return "", err
	}
	_, _ = audit.Write(ctx, e.Store, audit.Entry{
		ActorID: actorID,
		Action:  string(audit.ActionClusterDestroy),
		Target:  clusterID,
		Outcome: audit.OutcomeStarted,
	})

	runCtx, cancelRun := context.WithCancel(context.Background())
	e.register(depID, cancelRun)
	go e.runDestroy(runCtx, depID, clusterID, prof, logFile, actorID)
	return depID, nil
}

func (e *Executor) runDestroy(ctx context.Context, depID, clusterID string, prof profiles.Profile, logFile *os.File, actorID int64) {
	defer logFile.Close()
	defer e.deregister(depID)
	defer e.Mutex.Unlock(clusterID)

	logWriter := teeWriter{file: logFile, hub: e.Hub, depID: depID}

	publish := func(ev Event) { e.Hub.Publish(depID, ev) }
	stepStart := func(name string) { publish(Event{Type: EventStepStart, Step: name}) }
	stepEnd := func(name string, exit int) { publish(Event{Type: EventStepEnd, Step: name, Exit: exit}) }

	fail := func(msg string) {
		bg := context.Background()
		status, outcome := finishStatus(ctx.Err(), true)
		_ = e.Store.FinishDeployment(bg, depID, status, msg)
		_ = e.Store.UpdateClusterStatus(bg, clusterID, string(clusters.StatusError))
		_, _ = audit.Write(bg, e.Store, audit.Entry{
			ActorID: actorID,
			Action:  string(audit.ActionClusterDestroy),
			Target:  clusterID,
			Outcome: outcome,
			Details: map[string]any{"error": msg},
		})
		publish(Event{Type: EventDeploymentComplete, Status: status, Text: msg})
	}

	// 1. Pre-destroy hook (no-op for homelab; reserved for future profiles)
	if err := prof.PreDestroy(ctx, clusterID, e.Vault); err != nil {
		fail("pre-destroy: " + err.Error())
		return
	}

	// 2. Terraform destroy
	work := filepath.Join(e.TfStateRoot, "clusters", clusterID)
	tf, err := terraform.New(work, e.TerraformBin)
	if err != nil {
		fail("terraform setup: " + err.Error())
		return
	}
	if err := tf.CopyModule(prof.TerraformModuleDir()); err != nil {
		fail("copy module: " + err.Error())
		return
	}
	tfVars, err := prof.BuildTfvars(ctx, clusterID, e.Vault)
	if err != nil {
		fail("build tfvars: " + err.Error())
		return
	}
	if err := tf.WriteVars(tfVars); err != nil {
		fail("write tfvars: " + err.Error())
		return
	}
	stepStart("terraform.init")
	if err := tf.Init(ctx, &logWriter, &logWriter); err != nil {
		stepEnd("terraform.init", 1)
		fail("tf init: " + err.Error())
		return
	}
	stepEnd("terraform.init", 0)
	stepStart("terraform.destroy")
	if err := tf.Destroy(ctx, &logWriter, &logWriter); err != nil {
		stepEnd("terraform.destroy", 1)
		fail("tf destroy: " + err.Error())
		return
	}
	stepEnd("terraform.destroy", 0)

	// 3. Post-destroy hook — Vault cleanup. Failure is warn-only: infrastructure
	// is gone which is the user-visible meaning of "destroyed". Log it and continue.
	if err := prof.PostDestroy(ctx, clusterID, e.Vault); err != nil {
		fmt.Fprintf(&logWriter, "post-destroy warning: %s\n", err.Error())
	}

	// 4. Done
	_ = e.Store.FinishDeployment(ctx, depID, "succeeded", "")
	_ = e.Store.UpdateClusterStatus(ctx, clusterID, string(clusters.StatusDestroyed))
	_, _ = audit.Write(ctx, e.Store, audit.Entry{
		ActorID: actorID,
		Action:  string(audit.ActionClusterDestroy),
		Target:  clusterID,
		Outcome: audit.OutcomeSucceeded,
	})
	publish(Event{Type: EventDeploymentComplete, Status: "succeeded"})
}

// finishStatus picks the terminal deployment status + audit outcome based on
// whether the goroutine ctx was cancelled vs. hit a natural failure. Used by
// the fail closures in run/runDestroy/runUpgrade to distinguish operator-
// initiated cancel from a real error.
func finishStatus(ctxErr error, naturalFailure bool) (status string, outcome audit.Outcome) {
	if errors.Is(ctxErr, context.Canceled) {
		return "cancelled", audit.OutcomeCancelled
	}
	if naturalFailure {
		return "failed", audit.OutcomeFailed
	}
	return "succeeded", audit.OutcomeSucceeded
}

// actorIDFromCtx reads the authenticated user id from the request context.
// Returns 0 when the context wasn't through the auth middleware (eg. async
// recovery or test paths) — the audit row will have actor_id=NULL.
func actorIDFromCtx(ctx context.Context) int64 {
	if uid, ok := auth.UserIDFromContext(ctx); ok {
		return uid
	}
	return 0
}

// actorIDPtr converts an int64 actor id (0 = unauthenticated) into the *int64
// the deployments row expects. Returning nil for 0 keeps backfilled rows and
// system-driven recoveries from claiming a fictitious user id.
func actorIDPtr(id int64) *int64 {
	if id == 0 {
		return nil
	}
	return &id
}

// installTraefik runs `helm install traefik traefik/traefik` against the
// freshly-deployed cluster. Reads the dashboard toggle from Vault network.
// stdout/stderr are wired to the deploy log so chart-not-found, image-pull
// errors, and RBAC traces surface in the operator's deploy stream rather than
// being swallowed and reduced to a single wrapped error string.
func installTraefik(ctx context.Context, factory apps.HelmFactory, v *vault.Client, clusterID string, stdout, stderr io.Writer) error {
	if factory == nil {
		return fmt.Errorf("apps helm factory not configured")
	}
	helm, done, err := factory.For(ctx, clusterID)
	if err != nil {
		return err
	}
	defer done()
	paths := vault.Paths{}
	netData, _ := v.Get(ctx, paths.Network(clusterID))
	fqdn, _ := netData["fqdn"].(string)
	dashboardOn := true
	if v, ok := netData["traefik_dashboard"].(bool); ok {
		dashboardOn = v
	}
	req := apps.InstallRequest{
		Chart:            "traefik/traefik",
		Version:          "34.2.1",
		ReleaseName:      "traefik",
		Namespace:        "kube-system",
		Atomic:           true,
		IngressValuePath: "ingress.hostname",
		Values: `tlsStore:
  default:
    defaultCertificate:
      secretName: bandolier-wildcard-tls
`,
	}
	if dashboardOn && fqdn != "" {
		req.Hostname = "traefik." + fqdn
	}
	if err := helm.RepoAdd(ctx, "traefik", "https://traefik.github.io/charts"); err != nil {
		return fmt.Errorf("repo add: %w", err)
	}
	if err := helm.RepoUpdate(ctx); err != nil {
		return fmt.Errorf("repo update: %w", err)
	}
	// Phase 4: write Values YAML to a temp file so helm install can pick it
	// up via -f. The InstallRequest carries the rendered YAML so chart
	// overrides (tlsStore default cert) survive the wrapper boundary.
	var valuesPath string
	if req.Values != "" {
		f, err := os.CreateTemp("", "traefik-values-*.yaml")
		if err != nil {
			return fmt.Errorf("temp values: %w", err)
		}
		if _, err := f.WriteString(req.Values); err != nil {
			f.Close()
			os.Remove(f.Name())
			return fmt.Errorf("write values: %w", err)
		}
		f.Close()
		valuesPath = f.Name()
		defer os.Remove(valuesPath)
	}
	return helm.Install(ctx, req, valuesPath, stdout, stderr)
}

// writeWildcardDNS pulls the cluster's DNS authority config + FQDN from Vault,
// constructs the matching dns.Provider, and Upserts a wildcard A record
// (*.fqdn → masterIP). The 5s settle sleep gives recursive resolvers time to
// pick up the new record before Traefik / app installs try to hit it.
func writeWildcardDNS(ctx context.Context, v *vault.Client, clusterID, masterIP string) error {
	dnsData, err := v.Get(ctx, "clusters/"+clusterID+"/dns")
	if err != nil || dnsData == nil {
		// Pre-Phase-4 cluster (no dns secret was ever written) or operator
		// opted out. Treat as kind=none and no-op the step rather than
		// failing the deploy. TLS issuance is a separate step and still runs.
		return nil
	}
	netData, err := v.Get(ctx, vault.Paths{}.Network(clusterID))
	if err != nil || netData == nil {
		return fmt.Errorf("vault network read: %w", err)
	}
	fqdn, _ := netData["fqdn"].(string)
	if fqdn == "" {
		return fmt.Errorf("vault network: missing fqdn")
	}
	cfg := dns.Config{
		Kind:       dns.Kind(asStringDep(dnsData["kind"])),
		Server:     asStringDep(dnsData["server"]),
		Zone:       asStringDep(dnsData["zone"]),
		TSIGName:   asStringDep(dnsData["tsig_name"]),
		TSIGSecret: asStringDep(dnsData["tsig_secret"]),
	}
	provider, err := dns.NewProvider(cfg)
	if err != nil {
		return err
	}
	if err := provider.Upsert(ctx, dns.Record{
		Name: "*." + fqdn, Type: "A", Data: masterIP, TTL: 300,
	}); err != nil {
		return err
	}
	// Skip the resolver-settle wait when the operator manages DNS — there's
	// no record we just wrote that recursive resolvers need to pick up.
	if cfg.Kind != dns.KindNone {
		time.Sleep(5 * time.Second)
	}
	return nil
}

// issueWildcardTLS asks Vault PKI for a wildcard cert for *.fqdn, persists the
// bundle back to Vault for re-push convenience, and pushes it as
// `bandolier-wildcard-tls` in kube-system so Traefik's tlsStore default can
// pick it up.
func issueWildcardTLS(ctx context.Context, v *vault.Client, helmFactory apps.HelmFactory, clusterID string) error {
	netData, err := v.Get(ctx, vault.Paths{}.Network(clusterID))
	if err != nil || netData == nil {
		return fmt.Errorf("vault network read: %w", err)
	}
	fqdn, _ := netData["fqdn"].(string)
	if fqdn == "" {
		return fmt.Errorf("vault network: missing fqdn")
	}
	tlsData, err := v.Get(ctx, "clusters/"+clusterID+"/tls")
	if err != nil || tlsData == nil {
		return fmt.Errorf("vault tls read: %w", err)
	}
	pkiRole, _ := tlsData["pki_role"].(string)
	bundle, err := clusters.IssueWildcardCert(ctx, v, fqdn, pkiRole)
	if err != nil {
		return err
	}
	if err := clusters.PersistWildcardCert(ctx, v, clusterID, bundle); err != nil {
		return err
	}
	helm, done, err := helmFactory.For(ctx, clusterID)
	if err != nil {
		return err
	}
	defer done()
	return clusters.PushCertSecret(ctx, helm.KubeconfigFile(), bundle)
}

// asStringDep is a small "deploy-helper" coercer used by the wildcard DNS/TLS
// steps when reading Vault KV map entries that may legitimately be missing.
// Distinct from the apps/handlers JSON parsing helpers; kept private to this
// package.
func asStringDep(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

type teeWriter struct {
	file  *os.File
	hub   *Hub
	depID string
}

func (t *teeWriter) Write(p []byte) (int, error) {
	t.hub.Publish(t.depID, Event{Type: EventLog, Stream: "stdout", Text: string(p)})
	return t.file.Write(p)
}

// (no-op import; ssh package referenced lazily in Plan 2 with full host key verification)
var _ = io.Copy
