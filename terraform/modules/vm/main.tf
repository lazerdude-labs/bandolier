
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
    bridge  = var.bridge
    vlan_id = var.vlan_id
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

