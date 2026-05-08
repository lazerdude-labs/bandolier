# 📖 main.yml Defaults Overview

This file defines the **default variables** for the `k3s_server` role.

These defaults provide a baseline configuration for the role and can be overridden in inventory, playbooks, or extra-vars.

---

## Key Variables

| Variable               | Description                                                   |
| -----------------------   | ---------------------------------------------------------------- |
| `k3s_server_location`     | Directory where k3s stores cluster data (`/var/lib/rancher/k3s`) |
| `systemd_dir`             | Path to systemd unit files (`/etc/systemd/system`)               |
| `api_port`                | Kubernetes API server port (default 6443)                        |
| `kubeconfig`              | Path to generated kubeconfig (`~/.kube/config.new`)              |
| `user_kubectl`            | Whether non-root user can use `kubectl` (true/false)             |
| `cluster_context`         | Name of the kubectl context for this cluster                     |
| `server_group`            | Ansible inventory group for k3s server nodes                     |
| `agent_group`             | Ansible inventory group for k3s agent nodes                      |
| `use_external_database`   | Use external DB for k3s (false = default SQLite)                 |
| `extra_server_args`       | Extra CLI flags for k3s server                                   |
| `extra_install_envs`      | Extra environment variables for installation/startup             |

---

## Purpose

* Provide **safe default values** for the role.
* Ensure role is **reusable and plug-and-play**.
* Allow customization without changing the role code.
