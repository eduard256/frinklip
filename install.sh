#!/usr/bin/env bash
# frinklip installer.
#
# Fully automatic: detects OS/arch, downloads the latest release binary,
# verifies its sha256, installs to /usr/local/bin, registers a system
# service (systemd on Linux, launchd on macOS), and prints a clickable
# LAN URL when finished. Re-running the script upgrades an existing
# install in place.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/eduard256/frinklip/main/install.sh | bash
#
# Requirements: bash 3.2+, curl OR wget, sudo (the script will prompt
# once if the calling user is not already root).

set -euo pipefail

# ---------- config ----------

REPO="eduard256/frinklip"
BIN_NAME="frinklip"
INSTALL_PATH="/usr/local/bin/${BIN_NAME}"
DROP_DIR="/tmp/dropped"
SERVICE_USER="filedrop"
PORT="3467"

# Linux paths
SYSTEMD_UNIT="/etc/systemd/system/frinklip.service"
SYSTEMD_CONFIG_DIR="/etc/frinklip"

# macOS paths
LAUNCHD_PLIST="/Library/LaunchDaemons/com.frinklip.plist"

# ---------- ui helpers ----------

# ANSI colors only when stdout is a TTY (piping into a file should stay clean).
if [ -t 1 ] && command -v tput >/dev/null 2>&1 && [ "$(tput colors 2>/dev/null || echo 0)" -ge 8 ]; then
    C_GREEN=$(tput setaf 2)
    C_BLUE=$(tput setaf 4)
    C_YELLOW=$(tput setaf 3)
    C_RED=$(tput setaf 1)
    C_DIM=$(tput dim)
    C_BOLD=$(tput bold)
    C_RESET=$(tput sgr0)
else
    C_GREEN=""; C_BLUE=""; C_YELLOW=""; C_RED=""; C_DIM=""; C_BOLD=""; C_RESET=""
fi

log()  { printf '%s==>%s %s\n' "${C_BLUE}" "${C_RESET}" "$*"; }
ok()   { printf '%s ok %s %s\n' "${C_GREEN}" "${C_RESET}" "$*"; }
warn() { printf '%s !! %s %s\n' "${C_YELLOW}" "${C_RESET}" "$*" >&2; }
die()  { printf '%s xx %s %s\n' "${C_RED}" "${C_RESET}" "$*" >&2; exit 1; }

# ---------- privilege escalation ----------

# Resolve a sudo command we can re-use throughout.
#
# Three cases:
#   1. We're root — no sudo prefix needed.
#   2. We're a user with a real TTY (/dev/tty available) — use sudo and
#      let it prompt for a password the first time. We pre-warm the
#      credential cache via `sudo -v` so later calls don't block.
#   3. We're piped from `curl | bash` (no TTY for stdin) — sudo can't
#      prompt for a password; tell the user to re-run as `curl | sudo bash`.
SUDO=""
if [ "$(id -u)" -ne 0 ]; then
    command -v sudo >/dev/null 2>&1 \
        || die "this script needs root privileges and sudo is not installed"
    SUDO="sudo"

    # If sudo can run without a password (cached or NOPASSWD), great.
    # Otherwise we need an interactive terminal for the prompt.
    if ! sudo -n true 2>/dev/null; then
        if [ -t 0 ] || [ -r /dev/tty ]; then
            sudo -v || die "this script needs sudo privileges"
        else
            die "no terminal for sudo password prompt — re-run as: curl -fsSL https://raw.githubusercontent.com/${REPO}/main/install.sh | sudo bash"
        fi
    fi
fi

# ---------- platform detection ----------

detect_platform() {
    local os arch
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    arch="$(uname -m)"

    case "$os" in
        linux)  ;;
        darwin) ;;
        *) die "unsupported OS: $os (linux and macOS only)" ;;
    esac

    case "$arch" in
        x86_64|amd64)  arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *) die "unsupported arch: $arch (amd64 and arm64 only)" ;;
    esac

    echo "${os}-${arch}"
}

# ---------- downloader ----------

# Wrap curl/wget so the rest of the script doesn't care which one is present.
fetch() {
    local url="$1" out="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL --retry 3 --retry-delay 1 -o "$out" "$url"
    elif command -v wget >/dev/null 2>&1; then
        wget -q --tries=3 -O "$out" "$url"
    else
        die "neither curl nor wget is installed"
    fi
}

# ---------- sha256 ----------

# sha256sum (Linux/coreutils) and shasum -a 256 (macOS) print the same
# format. Pick whichever is available.
sha256_of() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | awk '{print $1}'
    elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "$1" | awk '{print $1}'
    else
        die "neither sha256sum nor shasum is installed"
    fi
}

# ---------- LAN ip detection ----------

