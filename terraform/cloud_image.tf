# terraform/cloud_image.tf
#
# Bandolier's terraform downloads the cluster's base cloud image directly
# to the operator's Proxmox storage. The Proxmox host fetches the URL
# itself (no Bandolier-side download), validates against the sha256, and
# the resulting file_id feeds the VM disk blocks in main.tf.
#
# `overwrite = false` makes apply idempotent — second cluster on the same
# host with the same image sees the file exists and skips.
resource "proxmox_virtual_environment_download_file" "cloud_image" {
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
