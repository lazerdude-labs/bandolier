
resource "proxmox_virtual_environment_vm" "vm" {

  name        = var.name
  description = "Built by Terraform and managed by Ansible"
  tags        = ["terraform"]
  node_name   = var.node


  agent {
    enabled = true
  }

  cpu {
    cores   = var.cores
    sockets = var.sockets
    type    = var.type
  }

  memory {
    dedicated = var.ram
  }

  disk {

    datastore_id = var.datastore_id
    file_id      = var.file_id
    interface    = var.interface
    iothread     = var.iothread
    size         = var.disk_size
  }

  network_device {
    bridge = var.bridge
    # vlan_id == 0 is the sentinel for "untagged / flat network". Passing
    # null tells the bpg/proxmox provider to omit the tag entirely, which
    # is what Proxmox itself wants for a bridge that's not VLAN-aware (or
    # for the default untagged VLAN on a VLAN-aware bridge). 1-4094 is a
    # standard 802.1Q tag.
    vlan_id = var.vlan_id == 0 ? null : var.vlan_id
    model   = var.model
  }

  initialization {
    # Pin the cloud-init drive to the same datastore as the VM disk. Without
    # this, the bpg/proxmox provider defaults to "local-lvm", which fails on
    # any Proxmox host that doesn't have local-lvm — RBD-backed setups,
    # Ceph-only homelabs, etc. Reported by an early user against vm_data
    # (Ceph RBD).
    datastore_id = var.datastore_id
    dns {
      servers = var.dns
    }
    ip_config {
      ipv4 {
        address = var.ip_address
        gateway = var.gateway
      }
    }
    user_data_file_id = var.user_data_file_id

  }
  lifecycle {
    ignore_changes = [
      initialization[0].user_data_file_id,
    ]
  }
}

