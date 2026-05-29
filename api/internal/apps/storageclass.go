package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"regexp"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

// storageClassNameRe matches a Kubernetes StorageClass name (an RFC 1123
// subdomain). validStorageClassName guards the operator-supplied value before
// it becomes a `helm --set global.storageClass=<v>` arg: helm splits --set on
// commas, so an unvalidated value like "longhorn,foo.admin.password=x" would
// smuggle a second value key into the release even though argv form already
// blocks shell injection. The allowlist rejects the comma/equals/space/meta
// characters that injection would require.
var storageClassNameRe = regexp.MustCompile(`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`)

// validStorageClassName reports whether s is a syntactically valid StorageClass
// name. The empty string is NOT valid here; callers treat "" as "unset" (no
// --set flag) and must skip validation for it.
func validStorageClassName(s string) bool {
	return len(s) <= 253 && storageClassNameRe.MatchString(s)
}

// StorageClass is one row in the per-cluster storage-class list. Drives the
// per-chart StorageClass picker in the bundle install modal.
type StorageClass struct {
	Name        string `json:"name"`
	Provisioner string `json:"provisioner"`
	IsDefault   bool   `json:"is_default"`
}

// StorageClassResponse is the GET /apps/storage-classes body.
type StorageClassResponse struct {
	StorageClasses []StorageClass `json:"storage_classes"`
}

// scRawList mirrors the JSON shape `kubectl get storageclass -o json` emits.
type scRawList struct {
	Items []struct {
		Metadata struct {
			Name        string            `json:"name"`
			Annotations map[string]string `json:"annotations"`
		} `json:"metadata"`
		Provisioner string `json:"provisioner"`
	} `json:"items"`
}

const defaultClassAnnotation = "storageclass.kubernetes.io/is-default-class"

func parseStorageClassJSON(b []byte) ([]StorageClass, error) {
	if len(b) == 0 {
		return []StorageClass{}, nil
	}
	var raw scRawList
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("parse storageclass list: %w", err)
	}
	out := make([]StorageClass, 0, len(raw.Items))
	for _, it := range raw.Items {
		out = append(out, StorageClass{
			Name:        it.Metadata.Name,
			Provisioner: it.Provisioner,
			IsDefault:   it.Metadata.Annotations[defaultClassAnnotation] == "true",
		})
	}
	return out, nil
}

// listStorageClasses shells out to kubectl against the cluster's temp
// kubeconfig (same pattern as probe.go — this codebase has no client-go).
func listStorageClasses(ctx context.Context, kubeconfigPath string) ([]StorageClass, error) {
	if kubeconfigPath == "" {
		return nil, fmt.Errorf("listStorageClasses: empty kubeconfig path")
	}
	cmd := exec.CommandContext(ctx, "kubectl",
		"--kubeconfig", kubeconfigPath,
		"get", "storageclass",
		"-o", "json",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("kubectl get storageclass: %w", err)
	}
	return parseStorageClassJSON(out)
}

// storageClassCache caches []StorageClass per cluster id. kubectl is a remote
// call so cache to keep the modal snappy; 30s matches releasesCacheTTL.
type storageClassCache struct {
	ttl  time.Duration
	mu   sync.Mutex
	data map[string]storageClassCacheEntry
}

type storageClassCacheEntry struct {
	at      time.Time
	classes []StorageClass
}

func newStorageClassCache(ttl time.Duration) *storageClassCache {
	return &storageClassCache{ttl: ttl, data: map[string]storageClassCacheEntry{}}
}

func (c *storageClassCache) get(key string) ([]StorageClass, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.data[key]
	if !ok {
		return nil, false
	}
	if time.Since(e.at) > c.ttl {
		delete(c.data, key)
		return nil, false
	}
	out := make([]StorageClass, len(e.classes))
	copy(out, e.classes)
	return out, true
}

func (c *storageClassCache) put(key string, classes []StorageClass) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([]StorageClass, len(classes))
	copy(cp, classes)
	c.data[key] = storageClassCacheEntry{at: time.Now(), classes: cp}
}

// StorageClasses returns the StorageClasses available on a cluster, cached 30s.
// Fail-open on a not-ready cluster: returns an empty list (HTTP 200) so the
// modal renders "no storage classes found" rather than erroring. A class is
// required to install storage-bearing charts, so the modal gates on emptiness.
//
// GET /api/clusters/{id}/apps/storage-classes
func (h *Handler) StorageClasses(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "id")

	if cached, ok := h.storageClasses.get(clusterID); ok {
		writeJSON(w, http.StatusOK, StorageClassResponse{StorageClasses: cached})
		return
	}

	helm, cleanup, err := h.hf.For(r.Context(), clusterID)
	if err != nil {
		// Fail open: a not-ready cluster has no kubeconfig yet. The picker is a
		// convenience — installs still work against the cluster default — so
		// degrade to an empty list rather than a 500. Log so a perpetually
		// empty picker is diagnosable server-side.
		slog.Warn("storage-classes: kubeconfig unavailable, returning empty list",
			"cluster_id", clusterID, "err", err)
		writeJSON(w, http.StatusOK, StorageClassResponse{StorageClasses: []StorageClass{}})
		return
	}
	defer cleanup()

	classes, err := listStorageClasses(r.Context(), helm.KubeconfigFile())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errMap(err))
		return
	}
	if classes == nil {
		classes = []StorageClass{}
	}
	h.storageClasses.put(clusterID, classes)
	writeJSON(w, http.StatusOK, StorageClassResponse{StorageClasses: classes})
}
