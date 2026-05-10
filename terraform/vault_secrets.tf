###################################
# Config locals (sourced from input variables)
#
# Secrets are no longer read from Vault here. The Bandolier API reads
# secrets from Vault and passes them to terraform via terraform.tfvars.json
# at deploy time. This keeps terraform stateless with respect to Vault and
# avoids needing a Vault token in the terraform execution environment.
###################################

locals {
  # Proxmox SSH usernames don't have a realm (e.g. "@pam"). Strip the realm
  # suffix from the API username so the same input field works for both
  # API auth (username@realm) and SSH (just username).
  proxmox_ssh_username = split("@", var.proxmox_username)[0]

  proxmox_config = {
    endpoint     = var.proxmox_endpoint
    api_token_id = "${var.proxmox_token_id}=${var.proxmox_token_secret}"
    username     = local.proxmox_ssh_username
    password     = var.proxmox_password
    node         = var.proxmox_node
  }

  # Resolved file_id of the cloud image — either the resource (Proxmox
  # downloaded it) or the data source (operator pre-uploaded it). Both
  # produce the same id shape: "<storage>:iso/<filename>".
  cloud_image_file_id = (
    var.proxmox_image_pre_uploaded
    ? data.proxmox_virtual_environment_file.cloud_image[0].id
    : proxmox_virtual_environment_download_file.cloud_image[0].id
  )

  # bpg/proxmox's initialization.ip_config.ipv4.address requires CIDR notation
  # (e.g. "192.0.2.21/24"). The form collects bare IPs + the network CIDR;
  # we derive the prefix length and append it here.
  network_prefix = split("/", var.network_cidr)[1]

  network_config = {
    bridge_name        = var.network_bridge_name
    vlan_id            = var.network_vlan
    ip_address_master  = "${var.network_master_ip}/${local.network_prefix}"
    ip_address_worker1 = "${var.network_agent1_ip}/${local.network_prefix}"
    ip_address_worker2 = "${var.network_agent2_ip}/${local.network_prefix}"
    gateway_ip         = var.network_gateway
    local_dns          = split(",", var.network_dns)
    fqdn               = var.network_fqdn
  }

  ssh_config = {
    public_key = var.ssh_public_key
  }
}
