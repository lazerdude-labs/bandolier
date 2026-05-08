resource "local_file" "ansible_inventory" {
  depends_on = [time_sleep.wait_for_ips]

  filename = "${path.root}/../ansible/inventory.ini"

  content = templatefile("${path.module}/inventory.ini.tmpl", {
    inventory      = local.inventory
    groups         = local.groups
    cluster_groups = local.cluster_groups
  })
}

resource "time_sleep" "wait_for_ips" {
  depends_on      = [module.vms]
  create_duration = "15s"
}
