package telemetry

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type ProxmoxCreds struct {
	Endpoint    string // e.g. https://198.51.100.253:8006
	TokenID     string // user@realm!tokenname
	TokenSecret string // UUID
	CABundle    string // PEM-encoded; empty = use InsecureSkipVerify (homelab default)
}

type ProxmoxVM struct {
	Node   string `json:"node"`
	VMID   int    `json:"vmid"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type ProxmoxProbe struct {
	Client *http.Client
}

// GetVMs queries the Proxmox cluster resources endpoint and returns the qemu
// VMs visible to the token. We filter out LXC containers and other types.
func (p ProxmoxProbe) GetVMs(ctx context.Context, creds ProxmoxCreds) ([]ProxmoxVM, error) {
	client := p.Client
	if client == nil {
		tlsCfg, err := buildTLSConfig(creds.CABundle)
		if err != nil {
			return nil, err
		}
		client = &http.Client{Transport: &http.Transport{TLSClientConfig: tlsCfg}}
	}
	url := strings.TrimSuffix(creds.Endpoint, "/") + "/api2/json/cluster/resources?type=vm"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", creds.TokenID, creds.TokenSecret))

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("proxmox get: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("proxmox status %d: %s", res.StatusCode, string(body))
	}

	var resp struct {
		Data []struct {
			Type   string `json:"type"`
			Node   string `json:"node"`
			VMID   int    `json:"vmid"`
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse proxmox json: %w", err)
	}
	out := make([]ProxmoxVM, 0, len(resp.Data))
	for _, v := range resp.Data {
		if v.Type != "qemu" {
			continue
		}
		out = append(out, ProxmoxVM{Node: v.Node, VMID: v.VMID, Name: v.Name, Status: v.Status})
	}
	return out, nil
}

// buildTLSConfig returns a tls.Config from a PEM-encoded CA bundle. Empty
// bundle yields InsecureSkipVerify (accepted homelab risk). Each PEM block
// is validated as a CERTIFICATE and parsed before adding to the pool;
// non-certificate blocks (e.g. attacker-supplied keys, malformed PEM)
// cause an error so operators don't silently trust extra material.
func buildTLSConfig(caBundle string) (*tls.Config, error) {
	if caBundle == "" {
		return &tls.Config{InsecureSkipVerify: true}, nil
	}
	pool := x509.NewCertPool()
	rest := []byte(caBundle)
	found := 0
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("proxmox: CA bundle has non-certificate block type %q", block.Type)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("proxmox: parse cert in CA bundle: %w", err)
		}
		pool.AddCert(cert)
		found++
	}
	if found == 0 {
		return nil, fmt.Errorf("proxmox: malformed CA bundle PEM")
	}
	return &tls.Config{RootCAs: pool}, nil
}
