package clusters

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/lazerdude-labs/bandolier/api/internal/store"
)

// initializeView is what GET /api/clusters/{id}/initialize returns. Mirrors
// the request shape from POST /initialize except secret fields are stripped
// — the operator is free to retrieve and re-submit non-secret values for
// edit, but the wire never carries the cleartext token/password/private
// key out of Vault. The `secrets_present` array tells the UI which secret
// fields exist in Vault so the wizard can show "Leave blank to keep
// existing" hints next to those inputs.
type initializeView struct {
	Proxmox struct {
		Endpoint        string `json:"endpoint"`
		TokenID         string `json:"token_id"`
		Node            string `json:"node"`
		Storage         string `json:"storage"`
		Username        string `json:"username"`
		ImageStorage    string `json:"image_storage"`
		SnippetsStorage string `json:"snippets_storage"`
		Distro          string `json:"distro"`
		CustomURL       string `json:"custom_url"`
		CustomSHA256    string `json:"custom_sha256"`
		CABundle        string `json:"ca_bundle"`
	} `json:"proxmox"`
	Network struct {
		CIDR             string   `json:"cidr"`
		Gateway          string   `json:"gateway"`
		DNS              []string `json:"dns"`
		FQDN             string   `json:"fqdn"`
		MasterIP         string   `json:"master_ip"`
		Agent1IP         string   `json:"agent1_ip"`
		Agent2IP         string   `json:"agent2_ip"`
		VLAN             int      `json:"vlan"`
		BridgeName       string   `json:"bridge_name"`
		TraefikDashboard *bool    `json:"traefik_dashboard"`
		DNSServer        string   `json:"dns_server"`
		DNSZone          string   `json:"dns_zone"`
		TSIGName         string   `json:"tsig_name"`
	} `json:"network"`
	SSH struct {
		PublicKey string `json:"public_key"`
		BYO       bool   `json:"byo"`
	} `json:"ssh"`
	// SecretsPresent lists field names the wizard should treat as
	// "already-set; leave the form input blank to keep". Possible entries:
	// "proxmox.token_secret", "proxmox.password", "ssh.private_key", and
	// "network.tsig_secret".
	SecretsPresent []string `json:"secrets_present"`
}

// HandleGet returns the cluster's current initialize values for re-edit.
// Sensitive fields (token secret, password, private key, TSIG secret) are
// never returned — the response includes a `secrets_present` array instead
// so the UI can render "Leave blank to keep existing" hints. Available for
// any cluster that has been initialized at least once (status != pending).
func (i *Initializer) HandleGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	c, err := i.store.GetCluster(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if Status(c.Status) == StatusPending {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "cluster has not been initialized"})
		return
	}

	prox, err := i.vault.Get(r.Context(), i.paths.Proxmox(id))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "vault proxmox: " + err.Error()})
		return
	}
	net, err := i.vault.Get(r.Context(), i.paths.Network(id))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "vault network: " + err.Error()})
		return
	}
	sshCfg, err := i.vault.Get(r.Context(), i.paths.SSH(id))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "vault ssh: " + err.Error()})
		return
	}

	var v initializeView
	v.Proxmox.Endpoint = stringFrom(prox, "endpoint")
	v.Proxmox.TokenID = stringFrom(prox, "token_id")
	v.Proxmox.Node = stringFrom(prox, "node")
	v.Proxmox.Storage = stringFrom(prox, "storage")
	v.Proxmox.Username = stringFrom(prox, "username")
	v.Proxmox.ImageStorage = stringFrom(prox, "image_storage")
	v.Proxmox.SnippetsStorage = stringFrom(prox, "snippets_storage")
	v.Proxmox.Distro = stringFrom(prox, "distro")
	v.Proxmox.CustomURL = stringFrom(prox, "custom_url")
	v.Proxmox.CustomSHA256 = stringFrom(prox, "custom_sha256")
	v.Proxmox.CABundle = stringFrom(prox, "ca_bundle")

	v.Network.CIDR = stringFrom(net, "cidr")
	v.Network.Gateway = stringFrom(net, "gateway")
	v.Network.DNS = stringsFrom(net, "dns")
	v.Network.FQDN = stringFrom(net, "fqdn")
	v.Network.MasterIP = stringFrom(net, "master_ip")
	v.Network.Agent1IP = stringFrom(net, "agent1_ip")
	v.Network.Agent2IP = stringFrom(net, "agent2_ip")
	v.Network.VLAN = intFrom(net, "vlan")
	v.Network.BridgeName = stringFrom(net, "bridge_name")
	if td, ok := net["traefik_dashboard"].(bool); ok {
		v.Network.TraefikDashboard = &td
	}
	v.Network.DNSServer = stringFrom(net, "dns_server")
	v.Network.DNSZone = stringFrom(net, "dns_zone")
	v.Network.TSIGName = stringFrom(net, "tsig_name")

	v.SSH.PublicKey = stringFrom(sshCfg, "public_key")
	if byo, ok := sshCfg["byo"].(bool); ok {
		v.SSH.BYO = byo
	}

	if stringFrom(prox, "token_secret") != "" {
		v.SecretsPresent = append(v.SecretsPresent, "proxmox.token_secret")
	}
	if stringFrom(prox, "password") != "" {
		v.SecretsPresent = append(v.SecretsPresent, "proxmox.password")
	}
	if stringFrom(net, "tsig_secret") != "" {
		v.SecretsPresent = append(v.SecretsPresent, "network.tsig_secret")
	}
	if stringFrom(sshCfg, "private_key") != "" {
		v.SecretsPresent = append(v.SecretsPresent, "ssh.private_key")
	}

	writeJSON(w, http.StatusOK, v)
}

// stringFrom is a small helper for reading optional string fields out of
// vault.Get's map[string]any. Returns "" when the key is missing or non-
// string, which is the behavior every consumer wants for these fields.
func stringFrom(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

// intFrom reads a numeric field, tolerating both float64 (the JSON default
// after vault round-trips) and int.
func intFrom(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	}
	return 0
}

// stringsFrom reads a string slice, tolerating the []any shape JSON
// unmarshal produces from a JSON array.
func stringsFrom(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}
	switch v := m[key].(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, x := range v {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
