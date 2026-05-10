package homelab

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// URLProber abstracts the minimal HTTP interface PickReachableURL needs so
// tests can inject deterministic responses without a network. *http.Client
// satisfies it. We only call Do.
type URLProber interface {
	Do(req *http.Request) (*http.Response, error)
}

// DefaultProber is used when no URLProber is supplied. The 5s per-request
// timeout caps the worst case at len(urls) * 5s — for a 3-mirror catalog
// entry, ~15s total budget before all mirrors are declared unreachable.
var DefaultProber URLProber = &http.Client{Timeout: 5 * time.Second}

// PickReachableURL returns the first URL in the candidate list that responds
// 2xx to an HTTP HEAD request, plus the list of per-URL errors encountered
// for any earlier candidates (useful for logging / audit details). On
// all-fail it returns ("", non-nil error wrapping every failure).
//
// Why HEAD: avoids transferring the multi-GB image just to test reachability.
// Why first-2xx wins (not lowest latency): keeps preference order — operators
// can rank mirrors deterministically. A misbehaving primary just falls through.
//
// Caveat (issue #11): the api container's egress != Proxmox's egress in all
// topologies. In the typical homelab setup they share NAT, so a HEAD probe
// from the api container is a reasonable predictor of what Proxmox will see.
// In split-network deployments, fall back to the custom-URL escape hatch.
func PickReachableURL(ctx context.Context, urls []string, prober URLProber) (string, []error, error) {
	if len(urls) == 0 {
		return "", nil, errors.New("no candidate URLs supplied")
	}
	if prober == nil {
		prober = DefaultProber
	}
	var attempts []error
	for _, u := range urls {
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, u, nil)
		if err != nil {
			attempts = append(attempts, fmt.Errorf("%s: build request: %w", u, err))
			continue
		}
		// Some mirrors (incl. CDNs that gate by UA) reject the default Go
		// User-Agent or empty UA. Identify ourselves so a 403 here is
		// likelier to reflect a real reachability problem rather than a UA
		// filter — and so operators can find Bandolier in mirror logs.
		req.Header.Set("User-Agent", "Bandolier/1 (+https://github.com/lazerdude-labs/bandolier)")
		resp, err := prober.Do(req)
		if err != nil {
			attempts = append(attempts, fmt.Errorf("%s: %w", u, err))
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return u, attempts, nil
		}
		attempts = append(attempts, fmt.Errorf("%s: HTTP %d", u, resp.StatusCode))
	}
	return "", attempts, fmt.Errorf("no reachable mirror among %d candidate(s): %s",
		len(urls), strings.Join(errStrings(attempts), "; "))
}

func errStrings(errs []error) []string {
	out := make([]string, len(errs))
	for i, e := range errs {
		out[i] = e.Error()
	}
	return out
}
