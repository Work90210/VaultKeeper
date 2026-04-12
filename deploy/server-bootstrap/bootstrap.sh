#!/usr/bin/env bash
#
# VaultKeeper server bootstrap — run once on a fresh Debian 12 install.
# Idempotent where practical. Execute as root.
#
# Usage:
#   scp bootstrap.sh root@<host>:/root/
#   ssh root@<host> 'ADMIN_USER=kyle ADMIN_SSH_PUBKEY="ssh-ed25519 AAAA..." bash /root/bootstrap.sh'
#
# Required env:
#   ADMIN_USER         non-root admin username (e.g. kyle)
#   ADMIN_SSH_PUBKEY   the full public key line for that user
#
# Optional env:
#   SSH_PORT           default 22
#   TIMEZONE           default Etc/UTC
#   TRUSTED_IPS        space-separated list of IPs/CIDRs to add to fail2ban ignoreip
#                      (your home/office IPs so you can't lock yourself out)

set -euo pipefail

: "${ADMIN_USER:?ADMIN_USER is required}"
: "${ADMIN_SSH_PUBKEY:?ADMIN_SSH_PUBKEY is required}"
SSH_PORT="${SSH_PORT:-22}"
TIMEZONE="${TIMEZONE:-Etc/UTC}"
TRUSTED_IPS="${TRUSTED_IPS:-}"

log() { printf '\033[1;36m[bootstrap]\033[0m %s\n' "$*"; }
die() { printf '\033[1;31m[bootstrap] ERROR:\033[0m %s\n' "$*" >&2; exit 1; }

[[ $EUID -eq 0 ]] || die "Must run as root"

log "Setting timezone to $TIMEZONE"
timedatectl set-timezone "$TIMEZONE" || true

log "Updating apt index and upgrading base packages"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get -yqq upgrade

log "Installing baseline packages"
apt-get -yqq install \
    ca-certificates curl gnupg lsb-release \
    ufw fail2ban python3-systemd unattended-upgrades apt-listchanges \
    chrony htop jq git rsync \
    apparmor apparmor-utils \
    needrestart debsums

########################################
# Admin user
########################################
if ! id -u "$ADMIN_USER" >/dev/null 2>&1; then
    log "Creating admin user: $ADMIN_USER"
    adduser --disabled-password --gecos "" "$ADMIN_USER"
fi
usermod -aG sudo "$ADMIN_USER"

# Passwordless sudo for the admin user (key-auth only; no password to escalate with)
install -m 0440 /dev/null "/etc/sudoers.d/90-$ADMIN_USER"
printf '%s ALL=(ALL) NOPASSWD:ALL\n' "$ADMIN_USER" > "/etc/sudoers.d/90-$ADMIN_USER"
visudo -c -q || die "sudoers validation failed"

install -d -m 0700 -o "$ADMIN_USER" -g "$ADMIN_USER" "/home/$ADMIN_USER/.ssh"
AUTH_KEYS="/home/$ADMIN_USER/.ssh/authorized_keys"
touch "$AUTH_KEYS"
if ! grep -qxF "$ADMIN_SSH_PUBKEY" "$AUTH_KEYS"; then
    printf '%s\n' "$ADMIN_SSH_PUBKEY" >> "$AUTH_KEYS"
fi
chown "$ADMIN_USER:$ADMIN_USER" "$AUTH_KEYS"
chmod 0600 "$AUTH_KEYS"

########################################
# SSH hardening
########################################
log "Hardening sshd"
SSHD_DROPIN=/etc/ssh/sshd_config.d/99-vaultkeeper.conf
cat > "$SSHD_DROPIN" <<EOF
# Managed by bootstrap.sh — do not edit by hand.
Port ${SSH_PORT}
Protocol 2
PermitRootLogin no
PasswordAuthentication no
ChallengeResponseAuthentication no
KbdInteractiveAuthentication no
UsePAM yes
PubkeyAuthentication yes
PermitEmptyPasswords no
X11Forwarding no
AllowAgentForwarding no
AllowTcpForwarding no
MaxAuthTries 3
MaxSessions 5
LoginGraceTime 30
ClientAliveInterval 300
ClientAliveCountMax 2
AllowUsers ${ADMIN_USER}
# Strong crypto only
KexAlgorithms curve25519-sha256,curve25519-sha256@libssh.org,diffie-hellman-group16-sha512,diffie-hellman-group18-sha512
Ciphers chacha20-poly1305@openssh.com,aes256-gcm@openssh.com,aes128-gcm@openssh.com
MACs hmac-sha2-512-etm@openssh.com,hmac-sha2-256-etm@openssh.com
HostKeyAlgorithms ssh-ed25519,rsa-sha2-512,rsa-sha2-256
EOF
chmod 0644 "$SSHD_DROPIN"
sshd -t || die "sshd config invalid — refusing to restart"
systemctl reload ssh || systemctl reload sshd

