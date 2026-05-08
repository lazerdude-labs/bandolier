terraform {
  required_providers {
    proxmox = { # Pulls in proxmox bpg provider
      source  = "bpg/proxmox"
      version = "0.90.0"
    }
    local = {
      source  = "hashicorp/local"
      version = "~> 2.0"
    }
    time = {
      source  = "hashicorp/time"
      version = "~> 0.9"
    }
  }
}

provider "proxmox" {
  endpoint  = local.proxmox_config.endpoint
  api_token = local.proxmox_config.api_token_id
  insecure  = true

  ssh {
    # agent = false → use the password explicitly. The api container has no
    # ssh-agent loaded; with agent=true the provider would attempt agent auth,
    # fail, and never fall back to the password.
    agent    = false
    username = local.proxmox_config.username
    password = local.proxmox_config.password
  }
}
