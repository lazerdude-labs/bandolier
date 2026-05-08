package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/lazerdude-labs/bandolier/api/internal/apps"
	"github.com/lazerdude-labs/bandolier/api/internal/audit"
	"github.com/lazerdude-labs/bandolier/api/internal/auth"
	"github.com/lazerdude-labs/bandolier/api/internal/clusters"
	"github.com/lazerdude-labs/bandolier/api/internal/deployments"
	"github.com/lazerdude-labs/bandolier/api/internal/profiles"
	"github.com/lazerdude-labs/bandolier/api/internal/profiles/homelab"
	"github.com/lazerdude-labs/bandolier/api/internal/store"
	"github.com/lazerdude-labs/bandolier/api/internal/vault"
)

type Deps struct {
	Logger       *slog.Logger
	Store        *store.Store
	Vault        *vault.Client
	Executor     *deployments.Executor
	Hub          *deployments.Hub
	Registry     *profiles.Registry
	Telemetry    clusters.NodeAggregator
	AppsHandler  *apps.Handler
	AppsRepos    *apps.RepoHandler
	AppsExec     *apps.ExecHandler
	WSSigningKey []byte
}

func New(deps Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(RequestIDMiddleware)
	r.Use(LoggingMiddleware(deps.Logger))
	r.Use(RecoveryMiddleware(deps.Logger))

	authHandler := auth.NewHandler(deps.Store)
	dh := deployments.NewHandler(deps.Store, deps.Executor, deps.Hub)

	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		info, err := deps.Vault.Health(r.Context())
		if err != nil {
			deps.Logger.Warn("health: vault status query failed", "err", err)
			// info is non-nil with sealed=true on err; safe to use.
		}
		status := deps.Vault.TokenStatus(r.Context())
		var lastRenewed string
		if !status.LastRenewed.IsZero() {
			lastRenewed = status.LastRenewed.Format(time.RFC3339)
		}
		vaultBlock := map[string]any{
			"sealed":             info.Sealed,
			"initialized":        info.Initialized,
			"version":            info.Version,
			"cluster_name":       info.ClusterName,
			"type":               info.Type,
			"auth_method":        info.AuthMethod,
			"token_ttl_seconds":  status.TTLSeconds,
			"token_last_renewed": lastRenewed,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"vault":  vaultBlock,
		})
	})

	r.Get("/api/auth/status", authHandler.Status)
	r.Post("/api/auth/setup", authHandler.Setup)
	r.Post("/api/auth/login", authHandler.Login)
	r.Post("/api/auth/logout", authHandler.Logout)

	r.Group(func(pr chi.Router) {
		pr.Use(auth.RequireSession(deps.Store))
		pr.Get("/api/auth/whoami", func(w http.ResponseWriter, r *http.Request) {
			uid, _ := auth.UserIDFromContext(r.Context())
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"user_id":` + itoa(uid) + `}`))
		})
		pr.Post("/api/auth/change-password", authHandler.ChangePassword)
		pr.Method(http.MethodPost, "/api/auth/ws-token", auth.NewWSTokenHandler(deps.WSSigningKey))

		pr.Get("/api/profiles", profiles.NewListHandler(deps.Registry).ServeHTTP)
		pr.Get("/api/distros", homelab.NewDistrosHandler().ServeHTTP)

		clusterH := clusters.NewHandler(deps.Store, deps.Registry, deps.Vault)
		pr.Post("/api/clusters", clusterH.Create)
		pr.Get("/api/clusters", clusterH.List)
		pr.Get("/api/clusters/{id}", clusterH.Get)

		pr.Post("/api/clusters/{id}/destroy", clusters.NewDestroyHandler(deps.Store, deps.Executor).ServeHTTP)
		pr.Post("/api/clusters/{id}/upgrade", clusters.NewUpgradeHandler(deps.Store, deps.Executor).ServeHTTP)
		pr.Get("/api/clusters/{id}/nodes", clusters.NewNodesHandler(deps.Store, deps.Telemetry).ServeHTTP)
		pr.Get("/api/clusters/{id}/kubeconfig", clusters.NewKubeconfigHandler(deps.Store, deps.Vault).ServeHTTP)
		pr.Post("/api/clusters/{id}/kubeconfig/retrieve",
			clusters.NewKubeconfigRetrieveHandler(deps.Store, deps.Executor).ServeHTTP)
		pr.Get("/api/clusters/{id}/join-token",
			clusters.NewJoinTokenHandler(deps.Store, deps.Vault).ServeHTTP)
		pr.Post("/api/clusters/{id}/join-token/retrieve",
			clusters.NewJoinTokenRetrieveHandler(deps.Store, deps.Executor).ServeHTTP)

		// Bandolier Applications (Helm). Read endpoints + repo CRUD; the
		// install/upgrade/uninstall executor wires up in Phase 3C.
		pr.Get("/api/clusters/{id}/apps/catalog", deps.AppsHandler.Catalog)
		pr.Get("/api/clusters/{id}/apps/releases", deps.AppsHandler.Releases)
		pr.Get("/api/clusters/{id}/apps/installs", deps.AppsHandler.Installs)
		pr.Get("/api/apps/installs/{id}", deps.AppsHandler.GetInstall)

		pr.Get("/api/clusters/{id}/apps/repos", deps.AppsRepos.List)
		pr.Post("/api/clusters/{id}/apps/repos", deps.AppsRepos.Add)
		pr.Delete("/api/clusters/{id}/apps/repos/{name}", deps.AppsRepos.Remove)

		pr.Post("/api/clusters/{id}/apps/install", deps.AppsExec.Install)
		pr.Post("/api/clusters/{id}/apps/bundle", deps.AppsExec.InstallBundleHandler)
		pr.Post("/api/clusters/{id}/apps/{release}/upgrade", deps.AppsExec.Upgrade)
		pr.Post("/api/clusters/{id}/apps/{release}/uninstall", deps.AppsExec.Uninstall)

		init := clusters.NewInitializer(deps.Store, deps.Vault)
		pr.Post("/api/clusters/{id}/initialize", init.Handle)
		pr.Post("/api/clusters/{id}/dns/test",
			clusters.NewDNSTestHandler(deps.Store, deps.Vault).ServeHTTP)

		pr.Post("/api/clusters/{id}/deploy", dh.Deploy)
		pr.Get("/api/clusters/{id}/deployments", dh.ListForCluster)
		pr.Get("/api/deployments/{id}", dh.GetDeployment)
		pr.Post("/api/deployments/{id}/cancel",
			deployments.NewCancelHandler(deps.Store, deps.Executor).ServeHTTP)

		pr.Get("/api/audit-log", audit.NewListHandler(deps.Store).ServeHTTP)
	})

	// WebSocket routes — authenticated via signed Sec-WebSocket-Protocol token
	// (cookies are incompatible with the WS upgrade in some browsers). Apps
	// install/upgrade/uninstall logs share the deploy Hub — same handler,
	// keyed by stream id (== install id).
	r.Group(func(wsr chi.Router) {
		wsr.Use(auth.WebSocketSession(deps.WSSigningKey))
		wsr.Get("/ws/deployments/{id}/logs", dh.Logs)
		wsr.Get("/ws/apps/installs/{id}/logs", dh.Logs)
	})

	return r
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
