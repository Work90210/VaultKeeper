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

variable "hcloud_token" {
  description = "Hetzner Cloud API token"
  type        = string
  sensitive   = true
}

variable "ssh_public_key" {
  description = "SSH public key for server access"
  type        = string
}

variable "ssh_allowed_ips" {
  description = "List of CIDR blocks allowed to SSH into the monitoring server"
  type        = list(string)
  default     = []
}

variable "monitoring_domain" {
  description = "Domain for the monitoring dashboard"
  type        = string
  default     = "status.vaultkeeper.eu"
}

resource "hcloud_ssh_key" "monitoring" {
  name       = "vaultkeeper-monitoring"
  public_key = var.ssh_public_key
}

resource "hcloud_firewall" "monitoring" {
  name = "vaultkeeper-monitoring"

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

resource "hcloud_server" "monitoring" {
  name         = "vk-monitoring"
  server_type  = "cx22"
  location     = "fsn1"
  image        = "ubuntu-22.04"
  ssh_keys     = [hcloud_ssh_key.monitoring.id]
  firewall_ids = [hcloud_firewall.monitoring.id]

  labels = {
    service = "vaultkeeper-monitoring"
  }
}

output "monitoring_ip" {
  description = "Public IPv4 address of the monitoring server"
  value       = hcloud_server.monitoring.ipv4_address
}

output "monitoring_id" {
  description = "Hetzner Cloud server ID for monitoring"
  value       = hcloud_server.monitoring.id
}
