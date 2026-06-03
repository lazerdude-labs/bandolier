package apps

import "regexp"

// hostnameRe matches an RFC 1123 DNS hostname (one or more dot-separated
// labels). It guards operator-supplied ingress hostnames before they become a
// `helm --set <path>=<hostname>` arg: helm splits --set on unescaped commas, so
// an unvalidated value like "app.example.com,admin.password=evil" would smuggle
// a second value key into the release even though argv form already blocks
// shell injection. This is the same threat model validStorageClassName guards
// for global.storageClass (see storageclass.go); the allowlist rejects the
// comma/equals/space/meta characters that injection would require.
var hostnameRe = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)

// validHostname reports whether s is a syntactically valid DNS hostname/FQDN.
// The empty string is NOT valid here; callers treat "" (or a nil pointer) as
// "unset" (no ingress --set) and must skip validation for it.
func validHostname(s string) bool {
	return len(s) <= 253 && hostnameRe.MatchString(s)
}
