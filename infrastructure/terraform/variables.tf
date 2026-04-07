variable "hcloud_token" {
  description = "Hetzner Cloud API token"
  type        = string
  sensitive   = true
}

variable "customer_name" {
  description = "Unique customer identifier (lowercase alphanumeric and hyphens)"
  type        = string

  validation {
    condition     = can(regex("^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$", var.customer_name))
    error_message = "Customer name must be 3-32 characters, lowercase alphanumeric and hyphens, cannot start or end with a hyphen."
  }
}

variable "tier" {
  description = "Deployment tier determining server resources"
  type        = string
  default     = "starter"

  validation {
    condition     = contains(["starter", "professional", "institution"], var.tier)
    error_message = "Tier must be one of: starter, professional, institution."
  }
}

variable "region" {
  description = "Hetzner Cloud datacenter location"
  type        = string
  default     = "fsn1"

  validation {
    condition     = contains(["fsn1", "nbg1", "hel1", "ash", "hil"], var.region)
    error_message = "Region must be a valid Hetzner location: fsn1, nbg1, hel1, ash, hil."
  }
}

variable "ssh_public_key" {
  description = "SSH public key for server access"
  type        = string
}

variable "ssh_allowed_ips" {
  description = "List of CIDR blocks allowed to SSH into the server"
  type        = list(string)
  default     = []
}
