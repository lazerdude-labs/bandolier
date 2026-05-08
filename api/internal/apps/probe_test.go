package apps

import (
	"sort"
	"testing"
)

func TestParseClaimedHostsFromIngress(t *testing.T) {
	// Standard k8s networking.k8s.io/v1 Ingress with two rules.
	body := []byte(`{
	  "items": [
	    {
	      "kind": "Ingress",
	      "spec": {
	        "rules": [
	          {"host": "app.example.com"},
	          {"host": "alt.example.com"}
	        ]
	      }
	    }
	  ]
	}`)

	got := parseClaimedHosts(body)
	sort.Strings(got)
	want := []string{"alt.example.com", "app.example.com"}
	if !equalStrings(got, want) {
		t.Fatalf("parseClaimedHosts: got %v, want %v", got, want)
	}
}

func TestParseClaimedHostsFromIngressRoute(t *testing.T) {
	// Traefik IngressRoute uses match strings like Host("foo") within the
	// route definition. Quotes variant.
	body := []byte(`{
	  "items": [
	    {
	      "kind": "IngressRoute",
	      "spec": {
	        "routes": [
	          {"match": "Host(\"app.example.com\") && PathPrefix(\"/\")"},
	          {"match": "Host(\"alt.example.com\")"}
	        ]
	      }
	    }
	  ]
	}`)

	got := parseClaimedHosts(body)
	sort.Strings(got)
	want := []string{"alt.example.com", "app.example.com"}
	if !equalStrings(got, want) {
		t.Fatalf("parseClaimedHosts: got %v, want %v", got, want)
	}
}

func TestParseClaimedHostsBackticks(t *testing.T) {
	// Backtick variant — also supported in Traefik match expressions.
	body := []byte("{\n" +
		"  \"items\": [\n" +
		"    {\n" +
		"      \"kind\": \"IngressRoute\",\n" +
		"      \"spec\": {\n" +
		"        \"routes\": [\n" +
		"          {\"match\": \"Host(`app.example.com`)\"},\n" +
		"          {\"match\": \"Host(`alt.example.com`) && PathPrefix(`/`)\"}\n" +
		"        ]\n" +
		"      }\n" +
		"    }\n" +
		"  ]\n" +
		"}")

	got := parseClaimedHosts(body)
	sort.Strings(got)
	want := []string{"alt.example.com", "app.example.com"}
	if !equalStrings(got, want) {
		t.Fatalf("parseClaimedHosts: got %v, want %v", got, want)
	}
}

func TestHostnameClaimed(t *testing.T) {
	hosts := []string{"foo.example.com", "bar.example.com"}
	if !hostnameClaimed("foo.example.com", hosts) {
		t.Fatalf("hostnameClaimed: expected match for foo.example.com")
	}
	if hostnameClaimed("baz.example.com", hosts) {
		t.Fatalf("hostnameClaimed: unexpected match for baz.example.com")
	}
	if hostnameClaimed("foo.example.com", nil) {
		t.Fatalf("hostnameClaimed: expected no match against empty host list")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
