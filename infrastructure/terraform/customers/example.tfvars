# Example customer configuration
# Copy this file and customize for each deployment
#
# Usage: terraform apply -var-file=customers/my-customer.tfvars

customer_name = "example-org"
tier          = "starter"
region        = "fsn1"

# SSH public key for deployment access
ssh_public_key = "ssh-ed25519 AAAA... deploy@vaultkeeper"

# Restrict SSH access to known IPs
ssh_allowed_ips = [
  "203.0.113.0/24",
]
