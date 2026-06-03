package apps

import "github.com/lazerdude-labs/bandolier/api/internal/validate"

// validHostname reports whether s is a syntactically valid DNS hostname/FQDN,
// guarding operator-supplied ingress hostnames before they reach a
// `helm --set <path>=<hostname>` arg (helm splits --set on commas, so an
// unvalidated value could smuggle a second value key). It delegates to the
// shared validate package so the same rule is enforced at the cluster-init FQDN
// boundary (clusters.initialize) and the apps install/upgrade/bundle boundaries.
// The empty string is NOT valid; callers treat "" (or a nil pointer) as "unset"
// and must skip the check for it.
func validHostname(s string) bool {
	return validate.Hostname(s)
}
