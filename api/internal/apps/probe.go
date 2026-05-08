package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
)

// hostMatcherRe captures the hostname out of a Traefik IngressRoute match
// expression. Both Host("foo") and Host(`foo`) variants are supported — the
// alternation in the character classes covers both.
var hostMatcherRe = regexp.MustCompile("Host\\([\"`]([^\"`]+)[\"`]\\)")

// probeRawList mirrors the trimmed-down shape of `kubectl get
// ingress,ingressroutes -o json`. We only care about spec.rules[].host
// (k8s Ingress) and spec.routes[].match (Traefik IngressRoute) — the rest
// is noise. Prefixed with probe to avoid collision with the helm.go rawList
// (used for `helm list` parsing).
type probeRawList struct {
	Items []probeRawItem `json:"items"`
}

type probeRawItem struct {
	Kind string       `json:"kind"`
	Spec probeRawSpec `json:"spec"`
}

type probeRawSpec struct {
	Rules  []probeRawRule  `json:"rules,omitempty"`
	Routes []probeRawRoute `json:"routes,omitempty"`
}

type probeRawRule struct {
	Host string `json:"host"`
}

type probeRawRoute struct {
	Match string `json:"match"`
}

// probeHostnameClaimed shells out to kubectl against the freshly-fetched
// kubeconfig for the cluster and returns whether the given hostname appears
// in either an Ingress.spec.rules[].host or a Traefik IngressRoute
// spec.routes[].match Host(...) expression in the namespace.
//
// Best-effort by design: the caller treats a non-nil error as "couldn't tell"
// rather than as failure. --ignore-not-found means a cluster with no
// IngressRoute CRD installed (Traefik not yet deployed) doesn't error.
func probeHostnameClaimed(ctx context.Context, kubeconfigPath, namespace, hostname string) (bool, error) {
	if kubeconfigPath == "" {
		return false, fmt.Errorf("probeHostnameClaimed: empty kubeconfig path")
	}
	cmd := exec.CommandContext(ctx, "kubectl",
		"--kubeconfig", kubeconfigPath,
		"-n", namespace,
		"get", "ingress,ingressroutes",
		"-o", "json",
		"--ignore-not-found",
	)
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("kubectl get: %w", err)
	}
	if len(out) == 0 {
		// --ignore-not-found with no resources prints empty output.
		return false, nil
	}
	hosts := parseClaimedHosts(out)
	return hostnameClaimed(hostname, hosts), nil
}

// parseClaimedHosts extracts every claimed hostname from a kubectl JSON list.
// Tolerant of empty/missing fields — anything that doesn't fit either shape is
// silently skipped so the probe stays best-effort.
func parseClaimedHosts(b []byte) []string {
	var list probeRawList
	if err := json.Unmarshal(b, &list); err != nil {
		return nil
	}
	var out []string
	for _, item := range list.Items {
		for _, r := range item.Spec.Rules {
			if r.Host != "" {
				out = append(out, r.Host)
			}
		}
		for _, route := range item.Spec.Routes {
			out = append(out, hostMatches(route.Match)...)
		}
	}
	return out
}

// hostMatches pulls every Host("...") / Host(`...`) hostname out of a Traefik
// match expression. Returns nil on no match.
func hostMatches(match string) []string {
	if match == "" {
		return nil
	}
	all := hostMatcherRe.FindAllStringSubmatch(match, -1)
	if len(all) == 0 {
		return nil
	}
	out := make([]string, 0, len(all))
	for _, m := range all {
		if len(m) >= 2 && m[1] != "" {
			out = append(out, m[1])
		}
	}
	return out
}

// hostnameClaimed reports whether want appears (case-sensitive exact match) in
// the slice of hosts. Empty hosts list returns false.
func hostnameClaimed(want string, hosts []string) bool {
	for _, h := range hosts {
		if h == want {
			return true
		}
	}
	return false
}
