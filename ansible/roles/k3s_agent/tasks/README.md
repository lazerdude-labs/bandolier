# 📖 K3s Agent Installation Playbook Overview

This Ansible playbook handles the **installation and configuration of k3s agents** on worker nodes. It ensures that agents are installed only when needed, properly configured, and able to join the cluster securely.

---

## Key Steps and Logic

1. **Check Installed Version**

   * Runs `k3s --version` to detect if the agent is already installed.
   * Sets `k3s_agent_installed_version` if k3s is present.
   * Allows conditional download and install only if missing or outdated.

2. **Download and Install k3s Agent**

   * Downloads the k3s install script if needed.
   * Installs the agent binary with environment variables:

     * `INSTALL_K3S_SKIP_START` prevents immediate start during install.
     * `INSTALL_K3S_SYSTEMD_DIR` sets the systemd service path.
     * `INSTALL_K3S_VERSION` specifies the desired version.
     * `INSTALL_K3S_EXEC` passes agent-specific startup arguments.
   * Merges any extra environment variables defined in `extra_install_envs`.

3. **Optional Configuration File**

   * Creates `/etc/rancher/k3s/config.yaml` if `agent_config_yaml` is defined.
   * Ensures the configuration directory exists and sets correct permissions.

4. **Cluster Join Token**

   * Retrieves the cluster token from the first server node.
   * Ensures the token is correctly set in the agent service environment file.

5. **Service Environment Setup**

   * Determines the correct environment file path based on the init system (`systemd` or other).
   * Adds extra environment variables if defined.
   * Removes any stale token entries and adds the current token securely (`no_log: true`).

6. **Enable and Start k3s Agent**

   * Starts or restarts the `k3s-agent` service depending on configuration changes.
   * Ensures the service is enabled for automatic start on boot.

---

## Notes

* Idempotent: The playbook only downloads or installs the agent when necessary.
* Supports **airgapped environments** by skipping downloads if artifacts are already present.
* Secure handling of tokens: `no_log: true` prevents logging sensitive information.
* Integrates with the control-plane server for seamless cluster joining.
* Uses Ansible facts and conditionals to ensure safe and repeatable operations across multiple agent nodes.