########################################
# UFW firewall
########################################
log "Configuring UFW"
ufw --force reset
ufw default deny incoming
ufw default allow outgoing
ufw limit "${SSH_PORT}/tcp" comment 'SSH (rate-limited)'
ufw allow 80/tcp  comment 'HTTP (Caddy ACME + redirect)'
ufw allow 443/tcp comment 'HTTPS (Caddy)'
ufw --force enable
ufw status verbose

########################################
# fail2ban
########################################
log "Configuring fail2ban"
cat > /etc/fail2ban/jail.d/vaultkeeper.local <<EOF
[DEFAULT]
bantime  = 1h
findtime = 10m
maxretry = 5
backend  = systemd
ignoreip = 127.0.0.1/8 ::1 ${TRUSTED_IPS}

[sshd]
enabled  = true
port     = ${SSH_PORT}
mode     = aggressive
backend  = systemd
EOF
systemctl enable --now fail2ban
systemctl restart fail2ban

########################################
# Unattended upgrades (security only)
########################################
log "Enabling unattended security upgrades"
cat > /etc/apt/apt.conf.d/51vaultkeeper-unattended <<'EOF'
Unattended-Upgrade::Origins-Pattern {
    "origin=Debian,codename=${distro_codename},label=Debian-Security";
};
Unattended-Upgrade::Automatic-Reboot "false";
Unattended-Upgrade::Remove-Unused-Dependencies "true";
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
APT::Periodic::AutocleanInterval "7";
EOF
systemctl enable --now unattended-upgrades

########################################
# Sysctl hardening
########################################
log "Applying sysctl hardening"
cat > /etc/sysctl.d/99-vaultkeeper-hardening.conf <<'EOF'
# Kernel
kernel.kptr_restrict = 2
kernel.dmesg_restrict = 1
kernel.yama.ptrace_scope = 1
kernel.unprivileged_bpf_disabled = 1
kernel.core_uses_pid = 1

# Network — ingress filtering and spoof protection
net.ipv4.conf.all.rp_filter = 1
net.ipv4.conf.default.rp_filter = 1
net.ipv4.conf.all.accept_source_route = 0
net.ipv4.conf.default.accept_source_route = 0
net.ipv6.conf.all.accept_source_route = 0
net.ipv4.conf.all.accept_redirects = 0
net.ipv4.conf.default.accept_redirects = 0
net.ipv6.conf.all.accept_redirects = 0
net.ipv4.conf.all.secure_redirects = 0
net.ipv4.conf.default.secure_redirects = 0
net.ipv4.conf.all.send_redirects = 0
net.ipv4.conf.default.send_redirects = 0
net.ipv4.conf.all.log_martians = 1
net.ipv4.icmp_echo_ignore_broadcasts = 1
net.ipv4.icmp_ignore_bogus_error_responses = 1
net.ipv4.tcp_syncookies = 1
net.ipv4.tcp_rfc1337 = 1
net.ipv6.conf.all.accept_ra = 0
net.ipv6.conf.default.accept_ra = 0

# IPv6 router advertisements (Hetzner uses static v6)
net.ipv6.conf.all.autoconf = 0
net.ipv6.conf.default.autoconf = 0
EOF
sysctl --system >/dev/null

########################################
# Docker CE + compose plugin
########################################
if ! command -v docker >/dev/null 2>&1; then
    log "Installing Docker CE"
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/debian/gpg \
        | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    chmod a+r /etc/apt/keyrings/docker.gpg
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/debian $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
        > /etc/apt/sources.list.d/docker.list
    apt-get update -qq
    apt-get -yqq install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
    systemctl enable --now docker
fi
usermod -aG docker "$ADMIN_USER"

# Docker daemon hardening
install -d -m 0755 /etc/docker
cat > /etc/docker/daemon.json <<'EOF'
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "50m",
    "max-file": "5"
  },
  "live-restore": true,
  "userland-proxy": false,
  "no-new-privileges": true,
  "icc": false,
  "default-ulimits": {
    "nofile": {
      "Name": "nofile",
      "Hard": 65535,
      "Soft": 65535
    }
  }
}
EOF
systemctl restart docker

########################################
# Disable root password login entirely
########################################
log "Locking root password"
passwd -l root

########################################
# Done
########################################
log "Bootstrap complete."
log "Next: log out of root, then log back in as: ssh -p ${SSH_PORT} ${ADMIN_USER}@<host>"
log "Verify: sudo -v && docker version && sudo ufw status"
