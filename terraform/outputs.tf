###################################
# Root-level outputs consumed by the Bandolier executor
#
# The executor's waitForSSH (api/internal/deployments/wait_ssh.go) reads
# master_ip / agent1_ip / agent2_ip from `terraform output -json`. We use
# the static IPs from the form rather than the VM's reported guest-agent
# IP, because:
#   1. The IPs ARE statically configured via cloud-init ipconfig0.
#   2. The guest agent may not be ready when terraform output runs,
#      leaving the bpg-emitted ipv4 attribute null.
###################################

output "master_ip" {
  description = "Static IP of the k3s server node."
  value       = var.network_master_ip
}

output "agent1_ip" {
  description = "Static IP of the first k3s agent node."
  value       = var.network_agent1_ip
}

output "agent2_ip" {
  description = "Static IP of the second k3s agent node."
  value       = var.network_agent2_ip
}