# Try, in order:
#   1. `ip route get` — asks the kernel which source IP it would use to
#      reach a public address. Most reliable when the box has internet.
#   2. `route -n get` (macOS) + `ipconfig getifaddr` on the resulting iface.
#   3. `hostname -I` (Linux only, gives all addrs — pick first non-docker).
#   4. ifconfig parse — last resort.
#
# Falls back to 127.0.0.1 if everything fails (still useful: user can open
# from the same machine).
detect_lan_ip() {
    local ip=""

    # 1. Linux: ip route get
    if command -v ip >/dev/null 2>&1; then
        ip="$(ip route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src"){print $(i+1); exit}}')"
        if [ -n "$ip" ]; then echo "$ip"; return; fi
    fi

    # 2. macOS: route + ipconfig
    if [ "$(uname -s)" = "Darwin" ] && command -v route >/dev/null 2>&1; then
        local iface
        iface="$(route -n get 1.1.1.1 2>/dev/null | awk '/interface:/{print $2}')"
        if [ -n "$iface" ] && command -v ipconfig >/dev/null 2>&1; then
            ip="$(ipconfig getifaddr "$iface" 2>/dev/null || true)"
            if [ -n "$ip" ]; then echo "$ip"; return; fi
        fi
    fi

    # 3. hostname -I (Linux). Skip docker/virtual bridges by preferring
    #    typical home/office private ranges.
    if command -v hostname >/dev/null 2>&1; then
        ip="$(hostname -I 2>/dev/null | tr ' ' '\n' | grep -E '^(192\.168|10\.|172\.(1[6-9]|2[0-9]|3[01]))\.' | head -1)"
        if [ -n "$ip" ]; then echo "$ip"; return; fi
    fi

    # 4. ifconfig — old-school fallback. Filter out 127.* and pick first private.
    if command -v ifconfig >/dev/null 2>&1; then
        ip="$(ifconfig 2>/dev/null | awk '/inet /{print $2}' | grep -E '^(192\.168|10\.|172\.(1[6-9]|2[0-9]|3[01]))\.' | head -1)"
        if [ -n "$ip" ]; then echo "$ip"; return; fi
    fi

    echo "127.0.0.1"
}

# ---------- service: linux/systemd ----------

install_systemd_service() {
    log "installing systemd service"

    # Create unprivileged user (idempotent).
    if ! id -u "$SERVICE_USER" >/dev/null 2>&1; then
        $SUDO useradd --system --no-create-home --shell /usr/sbin/nologin "$SERVICE_USER"
        ok "created system user '$SERVICE_USER'"
    fi

    # Drop dir owned by service user.
    $SUDO install -d -o "$SERVICE_USER" -g "$SERVICE_USER" -m 0755 "$DROP_DIR"
    $SUDO install -d -m 0755 "$SYSTEMD_CONFIG_DIR"

    # Config: bare minimum, mirrors the in-binary default.
    $SUDO tee "$SYSTEMD_CONFIG_DIR/frinklip.yaml" >/dev/null <<EOF
api:
  listen: ":${PORT}"

upload:
  dir: ${DROP_DIR}
EOF

    # systemd unit. Hardened the same way as systemd/frinklip.service in the repo.
    $SUDO tee "$SYSTEMD_UNIT" >/dev/null <<EOF
[Unit]
Description=frinklip — local file drop server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_USER}
ExecStart=${INSTALL_PATH} -config ${SYSTEMD_CONFIG_DIR}/frinklip.yaml
Restart=always
RestartSec=2

# hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=false
ReadWritePaths=${DROP_DIR}
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

    $SUDO systemctl daemon-reload
    $SUDO systemctl enable frinklip >/dev/null 2>&1 || true
    # Use restart (not start/enable --now) so that an already-running service
    # picks up the new binary and config on upgrade.
    $SUDO systemctl restart frinklip
    ok "systemd service enabled and started"
}

# ---------- service: macOS/launchd ----------

install_launchd_service() {
    log "installing launchd service"

    # On macOS we run the daemon as root for simplicity — file uploads land
    # in /tmp/dropped which is world-writable anyway. A dedicated user adds
    # complexity without much benefit on a single-user developer Mac.
    $SUDO mkdir -p "$DROP_DIR"
    $SUDO chmod 0777 "$DROP_DIR"

    $SUDO tee "$LAUNCHD_PLIST" >/dev/null <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>             <string>com.frinklip</string>
    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_PATH}</string>
    </array>
    <key>RunAtLoad</key>         <true/>
    <key>KeepAlive</key>         <true/>
    <key>StandardOutPath</key>   <string>/var/log/frinklip.log</string>
    <key>StandardErrorPath</key> <string>/var/log/frinklip.log</string>
</dict>
</plist>
EOF
    $SUDO chown root:wheel "$LAUNCHD_PLIST"
    $SUDO chmod 0644 "$LAUNCHD_PLIST"

    # Reload: bootout (ignore failure if not loaded) + bootstrap.
    $SUDO launchctl bootout system "$LAUNCHD_PLIST" 2>/dev/null || true
    $SUDO launchctl bootstrap system "$LAUNCHD_PLIST"
    $SUDO launchctl enable system/com.frinklip
    ok "launchd daemon loaded and started"
}

# ---------- main ----------

