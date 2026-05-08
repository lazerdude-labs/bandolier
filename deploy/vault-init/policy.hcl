path "bandolier/data/clusters/*" {
  capabilities = ["create", "read", "update", "delete"]
}

path "bandolier/metadata/clusters/*" {
  capabilities = ["read", "delete", "list"]
}

# Phase 5: WS signing key + future auth-related secrets persist under
# bandolier/data/auth/. Granted full CRUD because EnsureWSSigningKey
# generates + writes on first boot (idempotent on later boots).
path "bandolier/data/auth/*" {
  capabilities = ["create", "read", "update", "delete"]
}

path "bandolier/metadata/auth/*" {
  capabilities = ["read", "delete", "list"]
}

path "pki/issue/traefik" {
  capabilities = ["create", "update"]
}

path "auth/token/renew-self" {
  capabilities = ["update"]
}
