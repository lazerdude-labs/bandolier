# Any files you want output on it translate it here


output "ipv4" {
  description = "First non-loopback IPv4 address reported by guest agent"
  value = try(
    [
      for ip in flatten(proxmox_virtual_environment_vm.vm.ipv4_addresses) :
      ip if !startswith(ip, "127.")
    ][0],
    null
  )
}


output "name" {
  value = var.name
}

output "group" {
  value = var.group
}

