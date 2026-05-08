#cloud-config
# Users to create and configure
users:
  - name: root
    ssh_authorized_keys:
      - ${ssh_public_key}
  - name: ansible
    groups:
      - sudo
    shell: /bin/bash
    ssh_authorized_keys:
      - ${ssh_public_key}
    sudo: ALL=(ALL) NOPASSWD:ALL

# Commands to run on first boot
package_update: true
package_upgrade: true
package_reboot_if_required: true
hostname: "${name}"
preserve_hostname: false
fqdn: "${fqdn}"

packages:
  - qemu-guest-agent
  - epel-release
  - net-tools
  - nfs-utils
  - nginx

runcmd:

  - timedatectl set-timezone Europe/Berlin
  - systemctl enable qemu-guest-agent
  - systemctl start qemu-guest-agent
  - sysctl -w fs.inotify.max_queued_events=2099999999
  - sysctl -w fs.inotify.max_user_instances=2099999999
  - sysctl -w fs.inotify.max_user_watches=2099999999

#The above sysctl lines just raise kernel limits for file system event monitoring (inotify).
#These are adjusted for kubernetes and docker as they have multiple file system events when running and can cause errors if not increased.
#If your applications never use that many watches or events, the limits are simply unused.
#There’s no performance penalty for having higher limits set — they don’t consume memory unless processes actually create watches/events.
