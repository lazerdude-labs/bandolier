# 📖 K3s Installation & Setup Playbook Overview

This Ansible playbook handles the **installation and configuration of k3s** on server nodes. It is designed to:

* Detect if k3s is already installed and its version.
* Download and install k3s only if necessary.
* Configure TLS SANs for multi-node clusters.
* Deploy the k3s systemd service with custom environment variables.
* Initialize the first server and join additional servers.
* Configure `kubectl` for the user with proper context and autocomplete.

---

## Key Steps and Logic

1. **Check Installed Version**

   * Runs `k3s --version` to detect if k3s is present.
   * Sets `k3s_server_installed_version` if k3s is installed.
   * Allows conditional download and install only if needed (or if airgapped).

2. **Download and Install k3s**

   * Downloads `k3s` install script and binary if the version is missing or outdated.
   * Uses environment variables to control installation behavior.

3. **Optional Config and TLS SAN**

   * Creates `/etc/rancher/k3s/config.yaml` if `server_config_yaml` is defined.
   * Detects if `api_endpoint` needs to be added as a `--tls-san` argument.

4. **Systemd Service Deployment**

   * Copies `k3s.service.j2` template to `systemd_dir`.
   * Handles both single-server and HA setups.
   * Adds service environment variables and token management.
   * Enables and restarts the service as needed.

5. **Cluster Initialization & Joining Nodes**

   * First server handles cluster initialization.
   * Additional servers join the cluster using the token from the first server.
   * Verifies that all control-plane nodes have joined successfully.

6. **Kubectl Configuration for User**

   * Creates `~/.kube` directory and copies k3s kubeconfig.
   * Sets `KUBECONFIG` environment variable.
   * Adds bash completion for `kubectl` and `k3s`.

---

## Notes

* `changed_when: false` is used for version checks to avoid unnecessary reporting of changes.
* `ignore_errors: true` allows playbook to continue even if k3s is not installed yet.
* Tokens are handled securely (`no_log: true`) to prevent logging sensitive information.
* Airgapped environments can skip downloads, assuming binaries are already present.
* The playbook supports both **single server** and **multi-server (HA)** clusters.
* Uses Ansible facts and conditionals to ensure idempotent operations.
