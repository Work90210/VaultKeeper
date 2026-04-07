output "server_ip" {
  description = "Public IPv4 address of the VaultKeeper server"
  value       = hcloud_server.vaultkeeper.ipv4_address
}

output "server_id" {
  description = "Hetzner Cloud server ID"
  value       = hcloud_server.vaultkeeper.id
}

output "server_name" {
  description = "Server name in Hetzner Cloud"
  value       = hcloud_server.vaultkeeper.name
}

output "server_status" {
  description = "Current server status"
  value       = hcloud_server.vaultkeeper.status
}