main() {
    log "frinklip installer"

    local platform os arch
    platform="$(detect_platform)"
    os="${platform%-*}"
    arch="${platform#*-}"
    ok "platform: ${C_BOLD}${platform}${C_RESET}"

    # Check for an existing install — re-running is the documented upgrade path.
    local upgrading="false"
    if [ -x "$INSTALL_PATH" ]; then
        local current_ver
        current_ver="$($INSTALL_PATH -version 2>/dev/null || echo unknown)"
        log "existing install detected: ${current_ver}"
        log "will upgrade in place"
        upgrading="true"
    fi

    # Download binary + checksums into a temp dir. Use a global name so the
    # EXIT trap below (which runs after main() returns) can still see the
    # variable under `set -u`.
    TMPDIR_FRINKLIP="$(mktemp -d)"
    trap 'rm -rf "${TMPDIR_FRINKLIP:-}"' EXIT
    local tmp="$TMPDIR_FRINKLIP"

    local base_url="https://github.com/${REPO}/releases/latest/download"
    local asset="${BIN_NAME}-${platform}"

    log "downloading ${asset}"
    fetch "${base_url}/${asset}" "${tmp}/${asset}"
    fetch "${base_url}/checksums.txt" "${tmp}/checksums.txt"

    # Verify sha256. checksums.txt has lines like:
    #   <hash>  frinklip-linux-amd64
    log "verifying sha256"
    local expected actual
    expected="$(awk -v a="$asset" '$2==a {print $1}' "${tmp}/checksums.txt")"
    [ -n "$expected" ] || die "no checksum entry for ${asset} in checksums.txt"
    actual="$(sha256_of "${tmp}/${asset}")"
    [ "$expected" = "$actual" ] || die "sha256 mismatch: expected ${expected}, got ${actual}"
    ok "checksum matches"

    # Stop the running service before overwriting the binary on upgrade —
    # otherwise on Linux you can overwrite a busy file but on macOS launchd
    # may refuse to reload cleanly.
    if [ "$upgrading" = "true" ]; then
        if [ "$os" = "linux" ] && command -v systemctl >/dev/null 2>&1 && \
           systemctl list-unit-files frinklip.service >/dev/null 2>&1; then
            $SUDO systemctl stop frinklip 2>/dev/null || true
        elif [ "$os" = "darwin" ] && [ -f "$LAUNCHD_PLIST" ]; then
            $SUDO launchctl bootout system "$LAUNCHD_PLIST" 2>/dev/null || true
        fi
    fi

    # Install binary.
    log "installing to ${INSTALL_PATH}"
    $SUDO install -m 0755 "${tmp}/${asset}" "$INSTALL_PATH"
    ok "binary installed: $($INSTALL_PATH -version)"

    # Register service.
    case "$os" in
        linux)  install_systemd_service ;;
        darwin) install_launchd_service ;;
    esac

    # Health check: poll the root URL until it answers or we give up. The
    # service may need a moment after launchctl/systemctl reports success
    # (especially on low-power boxes). 30 attempts × 0.5s = 15s budget.
    log "waiting for the service to come up"
    local up="false" i
    for i in $(seq 1 30); do
        if curl -fsS -o /dev/null --max-time 1 "http://127.0.0.1:${PORT}/" 2>/dev/null; then
            up="true"
            break
        fi
        sleep 0.5
    done
    if [ "$up" = "true" ]; then
        ok "service is responding on port ${PORT}"
    else
        warn "service did not respond within 15 seconds — check logs:"
        if [ "$os" = "linux" ]; then
            warn "  journalctl -u frinklip -n 50"
        else
            warn "  tail -n 50 /var/log/frinklip.log"
        fi
    fi

    # Final URL.
    local lan_ip
    lan_ip="$(detect_lan_ip)"
    echo
    printf '%sfrinklip is running.%s\n' "${C_BOLD}${C_GREEN}" "${C_RESET}"
    echo
    printf '  Open in your browser:\n'
    printf '    %shttp://%s:%s/%s\n' "${C_BOLD}${C_BLUE}" "${lan_ip}" "${PORT}" "${C_RESET}"
    if [ "$lan_ip" != "127.0.0.1" ]; then
        printf '    %shttp://127.0.0.1:%s/%s  %s(local only)%s\n' \
            "${C_DIM}" "${PORT}" "${C_RESET}" "${C_DIM}" "${C_RESET}"
    fi
    echo
    printf '  Uploads land in: %s%s%s\n' "${C_BOLD}" "${DROP_DIR}" "${C_RESET}"
    echo
    if [ "$os" = "linux" ]; then
        printf '  %sControls:%s  systemctl {status,restart,stop} frinklip   (logs: journalctl -u frinklip -f)\n' \
            "${C_DIM}" "${C_RESET}"
    else
        printf '  %sControls:%s  sudo launchctl {kickstart,bootout} system/com.frinklip   (logs: tail -f /var/log/frinklip.log)\n' \
            "${C_DIM}" "${C_RESET}"
    fi
    echo
}

main "$@"
