# Troubleshooting

When a deploy fails, the failure is almost always one of: a Proxmox-side permission/storage issue, a CDN that blocks the cloud-image fetch, or a manual edit that went out of sync. This guide collects the cases real operators have hit and the verified fixes for each.

If you've hit something not on this list, please open an issue with the **Bug report** template and include the deploy log (`docker logs deploy-api-1`) plus the streamed deploy output.

---

## Table of contents

1. [Proxmox API token: required permissions](#proxmox-api-token-required-permissions)
2. [Snippets storage: enable the `snippets` content type](#snippets-storage-enable-the-snippets-content-type)
3. [Cloud image download fails with 403](#cloud-image-download-fails-with-403)
4. [Storage doesn't exist (`local-lvm`, etc.)](#storage-doesnt-exist-local-lvm-etc)
5. [Working directly with the live cluster workspace](#working-directly-with-the-live-cluster-workspace)
6. [Lock-file mismatch after a manual edit](#lock-file-mismatch-after-a-manual-edit)

---

## Proxmox API token: required permissions

Bandolier uses a single Proxmox API token (`<user>@pam!<token-name>`) for everything: cloning the cloud image, uploading cloud-init snippets, creating VMs, and reading status. The token needs **`PVEDatastoreAdmin`** on every storage Bandolier touches, plus **`PVEVMAdmin`** on the target node.

### Symptoms

```
Unable to list files from datastore <name> on node <node>:
received an HTTP 403 response - Reason: Permission check failed
(/storage/<name>, Datastore.Audit|Datastore.AllocateSpace)
```

### Fix

In the Proxmox web UI: **Datacenter → Permissions → Add → API Token Permission**:

| Path | Token | Role | Propagate |
|---|---|---|---|
| `/storage/<your-image-storage>` | `<user>@pam!<token-name>` | `PVEDatastoreAdmin` | ✅ |
| `/storage/<your-snippets-storage>` | same | `PVEDatastoreAdmin` | ✅ |
| `/storage/<your-vm-disk-storage>` | same | `PVEDatastoreAdmin` | ✅ |
| `/nodes/<your-node>` | same | `PVEVMAdmin` | ✅ |

If image, snippets, and VM disks all live on the same storage, that's one row instead of three. If they're on three different storages (Ceph + local + cephfs is common), it's three rows.

You only need to do this once per token — Bandolier reuses the same token for the lifetime of the cluster.

---

## Snippets storage: enable the `snippets` content type

The cloud-init configs Bandolier generates (one per VM) get uploaded as `snippets`-type files. By default Proxmox's `local` storage doesn't have `snippets` enabled — you'll see this on a fresh Proxmox install or any storage that wasn't manually flipped.

### Symptoms

```
the datastore "<name>" does not support content type "snippets";
supported content types are: [backup import iso vztmpl]
```

### Fix (option A — enable on the existing storage)

```bash
ssh root@<proxmox-host> "pvesm set <storage> --content backup,iso,vztmpl,snippets"
```

The `--content` flag is a **complete list** that replaces what's currently set, so you have to re-list everything that was already enabled. The example above keeps the typical defaults plus snippets; check `pvesm status -storage <storage>` first if your setup is non-standard.

### Fix (option B — point Bandolier at a different storage)

If you already have a storage with `snippets` enabled (e.g. `cephfs`), use the **Snippets storage** field on the initialize wizard's Proxmox step. Defaults to `local` to match the most common setup.

---

## Cloud image download fails with 403

The Rocky Linux CDN periodically rate-limits or blocks `HEAD` requests from non-browser User-Agents. The bpg/proxmox provider sends a HEAD as part of `proxmox_virtual_environment_download_file`'s preflight, and Proxmox itself sends a HEAD before the actual download. Either can hit a 403.

### Symptoms

```
Could not get file metadata: error retrieving URL metadata for
"https://dl.rockylinux.org/pub/rocky/9/images/x86_64/Rocky-9-GenericCloud.latest.x86_64.qcow2":
received an HTTP 403 response - Reason: Permission check failed
```

### Fix (built-in: mirror fallback)

Bandolier's catalog ships **three preference-ordered Rocky 9 mirrors**: `dl.rockylinux.org`, `download.rockylinux.org`, and `mirror.rackspace.com/rockylinux`. On deploy, the api HEAD-probes each in order and hands terraform the first 2xx URL. If your primary CDN is having a bad day, the deploy silently uses one of the alternates.

If all three of your candidate mirrors HEAD-block (rare but possible during incidents), use the workaround below.

### Workaround (manual pre-upload)

Download the image once to your laptop, upload it to the operator host, then `scp` it onto Proxmox's image storage:

```bash
# On your machine
curl -fOL https://download.rockylinux.org/pub/rocky/9/images/x86_64/Rocky-9-GenericCloud.latest.x86_64.qcow2

# Verify (optional)
curl -fOL https://download.rockylinux.org/pub/rocky/9/images/x86_64/CHECKSUM
sha256sum -c --ignore-missing CHECKSUM

# Push to Proxmox; the filename pattern Bandolier expects is in
# api/internal/profiles/homelab/distros.go (e.g. rocky9.img).
scp Rocky-9-GenericCloud.latest.x86_64.qcow2 root@<proxmox-host>:/var/lib/vz/template/iso/rocky9.img
```

After that, redeploy. Terraform will see the file already exists at the expected `file_name` and skip the download.

> Future direction: a `proxmox_image_pre_uploaded: true` flag that switches Bandolier to a `data "proxmox_virtual_environment_file"` source instead of `proxmox_virtual_environment_download_file`. Tracked in the issue tracker.

---

## Storage doesn't exist (`local-lvm`, etc.)

Through v0.1.2, Bandolier's terraform module hardcoded `local-lvm` for VM disks regardless of what the operator put in the initialize wizard's "Storage" field. The form input was silently dropped. **Fixed in v0.1.3** — the wizard's value is now actually used.

### Symptoms (v0.1.2 and earlier)

```
unable to create VM <id> - storage 'local-lvm' does not exist
```

### Fix

Upgrade to v0.1.3 or later:

```bash
docker pull ghcr.io/lazerdude-labs/bandolier/api:0.1
cd deploy && docker compose up -d
```

Then redeploy. The wizard's **Storage** field (e.g. `vm_data` for Ceph RBD setups) is now respected for both the VM disks and the cloud-init drive.

If you're stuck on an older version and can't upgrade, see the next section for how to patch the live workspace by hand.

---

## Working directly with the live cluster workspace

For the rare case you need to patch terraform mid-deploy (security-sensitive bug fix, infrastructure quirk that needs a one-off hack), there are **three relevant paths**:

| Path | Role | Edit when |
|---|---|---|
| `~/bandolier/terraform/` (host source) | Source of truth on disk; copied into container at build | Permanent fix for future clusters |
| `/opt/bandolier/terraform/` (in api container, read-only) | What gets copied into a new cluster's workspace at create time | Never — read-only mount, pointless to edit |
| `/var/lib/bandolier/tf-state/clusters/<id>/` (in api container) | Live cluster workspace; what `terraform apply` actually runs against | One-off fix to an existing cluster |

For a permanent fix:

```bash
# Edit on the host
$EDITOR ~/bandolier/terraform/main.tf
# Rebuild the api image so /opt/bandolier/terraform/ is updated
cd deploy && docker compose up -d --build api
# New clusters will get the fix; existing live workspaces are not retroactively patched
```

For a one-off fix to a running cluster:

```bash
CLUSTER=<cluster-id-from-bandolier>
docker exec deploy-api-1 sh -c "sed -i 's/old/new/g' /var/lib/bandolier/tf-state/clusters/$CLUSTER/main.tf"
docker exec deploy-api-1 sh -c "cd /var/lib/bandolier/tf-state/clusters/$CLUSTER && /usr/local/bin/terraform init -upgrade && /usr/local/bin/terraform apply -auto-approve"
```

The `init -upgrade` is required because manual edits invalidate the dependency lock file — see the next section.

---

## Lock-file mismatch after a manual edit

Manual edits to terraform files inside a live cluster workspace invalidate the `.terraform.lock.hcl` checksum. The next `apply` refuses to run.

### Symptoms

```
the cached package for registry.terraform.io/bpg/proxmox X.Y.Z does not match
any of the checksums recorded in the dependency lock file
```

### Fix

Always run `terraform init -upgrade` after manual edits to a live cluster's terraform:

```bash
docker exec deploy-api-1 sh -c "cd /var/lib/bandolier/tf-state/clusters/$CLUSTER && /usr/local/bin/terraform init -upgrade"
```

You only need this when you've directly edited files in `/var/lib/bandolier/tf-state/clusters/<id>/`. Routine deploys don't trigger it because terraform handles its own lock file when files are unchanged.

---

## Useful commands

For quick iteration during troubleshooting, with `CLUSTER` set to your cluster's id:

```bash
export CLUSTER=<cluster-id>

# Run terraform manually inside the api container
docker exec deploy-api-1 sh -c "cd /var/lib/bandolier/tf-state/clusters/$CLUSTER && /usr/local/bin/terraform apply -auto-approve 2>&1 | tail -50"

# Find every reference to a string in the live workspace
docker exec deploy-api-1 sh -c "grep -rn 'string' /var/lib/bandolier/tf-state/clusters/$CLUSTER/"

# Replace across the live workspace
docker exec deploy-api-1 sh -c "grep -rl 'old' /var/lib/bandolier/tf-state/clusters/$CLUSTER/ | xargs sed -i 's/old/new/g'"

# Destroy a cluster from outside the UI
docker exec deploy-api-1 sh -c "cd /var/lib/bandolier/tf-state/clusters/$CLUSTER && /usr/local/bin/terraform destroy -auto-approve"

# Confirm snippets land on Proxmox
ssh root@<proxmox-host> "ls -la /var/lib/vz/snippets/"

# Enable snippets content type on a Proxmox storage
ssh root@<proxmox-host> "pvesm set <storage> --content backup,iso,vztmpl,snippets"
```

The api container's working state lives at `/var/lib/bandolier/` (mounted from the `app-data` and `tf-state` named volumes). The host source lives wherever you cloned the repo. The container's read-only copy lives at `/opt/bandolier/`.

---

## Filing a useful bug report

If you hit something this guide doesn't cover:

1. Open an issue using the [Bug report template](https://github.com/lazerdude-labs/bandolier/issues/new?template=bug_report.yml).
2. Include the **api container log**: `docker logs deploy-api-1 2>&1 | tail -200`.
3. Include the **deploy stream**: scroll back in the UI's deploy view and copy the trailing 50 lines, OR run `cat /var/lib/bandolier/logs/<deployment-id>.log` inside the container.
4. Include the **Bandolier version**: image tag from Settings, or `git describe` if you build from source.
5. Include the **Proxmox version** and a sketch of your storage layout (`pvesm status` output is gold).

Do **not** include your token secret, vault unseal keys, or master password. The bug report template warns about this; if you accidentally include one, file a [security advisory](https://github.com/lazerdude-labs/bandolier/security/advisories/new) and rotate the credential.
