# VaultKeeper Server Bootstrap

End-to-end runbook for standing up the Hetzner dedicated server (`178.63.233.81`) securely, from Hetzner rescue mode to a hardened Debian 12 host ready to run the existing `docker-compose.yml` stack with Caddy fronting TLS.

> **Server**: Server Auction #2969224
> **IPv4**: `178.63.233.81`
> **IPv6**: `2a01:4f8:2240:1269::2`
> **Domain**: `vaultkeeper.eu` (DNS at Spaceship)

---

## 0. Security preflight (do this first)

The Hetzner order email contains a plaintext root password. Treat it as **burned** — anyone who has seen the email has it. You'll disable password authentication entirely in step 3, but until then assume the server could be probed.

Checklist before doing anything else:

- [ ] **DNS**: create an A record `vaultkeeper.eu → 178.63.233.81` and AAAA `vaultkeeper.eu → 2a01:4f8:2240:1269::2` at Spaceship. (Caddy uses HTTP-01 on port 80 which requires DNS to already resolve.)
- [ ] **Local SSH key**: generate a dedicated keypair for this host (step 1).
- [ ] **Host key fingerprints** from the Hetzner email — keep these in a scratch buffer to verify on first connection:
  ```
  URM92+kkZSTiMEL5m/eRFh/AfNUDyEuzqEK54U0HDoc  (ECDSA 256)
  y3NYbV+UnT8qbHgXCoYm2jsyUvU6zlRbIsKnrk9sW0E  (ED25519 256)
  kKazb2xMwEERzpW4gCfin5GD/sBrGokO8KhRkM69cXo  (RSA 3072)
  ```
  Note: these are the **rescue system** keys. The fingerprints will change after `installimage` replaces the OS.

---

## 1. Generate a dedicated SSH keypair (local)

Run this in your terminal — it prompts for a passphrase, which is why you run it, not Claude. Use a strong passphrase and store it in your password manager.

```bash
ssh-keygen -t ed25519 -a 100 \
  -f ~/.ssh/vaultkeeper_prod_ed25519 \
  -C "kyle@vaultkeeper-prod $(date +%Y-%m-%d)"
```

Flags:
- `-t ed25519` — modern elliptic-curve key (small, fast, secure).
- `-a 100` — 100 KDF rounds; makes the encrypted private key expensive to brute-force if stolen.
- `-f` — dedicated file; do not reuse `~/.ssh/id_ed25519`.
- `-C` — comment to identify the key in `authorized_keys`.

Verify and add an SSH config entry so future connections are effortless:

```bash
cat ~/.ssh/vaultkeeper_prod_ed25519.pub

cat >> ~/.ssh/config <<'EOF'

Host vaultkeeper-prod
    HostName 178.63.233.81
    User kyle
    IdentityFile ~/.ssh/vaultkeeper_prod_ed25519
    IdentitiesOnly yes
EOF
```

Add the key to your agent so you only type the passphrase once per session:

```bash
ssh-add --apple-use-keychain ~/.ssh/vaultkeeper_prod_ed25519   # macOS
# or: ssh-add ~/.ssh/vaultkeeper_prod_ed25519
```

---

## 2. Install Debian 12 from the rescue system

The server boots into Hetzner rescue by default. You'll use `installimage` to put a clean OS on it and inject your pubkey so you never need the rescue root password on the real system.

### 2.1 Log into rescue

```bash
ssh root@178.63.233.81
# password: <from Hetzner email>
```

On the **very first connection**, SSH will print the host key fingerprint. Compare it byte-for-byte against the fingerprints above. If they don't match, **disconnect** and contact Hetzner — someone is in the middle.

### 2.2 Upload your public key and installimage config

From your local machine, in a second terminal:

```bash
cd deploy/server-bootstrap

# Upload the installimage template
scp installimage.conf root@178.63.233.81:/root/vaultkeeper.installimage

# Upload your public key for the new system
scp ~/.ssh/vaultkeeper_prod_ed25519.pub root@178.63.233.81:/tmp/authorized_keys
```

### 2.3 Verify drives and run installimage

Back in the rescue SSH session:

```bash
lsblk                          # verify drive names (nvme0n1, nvme1n1 vs sda, sdb)
cat /root/vaultkeeper.installimage   # sanity-check the config
```

If `lsblk` shows `/dev/sda` + `/dev/sdb` instead of nvme, edit `/root/vaultkeeper.installimage` and replace the `DRIVE1`/`DRIVE2` lines accordingly.

Then run installimage **non-interactively** with our config:

```bash
installimage -a -c /root/vaultkeeper.installimage
```

