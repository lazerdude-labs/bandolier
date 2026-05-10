# Proxmox setup for Bandolier

Bandolier expects a small set of Proxmox-side prerequisites: an API token with the right permissions, three storages (or one storage with three content types enabled), a network bridge with VLAN tagging, and outbound internet on the host. This guide walks through each piece for a fresh Proxmox VE 8.x install.

If you already have a working Proxmox setup, skip to the [verification checklist](#verification-checklist) at the end and confirm everything matches.

---

## Table of contents

1. [Prerequisites](#prerequisites)
2. [Create the API token](#create-the-api-token)
3. [Grant token permissions](#grant-token-permissions)
4. [Storage: what Bandolier needs and where](#storage-what-bandolier-needs-and-where)
5. [Network: bridge and VLAN](#network-bridge-and-vlan)
6. [Cloud image: catalog vs. pre-upload](#cloud-image-catalog-vs-pre-upload)
7. [SSH into Proxmox from the api container](#ssh-into-proxmox-from-the-api-container)
8. [Verification checklist](#verification-checklist)
9. [Filling in the initialize wizard](#filling-in-the-initialize-wizard)

---

## Prerequisites

- **Proxmox VE 8.x or newer** on a single node. Multi-node clusters are supported in principle (Bandolier targets one node per cluster), but this guide covers the single-node case.
- **Network reachability** between the host running Bandolier (your laptop, a homelab utility VM, etc.) and the Proxmox API endpoint, typically `https://<proxmox-host>:8006`. The api container makes the API calls; if your laptop can `curl https://<host>:8006/api2/json/version -k`, the api container can too.
- **A separate VLAN or subnet** for the k3s cluster. Bandolier provisions three VMs (1 server + 2 agents) and assigns them static IPs from a CIDR you provide. Putting them on a dedicated VLAN keeps the cluster's traffic segmented from your management network — recommended but not required.
- **Outbound internet from Proxmox** during the first deploy, so Proxmox can fetch the Rocky Linux cloud image. If your Proxmox is air-gapped, skip ahead to [pre-upload](#cloud-image-catalog-vs-pre-upload).

---

## Create the API token

You can create the token in the web UI or via SSH. Both produce the same result.

### Web UI

1. **Datacenter → Permissions → API Tokens → Add**.
2. **User**: `root@pam` (or any other realm/user that exists). Most operators use `root@pam` for a homelab.
3. **Token ID**: `bandolier-api`. The full token name will become `root@pam!bandolier-api`.
4. **Privilege Separation**: ☐ **unchecked**. (When unchecked, the token inherits the user's privileges. When checked, you have to grant privileges to the token explicitly — which is what we'll do anyway in the next section, so unchecked is fine here.)
5. Click **Add**. Copy the token secret that's displayed — **you won't see it again**. It looks like a UUID.

### SSH

```bash
ssh root@<proxmox-host>
pveum user token add root@pam bandolier-api --privsep 0
```

The output includes the secret on the line `value: <secret>`. Copy it.

### What you have now

- **Token ID**: `root@pam!bandolier-api`
- **Token secret**: a UUID-ish string

You'll paste both into the Bandolier initialize wizard's Proxmox step. The secret is stored encrypted in Bandolier's Vault — never on disk in plaintext.

---

## Grant token permissions

Bandolier needs `PVEDatastoreAdmin` on every storage it touches and `PVEVMAdmin` on the target node. The minimum set:

| Path | Role | Why |
|---|---|---|
| `/storage/<image-storage>` | `PVEDatastoreAdmin` | Read/write the cloud image (qcow2) into iso-content storage. |
| `/storage/<snippets-storage>` | `PVEDatastoreAdmin` | Upload per-VM cloud-init configs. |
| `/storage/<vm-disk-storage>` | `PVEDatastoreAdmin` | Allocate VM disks and the cloud-init drive. |
| `/nodes/<node-name>` | `PVEVMAdmin` | Create, configure, start, stop, and destroy VMs on the node. |

If image / snippets / VM-disks all live on the same storage (typical for `local`), that's one row. If they're on three different storages (typical for Ceph + cephfs + local), it's three rows.

### Web UI

**Datacenter → Permissions → Add → API Token Permission** for each row in the table above. **Propagate** must be checked on every row so child paths inherit the role.

### SSH

```bash
ssh root@<proxmox-host>
TOKEN='root@pam!bandolier-api'

# Replace these names with whatever your storages are called
IMAGE_STORAGE=local
SNIPPETS_STORAGE=local
VM_DISK_STORAGE=local-lvm
NODE=pve

pveum acl modify "/storage/$IMAGE_STORAGE"    --tokens "$TOKEN" --roles PVEDatastoreAdmin --propagate 1
pveum acl modify "/storage/$SNIPPETS_STORAGE" --tokens "$TOKEN" --roles PVEDatastoreAdmin --propagate 1
pveum acl modify "/storage/$VM_DISK_STORAGE"  --tokens "$TOKEN" --roles PVEDatastoreAdmin --propagate 1
pveum acl modify "/nodes/$NODE"               --tokens "$TOKEN" --roles PVEVMAdmin --propagate 1
```

If a storage name is repeated (e.g. all three are `local`), running the command three times is fine — it's idempotent.

---

## Storage: what Bandolier needs and where

Bandolier uses **three logical storages**, which can map to one, two, or three actual Proxmox storages depending on your setup. The key thing is each one needs the right *content types* enabled.

| Logical use | Required content type | Common Proxmox storage |
|---|---|---|
| **Image** (the qcow2 cloud image) | `iso` (or `import`) | `local` |
| **Snippets** (per-VM cloud-init configs) | `snippets` | `local` (after enabling) |
| **VM disks + cloud-init drive** | `images` | `local-lvm`, `vm_data` (Ceph RBD), etc. |

### Enabling the `snippets` content type

A fresh `local` storage doesn't have `snippets` enabled. Bandolier will fail to upload its cloud-init configs if it isn't. Enable it via SSH:

```bash
ssh root@<proxmox-host>
# IMPORTANT: --content takes a complete list. Check the current set first.
pvesm status -storage local
# Then add 'snippets' to whatever's there. Defaults look like this:
pvesm set local --content backup,iso,vztmpl,snippets
```

If you already have a storage with `snippets` enabled (often `cephfs`), you can use that as your snippets storage instead by entering its name in the **Snippets storage** field on the initialize wizard's Proxmox step. Defaults to `local`.

### Verifying

```bash
ssh root@<proxmox-host> "pvesm status -content snippets"
# Should list at least one storage. If empty, snippets aren't enabled anywhere.
```

### Recommended layouts

- **Single-storage minimum** (small homelab, everything on one disk): one `local` storage with all three content types enabled (`iso`, `snippets`, `images`) and one Datastore permission.
- **Typical homelab** (separate ISO/template area + LVM-thin for VMs): `local` (with `iso` + `snippets`) for templates and configs, `local-lvm` for VM disks. Two Datastore permissions.
- **Ceph cluster** (RBD-backed): `local` (with `iso` + `snippets`) for templates and configs, an RBD pool (e.g. `vm_data`) for VM disks. Two Datastore permissions.
- **Ceph + cephfs** (separation of templates and VMs across the Ceph cluster): `cephfs` (with `iso` + `snippets`) for templates and configs, RBD pool for VM disks. Two Datastore permissions, both on Ceph paths.

---

## Network: bridge and VLAN

Bandolier provisions three VMs that need network access to each other and to the gateway. The wizard asks for:

| Field | Example | What it is |
|---|---|---|
| **CIDR** | `192.0.2.0/24` | The subnet the cluster lives on. Used to derive the prefix length for cloud-init's `ipconfig0`. |
| **Bridge** | `vmbr1` | A Proxmox network bridge configured on the host. |
| **VLAN tag** | `30` | The VLAN ID for cluster traffic. Use `0` if your bridge doesn't use VLAN tagging. |
| **Gateway** | `192.0.2.1` | The default gateway for the cluster's subnet. |
| **DNS** | `1.1.1.1, 192.0.2.5` | Comma-separated. Used by the VMs at boot via cloud-init. |
| **Master IP** | `192.0.2.21` | Static IP for the k3s server (master). |
| **Agent 1 IP** | `192.0.2.22` | Static IP for the first agent. |
| **Agent 2 IP** | `192.0.2.23` | Static IP for the second agent. |
| **FQDN** | `k3s.example.com` | Base domain. Per-VM hostnames become `master1.<fqdn>`, `worker1.<fqdn>`, etc. |

### Setting up a VLAN-aware bridge on Proxmox

If your bridge isn't already VLAN-aware, edit `/etc/network/interfaces` on the Proxmox host:

```ini
auto vmbr1
iface vmbr1 inet manual
    bridge-ports <physical-or-bond>
    bridge-stp off
    bridge-fd 0
    bridge-vlan-aware yes
    bridge-vids 2-4094
```

Then `ifreload -a` (or reboot). Confirm with `bridge link show` and `cat /sys/class/net/vmbr1/bridge/vlan_filtering` (should be `1`).

### If your network has no VLANs

Set the **VLAN tag** field to `0`. Proxmox treats `0` as "no tag" and the VMs will share the bridge's untagged network.

---

## Cloud image: catalog vs. pre-upload

Bandolier ships a small built-in catalog of cloud images. As of v0.1.3 it includes Rocky 9 with three preference-ordered mirrors. On deploy, Bandolier HEAD-probes each mirror in order and hands the first 2xx URL to terraform; Proxmox then downloads it and verifies the SHA256.

### When the catalog flow works

If your Proxmox host has outbound HTTPS to `dl.rockylinux.org` (or one of the alternate mirrors), pick **Rocky 9** in the wizard and you're done. Bandolier handles the rest.

### When you need to pre-upload

If your network blocks outbound, all upstream mirrors HEAD-block your User-Agent, or you want a deterministic offline-friendly setup, pre-upload the image once:

```bash
# On a machine with internet
curl -fOL https://download.rockylinux.org/pub/rocky/9/images/x86_64/Rocky-9-GenericCloud.latest.x86_64.qcow2

# Optional: verify
curl -fOL https://download.rockylinux.org/pub/rocky/9/images/x86_64/CHECKSUM
sha256sum -c --ignore-missing CHECKSUM

# Upload to Proxmox. The filename Bandolier expects is in
# api/internal/profiles/homelab/distros.go (e.g. rocky9.img).
scp Rocky-9-GenericCloud.latest.x86_64.qcow2 root@<proxmox-host>:/var/lib/vz/template/iso/rocky9.img
```

Once the file is in place at the expected location, the next deploy will see it exists and skip the download. (Bandolier's terraform passes `overwrite = false` to `proxmox_virtual_environment_download_file`.)

### Custom URL

If you have your own internal mirror or a custom image, use the **Custom URL** field on the wizard. You'll also need to provide the SHA256 (64 hex chars) so Proxmox can verify what it downloads.

---

## SSH into Proxmox from the api container

Bandolier's api container does **not** SSH into Proxmox during normal deploys — Terraform talks to the Proxmox API over HTTPS. The only thing that uses SSH is the post-deploy Ansible playbook against the freshly-provisioned k3s VMs (not Proxmox itself).

That said, if you're debugging and want to run `pvesm` / `qm` commands from the api container, you'll need ssh client + a key:

```bash
docker exec -it deploy-api-1 sh
# OpenSSH client is installed in the api image
# Drop your private key into a tmpfile and ssh as root
```

For the routine deploy flow you don't need this — it's purely a debugging convenience.

---

## Verification checklist

Before clicking **Deploy** in the Bandolier UI, confirm each of these from your laptop or the Bandolier host:

```bash
# 1. Token works. Read the secret without echoing it to the terminal or
#    leaving it in shell history; unset when done so it doesn't sit in
#    the environment for the rest of the session.
TOKEN_ID='root@pam!bandolier-api'
read -rs TOKEN_SECRET; export TOKEN_SECRET; echo
curl -k -H "Authorization: PVEAPIToken=$TOKEN_ID=$TOKEN_SECRET" \
  https://<proxmox-host>:8006/api2/json/version
# Expect: {"data":{"version":"8.x.y", ...}}

# 2. Token can list each storage
for s in <image-storage> <snippets-storage> <vm-disk-storage>; do
  echo "--- $s ---"
  curl -k -H "Authorization: PVEAPIToken=$TOKEN_ID=$TOKEN_SECRET" \
    "https://<proxmox-host>:8006/api2/json/nodes/<node>/storage/$s/content"
done
# Expect: each returns {"data":[...]}, never 403.

# When done with the verification commands:
unset TOKEN_SECRET

# 3. Snippets content type is enabled on the snippets storage
ssh root@<proxmox-host> "pvesm status -content snippets"
# Expect: at least one row, including <snippets-storage>.

# 4. Network reachability from Proxmox to the cloud image mirror
ssh root@<proxmox-host> "curl -fsSI https://dl.rockylinux.org/pub/rocky/9/images/x86_64/ | head -1"
# Expect: HTTP/2 200 (or 301/302). 403 → use one of the alternate mirrors or pre-upload.

# 5. VLAN-aware bridge (only if you're using VLAN tagging)
ssh root@<proxmox-host> "cat /sys/class/net/<bridge>/bridge/vlan_filtering"
# Expect: 1
```

If all five pass, the initialize wizard should have everything it needs.

---

## Filling in the initialize wizard

Map the values you've gathered to the wizard fields:

### Proxmox step

| Wizard field | Value |
|---|---|
| **Endpoint** | `https://<proxmox-host>:8006` |
| **Node** | `<your node name, e.g. pve>` |
| **Storage** | `<vm-disk-storage>` (e.g. `local-lvm`, `vm_data`) |
| **API token id** | `root@pam!bandolier-api` |
| **API token secret** | the secret from step 1 |
| **SSH username** | `root` (used by the bpg/proxmox provider's ssh block; Bandolier's api doesn't use it directly) |
| **SSH password** | the SSH user's password — see note below on key-based auth |
| **Image storage** | `<image-storage>` (default `local`) |
| **Snippets storage** | `<snippets-storage>` (default `local`) |
| **Image source** | Rocky 9, or **Custom URL** + SHA256 |

### Network step

Use the values from the [Network section](#network-bridge-and-vlan).

### SSH step

Optional. Leave both fields blank to have Bandolier auto-generate a keypair. Or paste a public + private key pair (both, or neither) if you want to bring your own.

> **Note on Proxmox-side SSH:** the wizard's "SSH password" field on the **Proxmox** step is what the bpg/proxmox provider uses to authenticate to your Proxmox host (separate from the Vault-stored API token, separate from the cluster VMs' SSH keys above). For now Bandolier's wizard only accepts a password there; if you'd rather use SSH key auth to Proxmox, the workaround is to manually edit `terraform/modules/vm/main.tf` (or the live cluster workspace) and add an `ssh_private_key` attribute to the provider block. Tracked as a v0.2+ enhancement to surface this in the wizard.

---

## What can still go wrong

After all of the above, the most common deploy failures are:

1. **403 on the cloud image fetch** — see [TROUBLESHOOTING.md → Cloud image download fails with 403](../TROUBLESHOOTING.md#cloud-image-download-fails-with-403).
2. **Snippets content type missing** — see [TROUBLESHOOTING.md → Snippets storage](../TROUBLESHOOTING.md#snippets-storage-enable-the-snippets-content-type).
3. **Token permissions missing on a storage** — see [TROUBLESHOOTING.md → Proxmox API token: required permissions](../TROUBLESHOOTING.md#proxmox-api-token-required-permissions).
4. **Storage name mismatch** — typo in the wizard's Storage / Image storage / Snippets storage fields. The Proxmox API will return a clear "storage 'X' does not exist" error in the deploy log.
5. **VLAN not tagged on the bridge** — VMs can't reach the gateway. Confirm the bridge's `bridge_vlan_aware` is `yes` and the VLAN range covers your tag.

For anything else, [open a bug report](https://github.com/lazerdude-labs/bandolier/issues/new?template=bug_report.yml) with the deploy log attached.
