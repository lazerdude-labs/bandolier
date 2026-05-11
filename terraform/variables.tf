###################################
# Cluster identity
###################################

variable "cluster_id" {
  description = "Bandolier cluster ID; used to scope Vault paths and label resources."
  type        = string
}

variable "vault_kv_mount" {
  description = "Vault KV v2 mount used by Bandolier (retained for reference; terraform no longer reads Vault directly)."
  type        = string
  default     = "bandolier"
}

###################################
# Proxmox
###################################

variable "proxmox_endpoint" {
  description = "Proxmox API endpoint URL."
  type        = string
}

variable "proxmox_token_id" {
  description = "Proxmox API token ID (user@realm!token-name)."
  type        = string
}

variable "proxmox_token_secret" {
  description = "Proxmox API token secret."
  type        = string
  sensitive   = true
}

variable "proxmox_username" {
  description = "Proxmox SSH username (used by the bpg provider ssh block)."
  type        = string
}

variable "proxmox_password" {
  description = "Proxmox SSH password (used by the bpg provider ssh block)."
  type        = string
  sensitive   = true
}

variable "proxmox_node" {
  description = "Proxmox node name where VMs are provisioned."
  type        = string
}

variable "proxmox_storage" {
  description = "Proxmox storage ID for VM disks."
  type        = string
  default     = "local-lvm"
}

variable "proxmox_image_url" {
  description = "URL Proxmox will fetch the cloud image qcow2 from."
  type        = string
}

variable "proxmox_image_sha256" {
  description = "Hex SHA256 of the qcow2; Proxmox verifies the download against this."
  type        = string
}

variable "proxmox_image_filename" {
  description = "Deterministic filename on Proxmox iso storage (becomes part of the file_id)."
  type        = string
}

variable "proxmox_image_storage" {
  description = "Proxmox storage pool (with 'iso' content type) where the image lands."
  type        = string
  default     = "local"
}

variable "proxmox_image_pre_uploaded" {
  description = "When true, terraform skips the cloud-image download and references an existing file at <proxmox_image_storage>:iso/<proxmox_image_filename>. Workaround for upstream CDN HEAD-blocks (e.g. Rocky's dl.rockylinux.org filtering Proxmox's User-Agent) — operator pre-uploads via `scp` and Bandolier never fetches."
  type        = bool
  default     = false
}

variable "proxmox_snippets_storage" {
  description = "Proxmox storage pool (with 'snippets' content type) where per-VM cloud-init configs are uploaded. Must have 'snippets' enabled in `pvesm set <storage> --content ...,snippets`."
  type        = string
  default     = "local"
}

###################################
# Network
###################################

variable "network_cidr" {
  description = "Network CIDR (e.g. 192.0.2.0/24). Prefix length is appended to per-VM IPs for cloud-init's ipconfig0."
  type        = string
}

variable "network_bridge_name" {
  description = "Proxmox bridge name for VM network interfaces."
  type        = string
}

variable "network_vlan" {
  description = "802.1Q VLAN tag for VM network interfaces. 1-4094 for a tagged VLAN; 0 to leave the network_device untagged (flat-network setups, or the default untagged VLAN on a VLAN-aware bridge)."
  type        = number
  validation {
    condition     = var.network_vlan >= 0 && var.network_vlan <= 4094
    error_message = "network_vlan must be 0 (untagged) or 1-4094 (tagged)."
  }
}

variable "network_master_ip" {
  description = "Static IP address for the k3s server (master) node."
  type        = string
}

variable "network_agent1_ip" {
  description = "Static IP address for k3s agent node 1."
  type        = string
}

variable "network_agent2_ip" {
  description = "Static IP address for k3s agent node 2."
  type        = string
}

variable "network_gateway" {
  description = "Default gateway IP for the k3s VLAN."
  type        = string
}

variable "network_dns" {
  description = "Comma-separated list of DNS server IPs."
  type        = string
}

variable "network_fqdn" {
  description = "Base domain/FQDN used for node hostnames."
  type        = string
}

###################################
# SSH
###################################

variable "ssh_public_key" {
  description = "SSH public key injected into VMs via cloud-init."
  type        = string
}
