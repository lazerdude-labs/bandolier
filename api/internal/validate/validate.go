// Package validate holds small, dependency-free input validators shared across
// the api. It is a leaf package (imports only the standard library) so any
// package can use it without creating an import cycle.
package validate

import "regexp"

// hostnameRe matches an RFC 1123 DNS hostname (one or more dot-separated
// labels). It guards operator-supplied hostnames/FQDNs before they become a
// `helm --set <path>=<hostname>` arg: helm splits --set on unescaped commas, so
// an unvalidated value like "app.example.com,admin.password=evil" would smuggle
// a second value key into the release even though argv form already blocks
// shell injection. The allowlist rejects the comma/equals/space/meta characters
// injection would require. Kubernetes ingress hostnames must be lowercase, so
// the pattern is lowercase-only by design.
var hostnameRe = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)

// Hostname reports whether s is a syntactically valid DNS hostname/FQDN. The
// empty string is NOT valid; callers that treat "" as "unset" must skip the
// check for it.
func Hostname(s string) bool {
	return len(s) <= 253 && hostnameRe.MatchString(s)
}
