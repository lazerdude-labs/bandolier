import { z } from 'zod'

export const initializeSchema = z.object({
  proxmox: z.object({
    endpoint: z.string().url(),
    token_id: z.string().min(1),
    token_secret: z.string().min(1),
    node: z.string().min(1),
    storage: z.string().min(1),
    username: z.string().min(1),
    password: z.string().min(1),
    ca_bundle: z.string().optional().default(''),
    image_storage: z.string().min(1).default('local'),
    distro: z.string().optional().default(''),
    custom_url: z.string().optional().default(''),
    custom_sha256: z.string().optional().default(''),
  }).refine(
    (p) => {
      const hasDistro = !!p.distro;
      const hasCustom = !!p.custom_url;
      if (hasDistro === hasCustom) return false; // both or neither
      if (hasCustom && !p.custom_sha256) return false;
      if (hasCustom && p.custom_sha256 && !/^[a-fA-F0-9]{64}$/.test(p.custom_sha256)) return false;
      return true;
    },
    { message: 'Pick a distro OR provide a custom URL + 64-char hex sha256' },
  ),
  network: z.object({
    cidr: z.string().min(1),
    gateway: z.string().min(1),
    dns: z.array(z.string()).min(1),
    fqdn: z.string().min(1),
    master_ip: z.string().min(1),
    agent1_ip: z.string().min(1),
    agent2_ip: z.string().min(1),
    vlan: z.number().int().min(1).max(4094),
    bridge_name: z.string().min(1),
    traefik_dashboard: z.boolean().default(true),
    manage_dns: z.boolean().default(true),
    dns_server: z.string().optional().default(''),
    dns_zone: z.string().optional().default(''),
    tsig_name: z.string().optional().default(''),
    tsig_secret: z.string().optional().default(''),
  }),
  ssh: z.object({
    public_key:  z.string().optional().default(''),
    private_key: z.string().optional().default(''),
  }).refine(
    (s) => {
      const hasPub  = !!s.public_key;
      const hasPriv = !!s.private_key;
      if (hasPub !== hasPriv) return false;
      if (hasPub && !/^ssh-(rsa|ed25519|ecdsa)\s\S+(\s.*)?$/.test(s.public_key)) return false;
      if (hasPriv && !s.private_key.includes('-----BEGIN')) return false;
      return true;
    },
    { message: 'Both public + private key required (or both blank for auto-gen)' }
  ),
})

export type InitializeInput = z.infer<typeof initializeSchema>