Wait for it to finish (partitioning, image extract, grub, ~5 min), then:

```bash
reboot
```

Close the SSH session. Wait ~60 seconds for the reboot.

### 2.4 First connection to the new system

```bash
ssh root@178.63.233.81
```

You will get a **new** host key fingerprint warning — this is expected (fresh OS means fresh keys). Accept it. Because we injected the pubkey via `SSHKEYS_URL`, this login will use your key, not the rescue password.

Tip: if your local `known_hosts` has an old entry, run `ssh-keygen -R 178.63.233.81` first.

---

## 3. Harden the server with `bootstrap.sh`

Still in the new-system SSH session (as root):

### 3.1 Upload the bootstrap script

From local:

```bash
scp deploy/server-bootstrap/bootstrap.sh root@178.63.233.81:/root/
```

### 3.2 Execute it

Back on the server, pass the admin user and pubkey as env vars. **The `ADMIN_SSH_PUBKEY` value must be the full single-line contents of your `.pub` file** — copy it from `cat ~/.ssh/vaultkeeper_prod_ed25519.pub`:

```bash
export ADMIN_USER=kyle
export ADMIN_SSH_PUBKEY='ssh-ed25519 AAAA...the-whole-line... kyle@vaultkeeper-prod 2026-04-11'
bash /root/bootstrap.sh
```

What the script does (see `bootstrap.sh` for the full source):

