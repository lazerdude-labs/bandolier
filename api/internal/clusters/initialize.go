package clusters

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/ssh"

	"github.com/lazerdude-labs/bandolier/api/internal/audit"
	"github.com/lazerdude-labs/bandolier/api/internal/auth"
	"github.com/lazerdude-labs/bandolier/api/internal/dns"
	"github.com/lazerdude-labs/bandolier/api/internal/profiles/homelab"
	"github.com/lazerdude-labs/bandolier/api/internal/store"
	"github.com/lazerdude-labs/bandolier/api/internal/vault"
)

type Initializer struct {
	store *store.Store
	vault *vault.Client
	paths vault.Paths
}

func NewInitializer(s *store.Store, v *vault.Client) *Initializer {
	return &Initializer{store: s, vault: v}
}

type initRequest struct {
	Proxmox struct {
		Endpoint     string `json:"endpoint"`
		TokenID      string `json:"token_id"`
		TokenSecret  string `json:"token_secret"`
		Node         string `json:"node"`
		Storage      string `json:"storage"`
		Username     string `json:"username"`
		Password     string `json:"password"`
		ImageStorage    string `json:"image_storage"`
		SnippetsStorage string `json:"snippets_storage"`
		Distro          string `json:"distro"`
		CustomURL    string `json:"custom_url"`
		CustomSHA256 string `json:"custom_sha256"`
		CABundle     string `json:"ca_bundle"`
	} `json:"proxmox"`
	Network struct {
		CIDR       string   `json:"cidr"`
		Gateway    string   `json:"gateway"`
		DNS        []string `json:"dns"`
		FQDN       string   `json:"fqdn"`
		MasterIP   string   `json:"master_ip"`
		Agent1IP   string   `json:"agent1_ip"`
		Agent2IP   string   `json:"agent2_ip"`
		VLAN       int      `json:"vlan"`
		BridgeName string   `json:"bridge_name"`
		TraefikDashboard *bool `json:"traefik_dashboard"` // nil → default true
		DNSServer  string `json:"dns_server"`
		DNSZone    string `json:"dns_zone"`
		TSIGName   string `json:"tsig_name"`
		TSIGSecret string `json:"tsig_secret"`
	} `json:"network"`
	SSH struct {
		PublicKey  string `json:"public_key"`
		PrivateKey string `json:"private_key"`
	} `json:"ssh"`
}

