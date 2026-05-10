# terraform/cloud_image.tf
#
# Bandolier hands Proxmox a URL to fetch the base cloud image — the Proxmox
# host downloads it server-side and verifies against the SHA256, then the
# resulting file_id feeds the VM disk blocks in main.tf.
#
# Two paths:
#
# 1. Default (`proxmox_image_pre_uploaded = false`): use
#    `proxmox_virtual_environment_download_file`. Proxmox issues a HEAD
#    against the URL and downloads it. Idempotent via `overwrite = false`.
#
# 2. Pre-uploaded (`proxmox_image_pre_uploaded = true`): use
#    `proxmox_virtual_environment_file` data source. Operator scp'd the
#    file to the iso storage themselves (workaround when the upstream
#    CDN HEAD-blocks Proxmox's User-Agent — known Rocky CDN issue).
#
# `count` toggles which one terraform actually instantiates so a single
# tfvars file can drive either path.

resource "proxmox_virtual_environment_download_file" "cloud_image" {
  count = var.proxmox_image_pre_uploaded ? 0 : 1

  content_type        = "iso"
  datastore_id        = var.proxmox_image_storage
  node_name           = var.proxmox_node
  url                 = var.proxmox_image_url
  file_name           = var.proxmox_image_filename
  checksum            = var.proxmox_image_sha256
  checksum_algorithm  = "sha256"
  overwrite           = false
  overwrite_unmanaged = false
}

data "proxmox_virtual_environment_file" "cloud_image" {
  count = var.proxmox_image_pre_uploaded ? 1 : 0

  content_type = "iso"
  datastore_id = var.proxmox_image_storage
  node_name    = var.proxmox_node
  file_name    = var.proxmox_image_filename
}
