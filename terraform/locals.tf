locals {
  inventory = {
    for name, vm in module.vms :
    name => {
      ansible_host = vm.ipv4
      group        = vm.group
    }
  }

  clusters = {
    k3s    = ["server", "agent"]
    docker = ["docker"]
  }

  cluster_groups = {
    for cluster, roles in local.clusters :
    cluster => [
      for g in local.groups : g
      if contains(roles, g)
    ]
  }
  groups = distinct([
    for vm in local.inventory : vm.group
  ])


}