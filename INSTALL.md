# Manual installation

This guide walks through every step that [`install.sh`](install.sh) performs,
so you can install frinklip by hand on any Linux or macOS machine. The
end result is identical to running:

```bash
curl -fsSL https://raw.githubusercontent.com/eduard256/frinklip/main/install.sh | sudo bash
```

Use the manual path when you want to inspect every change, install in
a non-standard layout, or operate on systems where piping a script into
the shell is not allowed.

## Contents

- [What gets installed](#what-gets-installed)
- [Linux (systemd)](#linux-systemd)
- [macOS (launchd)](#macos-launchd)
- [Verifying the install](#verifying-the-install)
- [Upgrading](#upgrading)
- [Uninstalling](#uninstalling)
- [Troubleshooting](#troubleshooting)

## What gets installed

| Path                                        | Purpose                                  | Linux | macOS |
|---------------------------------------------|------------------------------------------|:-----:|:-----:|
| `/usr/local/bin/frinklip`                   | The Go binary                            |   ✓   |   ✓   |
| `/etc/frinklip/frinklip.yaml`               | Service config (port, upload dir)        |   ✓   |       |
| `/etc/systemd/system/frinklip.service`      | systemd unit                             |   ✓   |       |
| `/Library/LaunchDaemons/com.frinklip.plist` | launchd daemon                           |       |   ✓   |
| `/tmp/dropped/`                             | Where uploaded files land                |   ✓   |   ✓   |
| system user `filedrop`                      | Unprivileged account the daemon runs as  |   ✓   |       |

The default listening port is **3467** (unprivileged, so no
`CAP_NET_BIND_SERVICE` or `setcap` dance is required). On macOS the
daemon runs as root for simplicity — `/tmp/dropped` is world-writable
either way and a single-user dev Mac does not benefit from a dedicated
service account.

---

## Linux (systemd)

Tested on Ubuntu 22.04+, Debian 12+, Fedora 39+, Arch (rolling), and
RHEL 9. Anything with systemd ≥ 245 should work.

### 1. Pick the right binary for your CPU

```bash
ARCH="$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')"
echo "linux-${ARCH}"
```

Expected output: `linux-amd64` (most servers/desktops) or `linux-arm64`
(Raspberry Pi 4/5, Ampere, AWS Graviton, etc.).

### 2. Download the binary and the checksum file

```bash
ASSET="frinklip-linux-${ARCH}"
BASE="https://github.com/eduard256/frinklip/releases/latest/download"

curl -fL -o "${ASSET}"      "${BASE}/${ASSET}"
curl -fL -o checksums.txt   "${BASE}/checksums.txt"
```

### 3. Verify the SHA256

Refusing to install a binary you have not checked is good hygiene.

```bash
sha256sum -c checksums.txt --ignore-missing
# expected: frinklip-linux-amd64: OK
```

If the line says anything other than `OK`, **stop** — re-download and
do not proceed.

### 4. Install the binary

```bash
sudo install -m 0755 "${ASSET}" /usr/local/bin/frinklip
/usr/local/bin/frinklip -version
```

The last command should print something like
`frinklip v0.1.0 (commit f5bec8c, built 2026-04-29T16:21:10Z)`.

### 5. Create the unprivileged service user

The daemon does not need root — it only needs to bind a port (3467 is
unprivileged) and write to its upload directory. Create a dedicated
account with no shell and no home so a compromise of the binary cannot
escalate to a real login.

```bash
id -u filedrop >/dev/null 2>&1 || \
    sudo useradd --system --no-create-home --shell /usr/sbin/nologin filedrop
```

The `id` check makes the command idempotent — re-running it is safe.

### 6. Create the upload directory

```bash
sudo install -d -o filedrop -g filedrop -m 0755 /tmp/dropped
```

`/tmp/dropped` lives under `/tmp`, so it survives until the next reboot
or until tmpfiles cleanup runs. That's intentional — frinklip is a
scratch space, not durable storage.

### 7. Write the config file

```bash
sudo install -d -m 0755 /etc/frinklip
sudo tee /etc/frinklip/frinklip.yaml >/dev/null <<'EOF'
api:
  listen: ":3467"

upload:
  dir: /tmp/dropped
EOF
```

The config matches the in-binary defaults; you can skip this file
entirely if you don't need to override anything, but having it explicit
makes the systemd unit's `-config` flag predictable.

### 8. Write the systemd unit

```bash
sudo tee /etc/systemd/system/frinklip.service >/dev/null <<'EOF'
[Unit]
Description=frinklip — local file drop server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=filedrop
Group=filedrop
ExecStart=/usr/local/bin/frinklip -config /etc/frinklip/frinklip.yaml
Restart=always
RestartSec=2

# hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=false
ReadWritePaths=/tmp/dropped
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictNamespaces=true
RestrictRealtime=true
LockPersonality=true
MemoryDenyWriteExecute=true

[Install]
WantedBy=multi-user.target
EOF
```

What the hardening directives do, in plain English:

- `NoNewPrivileges` — process and children can never gain new privileges
  (blocks setuid escalation).
- `ProtectSystem=strict` — `/usr`, `/boot`, `/etc` mounted read-only.
- `ProtectHome=true` — `/home`, `/root` are invisible to the daemon.
- `PrivateTmp=false` — we explicitly need to **share** `/tmp/dropped`
  with the host, so we opt out of systemd's per-service tmp namespace.
- `ReadWritePaths=/tmp/dropped` — the only path the daemon can write to.
- `Protect*` and `Restrict*` — block kernel tuning, module loading,
  cgroups manipulation, namespace creation, real-time scheduling.
- `LockPersonality` / `MemoryDenyWriteExecute` — block exec of
  freshly-written memory, defeats a class of code-injection attacks.

### 9. Reload systemd, enable, start

```bash
sudo systemctl daemon-reload
sudo systemctl enable frinklip
sudo systemctl restart frinklip
```

`daemon-reload` makes systemd notice the new unit. `enable` registers
it for autostart at boot. `restart` is preferred over `start` because
it works whether the service is currently running (upgrade case) or
not (fresh install).

Skip to [Verifying the install](#verifying-the-install).

---

## macOS (launchd)

Tested on macOS 13 Ventura, 14 Sonoma, 15 Sequoia, on both Intel and
Apple Silicon.

### 1. Pick the right binary

```bash
ARCH="$(uname -m | sed 's/x86_64/amd64/;s/arm64/arm64/')"
echo "darwin-${ARCH}"
```

Apple Silicon Macs (M1/M2/M3/M4) report `arm64`. Older Intel Macs
report `amd64`.

### 2. Download and verify

```bash
ASSET="frinklip-darwin-${ARCH}"
BASE="https://github.com/eduard256/frinklip/releases/latest/download"

curl -fL -o "${ASSET}"    "${BASE}/${ASSET}"
curl -fL -o checksums.txt "${BASE}/checksums.txt"

shasum -a 256 -c checksums.txt --ignore-missing
# expected: frinklip-darwin-arm64: OK
```

### 3. Strip the Gatekeeper quarantine attribute

macOS marks any binary downloaded via Safari/curl/wget with
`com.apple.quarantine`. Without removing it, the first launch is
blocked by Gatekeeper with a dialog you cannot dismiss from a daemon
context.

```bash
xattr -d com.apple.quarantine "${ASSET}" 2>/dev/null || true
```

The `|| true` guard handles the case where the attribute is already
absent (e.g. on a fresh `brew` build).

### 4. Install the binary

```bash
sudo install -m 0755 "${ASSET}" /usr/local/bin/frinklip
/usr/local/bin/frinklip -version
```

### 5. Create the upload directory

```bash
sudo mkdir -p /tmp/dropped
sudo chmod 0777 /tmp/dropped
```

The daemon will run as root on macOS, so the `0777` here is permissive
mostly for convenience — anyone on the box can write to `/tmp/dropped`.
On a single-user dev Mac that's the expected ergonomics.

### 6. Write the launchd plist

```bash
sudo tee /Library/LaunchDaemons/com.frinklip.plist >/dev/null <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>             <string>com.frinklip</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/frinklip</string>
    </array>
    <key>RunAtLoad</key>         <true/>
    <key>KeepAlive</key>         <true/>
    <key>StandardOutPath</key>   <string>/var/log/frinklip.log</string>
    <key>StandardErrorPath</key> <string>/var/log/frinklip.log</string>
</dict>
</plist>
EOF

sudo chown root:wheel /Library/LaunchDaemons/com.frinklip.plist
sudo chmod 0644       /Library/LaunchDaemons/com.frinklip.plist
```

`/Library/LaunchDaemons` is the system-wide location — the daemon
starts at boot regardless of which user is logged in.
`~/Library/LaunchAgents` would only run when **you** are logged in,
which is not what we want for a LAN drop service.

The plist keys, briefly:

- `Label` — unique reverse-DNS identifier; matches the filename without
  the extension.
- `ProgramArguments` — argv for the daemon; first element is the
  binary, the rest are CLI flags. We pass none — the binary picks the
  right defaults.
- `RunAtLoad` — start immediately when the plist is loaded.
- `KeepAlive` — restart the daemon if it exits, regardless of why.
- `Standard*Path` — log destinations (`launchctl` itself does not
  capture stdout/stderr).

### 7. Load and start

```bash
# bootout first (in case an old version is loaded), then bootstrap.
sudo launchctl bootout system /Library/LaunchDaemons/com.frinklip.plist 2>/dev/null || true
sudo launchctl bootstrap system /Library/LaunchDaemons/com.frinklip.plist
sudo launchctl enable system/com.frinklip
```

`bootstrap`/`bootout` is the modern launchctl interface (replaces the
deprecated `load`/`unload`). The `system/` prefix is the launchctl
"domain" — the system domain is the only one daemons can live in.
`enable` makes sure the service is not in a disabled state (which can
linger from a previous `launchctl disable`).

---

## Verifying the install

Same procedure on both OSes.

### Bind check

```bash
curl -s -o /dev/null -w 'HTTP %{http_code}\n' http://127.0.0.1:3467/
# expected: HTTP 200
```

If you get `Connection refused` or `HTTP 000`, the daemon is not
listening — see [Troubleshooting](#troubleshooting).

### Find your LAN address

The daemon listens on every interface (`0.0.0.0:3467`). To open the UI
from another device, you need this machine's address on the LAN.

**Linux:**

```bash
ip route get 1.1.1.1 | awk '{for(i=1;i<=NF;i++) if($i=="src"){print $(i+1); exit}}'
# example output: 192.168.1.42
```

**macOS:**

```bash
IFACE="$(route -n get 1.1.1.1 | awk '/interface:/{print $2}')"
ipconfig getifaddr "$IFACE"
# example output: 192.168.1.42
```

Open `http://<that-address>:3467/` in any browser on the same network.

### Service status

**Linux:**

```bash
systemctl status frinklip          # one-shot status
journalctl -u frinklip -f          # follow logs
systemctl is-enabled frinklip      # 'enabled' = autostarts at boot
```

**macOS:**

```bash
sudo launchctl print system/com.frinklip   # detailed status
tail -f /var/log/frinklip.log              # follow logs
```

### Survives a reboot

```bash
sudo reboot
```

After login:

```bash
# Linux
systemctl is-active frinklip   # → active

# macOS
sudo launchctl print system/com.frinklip | grep state
```

---

## Upgrading

Either re-run `install.sh` (it detects an existing install and replaces
it in place), or repeat steps **2–4 / 8–9** (Linux) or **2–4 / 7**
(macOS). Always end with a restart so the new binary is picked up:

```bash
# Linux
sudo systemctl restart frinklip

# macOS
sudo launchctl kickstart -k system/com.frinklip
```

`kickstart -k` kills the running daemon and immediately re-launches it
under the current plist, picking up any binary changes.

---

## Uninstalling

### Linux

```bash
sudo systemctl disable --now frinklip
sudo rm /etc/systemd/system/frinklip.service
sudo rm -rf /etc/frinklip
sudo rm /usr/local/bin/frinklip
sudo systemctl daemon-reload
```

Optional, removes user data too:

```bash
sudo userdel filedrop
sudo rm -rf /tmp/dropped
```

### macOS

```bash
sudo launchctl bootout system /Library/LaunchDaemons/com.frinklip.plist
sudo rm /Library/LaunchDaemons/com.frinklip.plist
sudo rm /usr/local/bin/frinklip
sudo rm -f /var/log/frinklip.log
```

Optional:

```bash
sudo rm -rf /tmp/dropped
```

---

## Troubleshooting

### `bind: address already in use` on port 3467

Something else is already listening. Find it:

```bash
sudo ss -ltnp | grep 3467     # Linux
sudo lsof -iTCP:3467 -sTCP:LISTEN    # macOS
```

Either stop that process, or change the port:

```bash
sudo sed -i 's/:3467/:8467/' /etc/frinklip/frinklip.yaml   # Linux
sudo systemctl restart frinklip
```

On macOS the binary reads no config by default — pass `-config` in the
plist's `ProgramArguments` if you need a non-default port.

### `service did not respond within 15 seconds`

Check the logs:

```bash
journalctl -u frinklip -n 50          # Linux
tail -n 50 /var/log/frinklip.log       # macOS
```

Common causes:

- The binary failed to start at all (bad binary download — re-verify
  the checksum).
- `/tmp/dropped` is missing or has wrong ownership — recreate it.
- Firewall is blocking even loopback (rare; check `iptables`/`pf`).

### Firewall blocks LAN access

The daemon binds `0.0.0.0:3467` so it's reachable from the LAN, but
the host firewall must allow inbound 3467/tcp.

**Linux (ufw, common on Ubuntu):**

```bash
sudo ufw allow 3467/tcp
```

**Linux (firewalld, common on Fedora/RHEL):**

```bash
sudo firewall-cmd --add-port=3467/tcp --permanent
sudo firewall-cmd --reload
```

**macOS:**

System Settings → Network → Firewall → Options → ensure `frinklip` is
allowed for incoming connections, or temporarily disable the firewall
for testing.

### macOS Gatekeeper blocks the binary

Symptom: `“frinklip” cannot be opened because the developer cannot be
verified.`

Fix:

```bash
sudo xattr -d com.apple.quarantine /usr/local/bin/frinklip
sudo launchctl kickstart -k system/com.frinklip
```

### "couldn't read the binary version" / `frinklip: command not found`

`/usr/local/bin` is in the default `PATH` on Linux and macOS, but not
in every environment (e.g. some minimal systemd unit contexts). Use
the absolute path:

```bash
/usr/local/bin/frinklip -version
```

### I want to run it without a service manager

Just run the binary in the foreground:

```bash
/usr/local/bin/frinklip
```

It will block in the terminal, listen on `:3467`, and exit on Ctrl+C.
Useful for debugging or one-off use; not what you want for "always
running on the LAN".
