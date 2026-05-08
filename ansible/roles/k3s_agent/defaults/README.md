# 📖 k3s Agent Defaults Overview

This file defines the **default variables for the k3s agent role** in Ansible. It provides baseline configuration values that can be overridden in inventory, playbooks, or extra-vars.

---

## Key Variables

| Variable              | Description                                                                                |
| --------------------- | ------------------------------------------------------------------------------------------ |
| `server_group`        | Ansible inventory group name for server nodes. Agents will connect to nodes in this group. |
| `k3s_server_location` | Path where k3s stores cluster data on the agent node.                                      |
| `systemd_dir`         | Directory where the k3s systemd service file will be placed.                               |
| `api_port`            | Kubernetes API port of the server (default 6443).                                          |
| `extra_agent_args`    | Additional command-line arguments to pass to the k3s agent at startup.                     |
| `extra_install_envs`  | Dictionary of extra environment variables for k3s installation or startup.                 |

---

## Purpose

* Provides safe default values for agent deployment.
* Ensures consistency across nodes.
* Enables customization without modifying the role code.
* Supports dynamic configuration via Ansible for multi-node clusters.