| Section | Effect |
|---|---|
| Base packages | Update, install `ufw fail2ban unattended-upgrades chrony apparmor`, etc. |
| Admin user | Creates `kyle`, installs your pubkey, adds passwordless sudo (safe because there's no password at all) |
| sshd | Drop-in config at `/etc/ssh/sshd_config.d/99-vaultkeeper.conf`: no root login, no passwords, strong crypto only, `AllowUsers kyle` |
| UFW | Default deny in / allow out; allow `80/tcp`, `443/tcp`, rate-limited SSH |
| fail2ban | Aggressive `sshd` jail, 1h ban, 5 retries per 10 min |
| Unattended upgrades | Security-only patches, no auto-reboot |
| Sysctl | Kernel hardening (kptr_restrict, yama ptrace scope, rp_filter, tcp syncookies, disable ICMP redirects, etc.) |
| Docker | Installs Docker CE + compose plugin; daemon hardened (`no-new-privileges`, `userland-proxy: false`, `live-restore`, JSON log rotation) |
| Root | Locks root password entirely |

### 3.3 Verify from a fresh session

**Open a new terminal** (keep the root session open as a safety net in case you got locked out):

```bash
ssh vaultkeeper-prod          # uses the config from step 1
sudo -v                        # should succeed without a password prompt
sudo ufw status verbose        # 22 rate-limited, 80/443 open
sudo systemctl status fail2ban
docker version
```

Only once this new session works end-to-end, close the original root session. From now on, **never SSH in as root again**.

---

## 4. Deploy VaultKeeper with Caddy

The repo already contains:

- `docker-compose.yml` — API, Postgres, MinIO, Keycloak, Meilisearch, Caddy
- `Caddyfile` — TLS termination for `vaultkeeper.eu`, automatic ACME via HTTP-01

Caddy obtains certs automatically on first start — no extra config needed, provided:

1. `vaultkeeper.eu` resolves to `178.63.233.81` (check with `dig +short vaultkeeper.eu`)
2. Port 80 is reachable from the internet (UFW rule allows it)
3. The `email` directive in `Caddyfile` is valid (currently `admin@vaultkeeper.eu` — make sure that mailbox exists so you get expiry warnings)

### 4.1 Clone the repo on the server

```bash
# on the server, as kyle
sudo mkdir -p /srv/vaultkeeper
sudo chown kyle:kyle /srv/vaultkeeper
cd /srv/vaultkeeper
git clone https://github.com/<your-org>/VaultKeeper.git .
```

### 4.2 Populate `.env`

The compose file has strict `:?required` guards for:

- `POSTGRES_PASSWORD`
- `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`
- `KEYCLOAK_ADMIN_PASSWORD`
- `MEILISEARCH_API_KEY`

Generate strong secrets and write them to `.env`:

```bash
umask 077
cat > .env <<EOF
POSTGRES_USER=vaultkeeper
POSTGRES_PASSWORD=$(openssl rand -base64 32 | tr -d '=+/' | cut -c1-32)
MINIO_ACCESS_KEY=$(openssl rand -hex 16)
MINIO_SECRET_KEY=$(openssl rand -base64 32 | tr -d '=+/' | cut -c1-40)
KEYCLOAK_ADMIN=admin
KEYCLOAK_ADMIN_PASSWORD=$(openssl rand -base64 32 | tr -d '=+/' | cut -c1-32)
MEILISEARCH_API_KEY=$(openssl rand -base64 32 | tr -d '=+/' | cut -c1-40)
EOF
chmod 0600 .env
```

Record these in your password manager before running anything — the Keycloak admin password in particular is painful to rotate.

### 4.3 Start the stack

```bash
docker compose up -d
docker compose ps
docker compose logs -f caddy
```

Caddy will:
1. Bind to 80/443.
2. Send an ACME order to Let's Encrypt for `vaultkeeper.eu`.
3. Solve the HTTP-01 challenge on port 80.
4. Write the cert + key into the `caddy_data` volume (persistent).
5. Start serving HTTPS on 443 with the security headers from `Caddyfile`.

Verify externally:

```bash
# From your laptop:
curl -vI https://vaultkeeper.eu/health
# Expect: HTTP/2 200, valid cert, HSTS + CSP headers
```

If the cert fails to issue, check:

- `docker compose logs caddy` for ACME errors
- `dig +short vaultkeeper.eu` from multiple resolvers
- Port 80 reachability: `curl -v http://vaultkeeper.eu/` from an external host (not the server itself)

### 4.4 (Optional) Wildcard certs

You selected DNS-01 with Spaceship earlier, but **the stock `caddy:2-alpine` image does not include a Spaceship DNS plugin** (Caddy's DNS providers are compile-time modules via `xcaddy`). Sticking with HTTP-01 for the apex + subdomains is simplest and has no downside unless you need genuinely wildcard certs.

If you do need `*.vaultkeeper.eu`:

1. Build a custom Caddy image with [xcaddy](https://github.com/caddyserver/xcaddy) and a DNS provider module. Spaceship has no first-party Caddy module at the moment; your options are (a) transfer DNS to Cloudflare/Hetzner/deSEC which do have modules, or (b) write a `tls.dns.exec` shim against Spaceship's API.
2. Add `tls { dns <provider> {env.API_TOKEN} }` to the Caddyfile site block.

Unless you have a pressing need for wildcards, defer this.

---

## 5. Post-deploy verification

- [ ] `https://vaultkeeper.eu/health` returns 200 with a valid Let's Encrypt cert
- [ ] [SSL Labs](https://www.ssllabs.com/ssltest/analyze.html?d=vaultkeeper.eu) — aim for A/A+
- [ ] [securityheaders.com](https://securityheaders.com/?q=vaultkeeper.eu) — aim for A+
- [ ] `ssh root@178.63.233.81` is **refused** (root login disabled)
- [ ] `ssh -o PreferredAuthentications=password kyle@vaultkeeper-prod` is **refused** (passwords disabled)
- [ ] `sudo fail2ban-client status sshd` shows the jail active
- [ ] `sudo unattended-upgrade --dry-run --debug` runs clean
- [ ] Database backups (handled by the Go app's internal scheduler per `infrastructure/ansible/deploy.yml:41`) are writing to their configured destination
- [ ] `docker compose ps` — all services `Up (healthy)`

---

## 6. Ongoing operations

| Task | Command |
|---|---|
| View Caddy access log | `docker compose exec caddy tail -f /var/log/caddy/access.log` |
| Rotate admin SSH key | Append new pubkey to `/home/kyle/.ssh/authorized_keys`, verify, remove old |
| Reboot for kernel updates | `sudo needrestart -r a` then `sudo reboot` |
| Review failed SSH attempts | `sudo fail2ban-client status sshd` and `sudo journalctl -u ssh --since -1d` |
| Audit installed packages | `sudo debsums -c` |
| Check cert expiry | `docker compose exec caddy caddy list-certificates` (Caddy auto-renews at 30d remaining) |

---

## Files in this directory

| File | Purpose |
|---|---|
| `README.md` | This runbook |
| `installimage.conf` | Hetzner `installimage` template — Debian 12, RAID1, LVM, dedicated Docker volume |
| `bootstrap.sh` | Idempotent hardening script (user, SSH, UFW, fail2ban, sysctl, Docker) |
| `.gitignore` | Keeps `.env`, keys, and ACME state out of git |

## What is NOT in this directory (by design)

- The generated SSH keypair — lives in `~/.ssh/vaultkeeper_prod_ed25519`, never checked in
- `.env` with real secrets — generated on the server
- Caddy config — the existing `Caddyfile` at repo root already handles TLS for `vaultkeeper.eu`; no changes needed for HTTP-01 issuance