func (i *Initializer) Handle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	uid, _ := auth.UserIDFromContext(r.Context())

	c, err := i.store.GetCluster(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := CanTransition(Status(c.Status), StatusInitializing); err != nil {
		_, _ = audit.Write(r.Context(), i.store, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionClusterInit),
			Target:  id,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "invalid_state_transition"},
		})
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	var req initRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		_, _ = audit.Write(r.Context(), i.store, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionClusterInit),
			Target:  id,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "invalid_json"},
		})
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.Proxmox.Endpoint == "" || req.Proxmox.TokenID == "" || req.Proxmox.TokenSecret == "" ||
		req.Proxmox.Username == "" || req.Proxmox.Password == "" ||
		req.Network.MasterIP == "" || req.Network.Agent1IP == "" || req.Network.Agent2IP == "" ||
		req.Network.BridgeName == "" {
		_, _ = audit.Write(r.Context(), i.store, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionClusterInit),
			Target:  id,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "missing_required_fields"},
		})
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing required fields"})
		return
	}
	if req.Proxmox.ImageStorage == "" {
		req.Proxmox.ImageStorage = "local"
	}
	if req.Proxmox.SnippetsStorage == "" {
		req.Proxmox.SnippetsStorage = "local"
	}
	if _, err := homelab.ResolveImage(req.Proxmox.Distro, req.Proxmox.CustomURL, req.Proxmox.CustomSHA256); err != nil {
		_, _ = audit.Write(r.Context(), i.store, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionClusterInit),
			Target:  id,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "image_resolve"},
		})
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "image: " + err.Error()})
		return
	}
	// DNS is optional. If all four DNS fields are empty, the operator manages
	// DNS themselves; persist kind=none and skip the BIND pre-flight. Wildcard
	// TLS issuance via Vault PKI continues unconditionally — the cert is valid
	// for the operator-supplied FQDN regardless of who owns the records.
	dnsKind := "bind"
	if req.Network.DNSServer == "" && req.Network.DNSZone == "" &&
		req.Network.TSIGName == "" && req.Network.TSIGSecret == "" {
		dnsKind = "none"
	}

	_ = i.store.UpdateClusterStatus(r.Context(), id, string(StatusInitializing))

	// SSH key: operator may paste their own keypair (BYO mode), or leave both
	// blank to have Bandolier generate. One blank + one set is rejected.
	sshPub := req.SSH.PublicKey
	sshPriv := req.SSH.PrivateKey
	byo := false
	switch {
	case sshPub != "" && sshPriv != "":
		// BYO — use what operator pasted, skip keygen
		byo = true
	case sshPub == "" && sshPriv == "":
		// Auto-gen
		p, k, err := generateSSHKey()
		if err != nil {
			_ = i.store.UpdateClusterStatus(r.Context(), id, string(StatusError))
			_, _ = audit.Write(r.Context(), i.store, audit.Entry{
				ActorID: uid,
				Action:  string(audit.ActionClusterInit),
				Target:  id,
				Outcome: audit.OutcomeFailure,
				Details: map[string]any{"reason": "ssh_keygen"},
			})
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "ssh keygen: " + err.Error()})
			return
		}
		sshPub = p
		sshPriv = k
	default:
		_ = i.store.UpdateClusterStatus(r.Context(), id, string(StatusError))
		_, _ = audit.Write(r.Context(), i.store, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionClusterInit),
			Target:  id,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "ssh_byo_partial"},
		})
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "ssh: provide both public + private key, or neither (for auto-gen)",
		})
		return
	}

	if err := i.vault.Put(r.Context(), i.paths.Proxmox(id), map[string]any{
		"endpoint":      req.Proxmox.Endpoint,
		"token_id":      req.Proxmox.TokenID,
		"token_secret":  req.Proxmox.TokenSecret,
		"node":          req.Proxmox.Node,
		"storage":       req.Proxmox.Storage,
		"username":      req.Proxmox.Username,
		"password":      req.Proxmox.Password,
		"image_storage":    req.Proxmox.ImageStorage,
		"snippets_storage": req.Proxmox.SnippetsStorage,
		"distro":           req.Proxmox.Distro,
		"custom_url":    req.Proxmox.CustomURL,
		"custom_sha256": req.Proxmox.CustomSHA256,
		"ca_bundle":     req.Proxmox.CABundle,
	}); err != nil {
		_ = i.store.UpdateClusterStatus(r.Context(), id, string(StatusError))
		_, _ = audit.Write(r.Context(), i.store, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionClusterInit),
			Target:  id,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "vault_proxmox"},
		})
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "vault proxmox"})
		return
	}
	if err := i.vault.Put(r.Context(), i.paths.Network(id), map[string]any{
		"cidr":        req.Network.CIDR,
		"gateway":     req.Network.Gateway,
		"dns":         req.Network.DNS,
		"fqdn":        req.Network.FQDN,
		"master_ip":   req.Network.MasterIP,
		"agent1_ip":   req.Network.Agent1IP,
		"agent2_ip":   req.Network.Agent2IP,
		"vlan":        req.Network.VLAN,
		"bridge_name": req.Network.BridgeName,
		"traefik_dashboard": req.Network.TraefikDashboard == nil || *req.Network.TraefikDashboard,
	}); err != nil {
		_ = i.store.UpdateClusterStatus(r.Context(), id, string(StatusError))
		_, _ = audit.Write(r.Context(), i.store, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionClusterInit),
			Target:  id,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "vault_network"},
		})
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "vault network"})
		return
	}
	if err := i.vault.Put(r.Context(), i.paths.SSH(id), map[string]any{
		"public_key":  sshPub,
		"private_key": sshPriv,
		"byo":         byo,
	}); err != nil {
		_ = i.store.UpdateClusterStatus(r.Context(), id, string(StatusError))
		_, _ = audit.Write(r.Context(), i.store, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionClusterInit),
			Target:  id,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "vault_ssh"},
		})
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "vault ssh"})
		return
	}
	if err := i.vault.Put(r.Context(), "clusters/"+id+"/dns", map[string]any{
		"kind":        dnsKind,
		"server":      req.Network.DNSServer,
		"zone":        req.Network.DNSZone,
		"tsig_name":   req.Network.TSIGName,
		"tsig_secret": req.Network.TSIGSecret,
	}); err != nil {
		_ = i.store.UpdateClusterStatus(r.Context(), id, string(StatusError))
		_, _ = audit.Write(r.Context(), i.store, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionClusterInit),
			Target:  id,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "vault_dns"},
		})
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "vault dns write: " + err.Error()})
		return
	}
	if err := i.vault.Put(r.Context(), "clusters/"+id+"/tls", map[string]any{
		"pki_role": "traefik",
	}); err != nil {
		_ = i.store.UpdateClusterStatus(r.Context(), id, string(StatusError))
		_, _ = audit.Write(r.Context(), i.store, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionClusterInit),
			Target:  id,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "vault_tls"},
		})
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "vault tls write: " + err.Error()})
		return
	}

	if dnsKind != "none" {
		provider, err := dns.NewProvider(dns.Config{
			Kind:       dns.KindBind,
			Server:     req.Network.DNSServer,
			Zone:       req.Network.DNSZone,
			TSIGName:   req.Network.TSIGName,
			TSIGSecret: req.Network.TSIGSecret,
		})
		if err != nil {
			_ = i.store.UpdateClusterStatus(r.Context(), id, string(StatusError))
			_, _ = audit.Write(r.Context(), i.store, audit.Entry{
				ActorID: uid,
				Action:  string(audit.ActionClusterInit),
				Target:  id,
				Outcome: audit.OutcomeFailure,
				Details: map[string]any{"reason": "dns_provider"},
			})
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "dns provider: " + err.Error()})
			return
		}
		if err := provider.Healthy(r.Context()); err != nil {
			_ = i.store.UpdateClusterStatus(r.Context(), id, string(StatusError))
			_, _ = audit.Write(r.Context(), i.store, audit.Entry{
				ActorID: uid,
				Action:  string(audit.ActionClusterInit),
				Target:  id,
				Outcome: audit.OutcomeFailure,
				Details: map[string]any{"reason": "dns_preflight"},
			})
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "dns pre-flight: " + err.Error()})
			return
		}
	}

	_ = i.store.UpdateClusterStatus(r.Context(), id, string(StatusInitialized))
	_, _ = audit.Write(r.Context(), i.store, audit.Entry{
		ActorID: uid,
		Action:  string(audit.ActionClusterInit),
		Target:  id,
		Outcome: audit.OutcomeSuccess,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "initialized"})
}

func generateSSHKey() (publicAuthorized, privatePEM string, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return "", "", err
	}
	publicAuthorized = string(ssh.MarshalAuthorizedKey(sshPub))

	pemBlock, err := ssh.MarshalPrivateKey(priv, "bandolier-cluster")
	if err != nil {
		return "", "", err
	}
	privatePEM = string(pem.EncodeToMemory(pemBlock))
	return
}
