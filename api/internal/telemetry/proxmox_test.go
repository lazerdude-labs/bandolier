package telemetry

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProxmoxProbeGetVMs(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/api2/json/cluster/resources") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "PVEAPIToken=") {
			t.Errorf("missing PVEAPIToken: %s", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[
			{"node":"pve-01","vmid":211,"name":"master1","type":"qemu","status":"running"},
			{"node":"pve-01","vmid":212,"name":"worker1","type":"qemu","status":"running"},
			{"node":"pve-01","vmid":42,"name":"unrelated","type":"lxc","status":"stopped"}
		]}`))
	}))
	defer srv.Close()

	p := ProxmoxProbe{Client: srv.Client()}
	vms, err := p.GetVMs(context.Background(), ProxmoxCreds{
		Endpoint:    srv.URL,
		TokenID:     "user@pve!ops",
		TokenSecret: "secret-uuid",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Only qemu VMs returned (lxc filtered out)
	if len(vms) != 2 {
		t.Fatalf("want 2 qemu VMs, got %d: %+v", len(vms), vms)
	}
	if vms[0].Name != "master1" || vms[0].VMID != 211 {
		t.Fatalf("vm[0] mismatch: %+v", vms[0])
	}
}

func TestProxmoxClientUsesCABundleWhenSet(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	certPEM := srv.Certificate()
	pemStr := pemEncode(certPEM)

	p := ProxmoxProbe{} // intentional: no Client override; force GetVMs to build one
	_, err := p.GetVMs(context.Background(), ProxmoxCreds{
		Endpoint: srv.URL, TokenID: "u@p!t", TokenSecret: "x",
		CABundle: pemStr,
	})
	if err != nil {
		t.Fatalf("strict TLS with valid CA failed: %v", err)
	}
}

func TestProxmoxRejectsMalformedCABundle(t *testing.T) {
	p := ProxmoxProbe{}
	_, err := p.GetVMs(context.Background(), ProxmoxCreds{
		Endpoint: "https://anywhere", TokenID: "u@p!t", TokenSecret: "x",
		CABundle: "not a pem cert",
	})
	if err == nil {
		t.Fatal("expected error on malformed PEM, got nil")
	}
}

func pemEncode(cert *x509.Certificate) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))
}

func TestProxmoxRejectsNonCertPEMBlock(t *testing.T) {
	// Valid PEM frame, but type is RSA PRIVATE KEY (not CERTIFICATE).
	bundle := "-----BEGIN RSA PRIVATE KEY-----\nMIIBOgIBAAJBAKj34GkxFhD90vcNLYLInFEX6Ppy1tPf9Cnzj4p4WGeKLs1Pt8Qu\nKUpRKfFLfRYC9AIKjbJTWit+CqvjWYzvQwECAwEAAQJAIJLixBy2qpFoS4DSmoEm\no3qGy0t6z09AIJtH+5OeRV1be+N4cDYJKffGzDa88vQENZiRm0GRq6a+HPGQMd2k\nTQIhAKMSvzIBnni7ot/OSie2TmJLY4SwTQAevXysE2RbFDYdAiEBCUEaRQnMnbp7\n9mxDXDf6AU0cN/RPBjb9qSHDcWZHGzUCIG2Es59z8ugGrDY+pxLQnwfotadxd+Uy\nv/Ow5T0q5gIJAiEAyS4RaI9YG8EWx/2w0T67ZUVAw8eOMB6BIUg0Xcu+3okCIBOs\n/5OiPgoTdSy7bcF9IGpSE8ZgGKzgYQVZeN97YE00\n-----END RSA PRIVATE KEY-----\n"
	p := ProxmoxProbe{}
	_, err := p.GetVMs(context.Background(), ProxmoxCreds{
		Endpoint: "https://anywhere", TokenID: "u@p!t", TokenSecret: "x",
		CABundle: bundle,
	})
	if err == nil {
		t.Fatal("expected error for non-CERTIFICATE PEM block, got nil")
	}
}
