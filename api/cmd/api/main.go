package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lazerdude-labs/bandolier/api/internal/apps"
	"github.com/lazerdude-labs/bandolier/api/internal/audit"
	"github.com/lazerdude-labs/bandolier/api/internal/auth"
	"github.com/lazerdude-labs/bandolier/api/internal/clusters"
	"github.com/lazerdude-labs/bandolier/api/internal/deployments"
	"github.com/lazerdude-labs/bandolier/api/internal/httpserver"
	"github.com/lazerdude-labs/bandolier/api/internal/profiles"
	"github.com/lazerdude-labs/bandolier/api/internal/profiles/blueteam"
	"github.com/lazerdude-labs/bandolier/api/internal/profiles/greyspace"
	"github.com/lazerdude-labs/bandolier/api/internal/profiles/homelab"
	"github.com/lazerdude-labs/bandolier/api/internal/profiles/redteam"
	"github.com/lazerdude-labs/bandolier/api/internal/store"
	"github.com/lazerdude-labs/bandolier/api/internal/telemetry"
	vaultpkg "github.com/lazerdude-labs/bandolier/api/internal/vault"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	dbPath := envWithDefault("BANDOLIER_DB_PATH", "/var/lib/bandolier/app.db")
	tfStateRoot := envWithDefault("BANDOLIER_TF_STATE_ROOT", "/var/lib/bandolier/tf-state")
	logRoot := envWithDefault("BANDOLIER_LOG_ROOT", "/var/lib/bandolier/logs")
	if err := os.MkdirAll(dirOf(dbPath), 0o755); err != nil {
		logger.Error("mkdir db", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, dbPath)
	if err != nil {
		logger.Error("store open", "err", err)
		os.Exit(1)
	}
	defer func() { _ = st.Close() }()

	addr := envWithDefault("BANDOLIER_VAULT_ADDR", "https://vault:8200")
	approlePath := envWithDefault("BANDOLIER_VAULT_APPROLE_PATH", "/vault-init-state/approle.json")
	// Read the mTLS paths directly (not via envWithDefault): an explicitly
	// empty BANDOLIER_VAULT_CACERT must reach the plaintext-dev path in
	// LoginAppRole rather than silently falling back to the cert default. The
	// compose stack sets all three; a local dev run against a non-TLS Vault
	// leaves them empty.
	vaultTLS := vaultpkg.TLSConfig{
		CACert:     os.Getenv("BANDOLIER_VAULT_CACERT"),
		ClientCert: os.Getenv("BANDOLIER_VAULT_CLIENT_CERT"),
		ClientKey:  os.Getenv("BANDOLIER_VAULT_CLIENT_KEY"),
	}

	vaultCli, loginSecret, approleCreds, err := vaultpkg.LoginAppRole(ctx, addr, approlePath, vaultTLS)
	if err != nil {
		logger.Error("vault login", "err", err)
		os.Exit(1)
	}
	vaultClient := vaultpkg.NewClient(vaultCli, vaultpkg.KVMount)

	// Background lifetime watcher — keeps the AppRole token renewed and
	// falls back to a fresh login if renewal fails. Each renew/relogin
	// emits a vault_token_renew audit row.
	tokenAuditWriter := func(ctx context.Context, action string, details map[string]any) {
		_, _ = audit.Write(ctx, st, audit.Entry{
			Action:  action,
			Outcome: audit.OutcomeSuccess,
			Details: details,
		})
	}
	if err := vaultClient.StartLifetimeWatcher(ctx, logger, loginSecret, approleCreds, tokenAuditWriter); err != nil {
		logger.Error("vault watcher start", "err", err)
		os.Exit(1)
	}

	wsKey, err := auth.EnsureWSSigningKey(ctx, vaultClient)
	if err != nil {
		logger.Error("ws signing key", "err", err)
		os.Exit(1)
	}

	hub := deployments.NewHub()
	mu := deployments.NewClusterMutex()
	registry := profiles.NewRegistry()
	registry.Register(homelab.New(
		os.Getenv("BANDOLIER_TERRAFORM_DIR"),
		os.Getenv("BANDOLIER_ANSIBLE_DIR"),
	))
	registry.Register(redteam.New(
		os.Getenv("BANDOLIER_TERRAFORM_DIR"),
		os.Getenv("BANDOLIER_ANSIBLE_DIR"),
	))
	registry.Register(blueteam.New(
		os.Getenv("BANDOLIER_TERRAFORM_DIR"),
		os.Getenv("BANDOLIER_ANSIBLE_DIR"),
	))
	registry.Register(greyspace.New(
		os.Getenv("BANDOLIER_TERRAFORM_DIR"),
		os.Getenv("BANDOLIER_ANSIBLE_DIR"),
	))
	exec := &deployments.Executor{
		Store:        st,
		Vault:        vaultClient,
		Hub:          hub,
		Mutex:        mu,
		Registry:     registry,
		TfStateRoot:  tfStateRoot,
		LogRoot:      logRoot,
		TerraformBin: "terraform",
		AnsibleBin:   "ansible-runner",
	}

	// Mark any deployments left in `running` state from a previous process
	// as failed, and flip their clusters to error. This unsticks clusters
	// after a container restart killed the in-flight executor goroutine.
	if err := exec.RecoverOrphanedDeployments(ctx); err != nil {
		logger.Warn("orphan recovery failed", "err", err)
	}

	agg := telemetry.NewAggregator(vaultClient)

	// Apps wiring — Phase 3D: helm install/upgrade/uninstall executor + handlers.
	// The Executor publishes to the same deployments.Hub via a thin adapter so
	// /ws/apps/installs/{id}/logs and /ws/deployments/{id}/logs share a backend.
	appsStore := apps.NewStore(st)

	// Reconcile factory Helm repos onto existing clusters. When a release
	// adds a new factory repo (v0.1.12 added longhorn + wikijs), clusters
	// created before that release miss the repo at create time and any
	// bundle install referencing it fails with "repo not found". The
	// reconciler is purely additive — see clusters/factory_repo_reconcile.go
	// for the regression-vs-correctness tradeoff. Failure is non-fatal:
	// existing clusters keep their current repo set, the boot continues.
	if err := clusters.ReconcileFactoryRepos(ctx, st, appsStore); err != nil {
		logger.Warn("factory repo reconcile failed", "err", err)
	}

	appsCatalog := apps.NewCatalog(appsStore)
	helmFactory := apps.NewVaultHelmFactory(vaultClient, "helm")
	hubAdapter := &hubEventAdapter{h: hub}
	appsExec := &apps.Executor{
		Apps:    appsStore,
		Core:    st,
		Catalog: appsCatalog,
		Hub:     hubAdapter,
		Mutex:   mu,
		Helm:    helmFactory,
		Vault:   vaultClient,
		LogRoot: logRoot,
	}
	appsHandler := apps.NewHandler(appsStore, appsCatalog, helmFactory)
	appsRepos := apps.NewRepoHandler(appsStore, st, appsCatalog, helmFactory)
	appsExecHandler := apps.NewExecHandler(appsStore, appsExec, appsHandler.ReleasesCache())

	// Wire AppsHelm on the deploy executor so the post-bootstrap Traefik install
	// runs inside the deploy goroutine.
	exec.AppsHelm = helmFactory

	handler := httpserver.New(httpserver.Deps{
		Logger:       logger,
		Store:        st,
		Vault:        vaultClient,
		Executor:     exec,
		Hub:          hub,
		Registry:     registry,
		Telemetry:    agg,
		AppsHandler:  appsHandler,
		AppsRepos:    appsRepos,
		AppsExec:     appsExecHandler,
		WSSigningKey: wsKey,
	})

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Wildcard cert renewal background loop. Runs every hour, re-issues any
	// `ready` cluster cert within 7 days of expiry. System-initiated; ActorID
	// stays 0 in audit rows.
	auditWriter := func(ctx context.Context, action, target string, details map[string]any) {
		_, _ = audit.Write(ctx, st, audit.Entry{
			Action:  action,
			Target:  target,
			Outcome: audit.OutcomeSuccess,
			Details: details,
		})
	}
	go clusters.RenewLoop(context.Background(), st, vaultClient, renewalHelmAdapter{inner: helmFactory}, auditWriter)

	go func() {
		logger.Info("api listening",
			"addr", srv.Addr,
			"db", dbPath,
			"vault", addr,
			"tf_state_root", tfStateRoot,
			"log_root", logRoot,
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("listen failed", "err", err)
			os.Exit(1)
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

// hubEventAdapter bridges apps.EventPublisher to the shared deployments.Hub.
// apps cannot import deployments (cycle: apps ← clusters ← deployments), so
// the narrow EventPublisher interface lives in apps and the adapter is wired
// here in main where both packages are visible.
type hubEventAdapter struct{ h *deployments.Hub }

func (a *hubEventAdapter) PublishLog(id, text string) {
	a.h.Publish(id, deployments.Event{Type: deployments.EventLog, Text: text})
}

func (a *hubEventAdapter) PublishStepStart(id, step string) {
	a.h.Publish(id, deployments.Event{Type: deployments.EventStepStart, Step: step})
}

func (a *hubEventAdapter) PublishStepEnd(id, step string, exit int) {
	a.h.Publish(id, deployments.Event{Type: deployments.EventStepEnd, Step: step, Exit: exit})
}

func (a *hubEventAdapter) PublishStepProgress(id, step string, data any) {
	a.h.Publish(id, deployments.Event{Type: deployments.EventStepProgress, Step: step, Data: data})
}

func (a *hubEventAdapter) PublishComplete(id, status, text string) {
	a.h.Publish(id, deployments.Event{Type: deployments.EventDeploymentComplete, Status: status, Text: text})
}

// renewalHelmAdapter bridges apps.HelmFactory (returns apps.Helm) to
// clusters.HelmFactoryLike (returns clusters.HelmKubeconfigOnly). apps.Helm
// satisfies HelmKubeconfigOnly structurally via KubeconfigFile(), but Go's
// type system doesn't auto-convert interface return types — hence this shim.
type renewalHelmAdapter struct{ inner apps.HelmFactory }

func (a renewalHelmAdapter) For(ctx context.Context, clusterID string) (clusters.HelmKubeconfigOnly, func(), error) {
	h, done, err := a.inner.For(ctx, clusterID)
	if err != nil {
		return nil, func() {}, err
	}
	return h, done, nil
}

func dirOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return "."
}

// envWithDefault returns os.Getenv(name) when set and non-empty, otherwise
// def. Single-source pattern for the BANDOLIER_* runtime overrides so a new
// override never gets added without a default fallback (and the next reader
// finds them all the same way).
func envWithDefault(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
