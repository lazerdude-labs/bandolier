###################################
# Define VM   
###################################

locals {

  vm_definitions = {


    # Below builds the master server up
    master1 = {
      node         = local.proxmox_config.node
      cores        = 2
      sockets      = 2
      type         = "x86-64-v2-AES"
      ram          = 8192
      disk_size    = 120
      datastore_id = var.proxmox_storage
      file_id      = local.cloud_image_file_id
      interface    = "virtio0"
      iothread     = true
      bridge       = local.network_config.bridge_name
      vlan_id      = local.network_config.vlan_id
      model        = "virtio"
      ip_address   = local.network_config.ip_address_master
      gateway      = local.network_config.gateway_ip
      dns          = local.network_config.local_dns
      group        = "server"
    }
    # Builds first worker
    worker1 = {
      node         = local.proxmox_config.node
      cores        = 2
      sockets      = 2
      type         = "x86-64-v2-AES"
      ram          = 4096
      disk_size    = 120
      datastore_id = var.proxmox_storage
      file_id      = local.cloud_image_file_id
      interface    = "virtio0"
      iothread     = true
      bridge       = local.network_config.bridge_name
      vlan_id      = local.network_config.vlan_id
      model        = "virtio"
      ip_address   = local.network_config.ip_address_worker1
      gateway      = local.network_config.gateway_ip
      dns          = local.network_config.local_dns
      group        = "agent"
    }
    # Builds 2nd Worker
    worker2 = {
      node         = local.proxmox_config.node
      cores        = 2
      sockets      = 2
      type         = "x86-64-v2-AES"
      ram          = 4096
      disk_size    = 120
      datastore_id = var.proxmox_storage
      file_id      = local.cloud_image_file_id
      interface    = "virtio0"
      iothread     = true
      bridge       = local.network_config.bridge_name
      vlan_id      = local.network_config.vlan_id
      model        = "virtio"
      ip_address   = local.network_config.ip_address_worker2
      gateway      = local.network_config.gateway_ip
      dns          = local.network_config.local_dns
      group        = "agent"
    }
  }
}

###################################
# Call VM Module
################################### 
#Create per-VM cloud-init files from template
resource "local_file" "cloudinit" {
  for_each = local.vm_definitions
  filename = "${path.root}/terraform/cloud-init/generated-${each.key}.yml"
  content = templatefile("${path.module}/cloud-init/rocky-config.tpl", {
    name           = each.key
    ssh_public_key = local.ssh_config.public_key
    fqdn           = "${each.key}.${local.network_config.fqdn}"
  })
}

# 2️⃣ Upload to Proxmox
resource "proxmox_virtual_environment_file" "rocky_config" {
  for_each     = local.vm_definitions
  content_type = "snippets"
  datastore_id = var.proxmox_snippets_storage
  node_name    = each.value.node

  source_file {
    path = local_file.cloudinit[each.key].filename
  }

  lifecycle {
    ignore_changes = [
      source_file
    ]
  }
}

module "vms" {
  source            = "./modules/vm"
  for_each          = local.vm_definitions
  name              = each.key
  node              = each.value.node
  cores             = each.value.cores
  sockets           = each.value.sockets
  type              = each.value.type
  ram               = each.value.ram
  disk_size         = each.value.disk_size
  datastore_id      = each.value.datastore_id
  file_id           = each.value.file_id
  interface         = each.value.interface
  iothread          = each.value.iothread
  bridge            = each.value.bridge
  vlan_id           = each.value.vlan_id
  model             = each.value.model
  ip_address        = each.value.ip_address
  gateway           = each.value.gateway
  dns               = each.value.dns
  group             = each.value.group
  user_data_file_id = proxmox_virtual_environment_file.rocky_config[each.key].id
}

