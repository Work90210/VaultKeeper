terraform {
  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "~> 1.45"
    }
  }
  required_version = ">= 1.5.0"
}

provider "hcloud" {
  token = var.hcloud_token
}

locals {
  server_types = {
    starter      = "cpx31"
    professional = "cpx41"
    institution  = "ax42"
  }
}

resource "hcloud_ssh_key" "deploy" {
  name       = "vaultkeeper-${var.customer_name}"
  public_key = var.ssh_public_key
}

resource "hcloud_firewall" "vaultkeeper" {
  name = "vaultkeeper-${var.customer_name}"

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "443"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "22"
    source_ips = var.ssh_allowed_ips
  }
}

resource "hcloud_server" "vaultkeeper" {
  name         = "vk-${var.customer_name}"
  server_type  = local.server_types[var.tier]
  location     = var.region
  image        = "ubuntu-22.04"
  ssh_keys     = [hcloud_ssh_key.deploy.id]
  firewall_ids = [hcloud_firewall.vaultkeeper.id]

  labels = {
    service  = "vaultkeeper"
    customer = var.customer_name
    tier     = var.tier
  }
}
