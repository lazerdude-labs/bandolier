
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
    dns {
      servers = var.dns #Change this later on to the DNS server that is being stood up
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

