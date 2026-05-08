# 📖 K3s Prerequisite Tasks Overview

This Ansible task file prepares the host nodes for **k3s installation** by ensuring system prerequisites, networking, firewall rules, and necessary packages are in place. It is run **before installing k3s** on server or agent nodes.

---

## Key Steps and Purpose

### 1️⃣ Ansible Version Check

* Ensures the minimum required Ansible version is **2.15** to support the playbook.

### 2️⃣ Package Dependencies

* Installs required packages based on the OS:

  * Ubuntu: `policycoreutils` (SELinux context restoration)
  * RHEL 10: `kernel-modules-extra` (for `br_netfilter` module)

### 3️⃣ Network Configuration

* Enables **IPv4 and IPv6 forwarding**.
* Loads `br_netfilter` and sets `bridge-nf-call-iptables`/`ip6tables` for RedHat/Archlinux.

### 4️⃣ Firewall Configuration

* Configures **UFW** or **Firewalld** to open required ports:

  * API server port (`{{ api_port }}`)
  * Etcd ports for HA servers
  * Flannel VXLAN & Wireguard ports
  * Kubelet metrics port
  * Cluster and service CIDRs

### 5️⃣ AppArmor & Security

* Checks for AppArmor and installs parser packages where required (SUSE/Debian).
* Warns if iptables version has known K3s issues (v1.8.0–1.8.4).
* Ensures `/usr/local/bin` is in `sudo` secure_path on RedHat.

### 6️⃣ Optional Directory Setup

* Sets up **alternative k3s server directory** if customized.
* Creates directories for **extra manifests** and copies them to `/var/lib/rancher/k3s/server/manifests`.
* Creates optional **private registry config** in `/etc/rancher/k3s/registries.yaml`.

### 7️⃣ Miscellaneous

* Populates Ansible service facts for later conditional tasks.
* Handles modern nftables/iptables on Arch Linux ARM 6.18+.

---

## Notes

* Idempotent: running multiple times will not cause side effects.
* Conditional logic ensures compatibility across **Ubuntu, RHEL, SUSE, Arch Linux**.
* Prepares the host for seamless **k3s server and agent installation**.
* Handles both HA and single-node deployments.
* Includes security and firewall best practices for cluster networking.
