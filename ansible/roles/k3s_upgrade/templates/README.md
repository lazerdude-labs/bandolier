# 📖 k3s.service Systemd Unit Overview

This file is a **custom systemd service unit** for the k3s server. While the standard k3s install script automatically creates a systemd service, this template is used in an Ansible-managed homelab to allow **custom paths, dynamic arguments, cluster initialization flags, and environment variable injection**.

It ensures the k3s server starts automatically, restarts on failure, and is configured consistently across nodes.

---

## [Unit] 

| Line                                 | Purpose                                               |
| -------------------------------------------| ----------------------------------------------------- |
| `Description=Lightweight Kubernetes --` |    Describes the service in systemd.                    |
| `Documentation=https://k3s.io`       |    Provides a reference link.                            |
| `Wants=network-online.target`        |     Indicates the service wants the network to be up.     |
| `After=network-online.target`        |    Ensures the service starts after networking is ready. |

## [Install] 

| Line                         | Purpose                                                                    |
| ---------------------------- | -------------------------------------------------------------------------- |
| `WantedBy=multi-user.target` | Enables the service to start at boot under the standard multi-user target. |

## [Service] 

| Line                                                   | Purpose                                                                                                                                                 |
| ------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Type=notify`                                          | Service will notify systemd when ready.                                                                                                                 |
| `EnvironmentFile=-/etc/default/%N`                     | Loads optional environment variables.                                                                                                                   |
| `EnvironmentFile=-/etc/sysconfig/%N`                   | Loads additional optional environment variables.                                                                                                        |
| `EnvironmentFile=-/etc/systemd/system/k3s.service.env --` | Loads custom environment variables for k3s.                                                                                                             |
| `KillMode=process`                                     | Only kills the main process on stop/restart.                                                                                                            |
| `Delegate=yes`                                         | Allows k3s to manage its own cgroups.                                                                                                                   |
| `LimitNOFILE=1048576`                                  | Increases open file descriptor limit.                                                                                                                   |
| `LimitNPROC=infinity`                                  | Removes process number limit.                                                                                                                           |
| `LimitCORE=infinity`                                   | Removes core dump size limit.                                                                                                                           |
| `TasksMax=infinity`                                    | Removes limit on tasks/threads.                                                                                                                         |
| `TimeoutStartSec=0`                                    | Disables startup timeout.                                                                                                                               |
| `Restart=always`                                       | Automatically restarts on failure.                                                                                                                      |
| `RestartSec=5s`                                        | Wait 5 seconds before restarting.                                                                                                                       |
| `ExecStartPre=-/sbin/modprobe br_netfilter`            | Loads the br_netfilter kernel module.                                                                                                                   |
| `ExecStartPre=-/sbin/modprobe overlay`                 | Loads the overlay filesystem module.                                                                                                                    |
| `ExecStart=/usr/local/bin/k3s server ...`              | Starts k3s with dynamically injected variables like `k3s_server_location`, `cluster-init`, server join flags, TLS SANs, and any extra server arguments. |
