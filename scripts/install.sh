#!/usr/bin/env bash
# Nefo installer.
#
#   curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | sudo bash
#
# Environment variables:
#   NEFO_REPO     GitHub repository to download from (default: nefocp/nefo)
#   NEFO_VERSION  release tag such as v0.1.0 (default: latest)
#   NEFO_FORCE=1  try to install on an operating system that is not supported
#   NEFO_SETUP    options for "nefo setup", run once the panel has started
#
# To set the server up in the same run, give "nefo setup" options after --
# or in NEFO_SETUP (see "nefo setup -h"):
#
#   curl -fsSL .../install.sh | sudo bash -s -- --web nginx --php 8.4 --db mariadb
#   curl -fsSL .../install.sh | sudo NEFO_SETUP="--web nginx --php 8.4" bash
#
# Without them, nothing but the panel is installed.
set -euo pipefail

REPO="${NEFO_REPO:-nefocp/nefo}"
VERSION="${NEFO_VERSION:-latest}"
BIN=/usr/local/bin/nefo
CONF_DIR=/etc/nefo
CONF="$CONF_DIR/config.json"
DATA_DIR=/var/lib/nefo
UNIT=/etc/systemd/system/nefo.service

die() { echo "Error: $*" >&2; exit 1; }
step() { echo "==> $*"; }

[ "$(id -u)" -eq 0 ] || die "run this script as root (for example with sudo)"
command -v systemctl >/dev/null 2>&1 || die "systemd is required"
command -v curl >/dev/null 2>&1 || die "curl is required"
command -v sha256sum >/dev/null 2>&1 || die "sha256sum is required"

# ---- setup options ----
# The options for "nefo setup": NEFO_SETUP, then the arguments after "--".
# NEFO_SETUP is split on spaces on purpose, with globbing off; an option
# may only hold letters, digits and . , = _ -
if [ "${1:-}" = "--" ]; then
  shift
fi
set -f
# shellcheck disable=SC2086
set -- ${NEFO_SETUP:-} "$@"
set +f
for SETUP_ARG in "$@"; do
  case "$SETUP_ARG" in
    *[!A-Za-z0-9.,=_-]*) die "invalid setup option \"$SETUP_ARG\" (see nefo setup -h)" ;;
  esac
done

# ---- operating system ----
[ -r /etc/os-release ] || die "cannot read /etc/os-release"
# Read it in a subshell: os-release sets VERSION, NAME and more, which would
# overwrite this script's own variables.
OS="$(. /etc/os-release && printf '%s:%s' "${ID:-}" "${VERSION_ID:-}")"
case "$OS" in
  ubuntu:22.04 | ubuntu:24.04 | ubuntu:26.04 | debian:12 | debian:13) ;;
  *)
    if [ "${NEFO_FORCE:-0}" != 1 ]; then
      die "$(. /etc/os-release && printf '%s' "${PRETTY_NAME:-this system}") is not supported yet. Supported: Ubuntu 22.04/24.04/26.04, Debian 12/13. Set NEFO_FORCE=1 to try anyway."
    fi
    ;;
esac

case "$(uname -m)" in
  x86_64 | amd64) ARCH=amd64 ;;
  aarch64 | arm64) ARCH=arm64 ;;
  *) die "unsupported CPU architecture $(uname -m)" ;;
esac

if [ -e "$CONF" ]; then
  die "the panel is already installed ($CONF exists). Run 'nefo info' to see its URL."
fi

# ---- download and verify ----
if [ "$VERSION" = latest ]; then
  BASE="https://github.com/$REPO/releases/latest/download"
else
  BASE="https://github.com/$REPO/releases/download/$VERSION"
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

step "Downloading nefo-linux-$ARCH ($VERSION)"
curl -fsSL "$BASE/nefo-linux-$ARCH" -o "$TMP/nefo-linux-$ARCH"
curl -fsSL "$BASE/checksums.txt" -o "$TMP/checksums.txt"

# The release signing key (Ed25519, PEM). The release build fills it in;
# when it is empty (a development build) the signature is not checked.
SIGNING_KEY="$(cat <<'NEFO_SIGNING_KEY'
NEFO_SIGNING_KEY
)"

if [ -n "$SIGNING_KEY" ]; then
  step "Verifying the release signature"
  command -v openssl >/dev/null 2>&1 || die "openssl is required to verify the release signature"
  curl -fsSL "$BASE/checksums.txt.sig" -o "$TMP/checksums.txt.sig"
  printf '%s\n' "$SIGNING_KEY" > "$TMP/release.pub"
  openssl pkeyutl -verify -pubin -inkey "$TMP/release.pub" -rawin \
    -in "$TMP/checksums.txt" -sigfile "$TMP/checksums.txt.sig" >/dev/null 2>&1 ||
    die "invalid signature: checksums.txt was not signed by the project. Do not install this file."
else
  echo "  (development build: the release signature is not checked)"
fi

step "Verifying checksum"
(cd "$TMP" && grep " nefo-linux-$ARCH\$" checksums.txt | sha256sum -c --status -) ||
  die "checksum mismatch: the download is corrupt or has been tampered with"

install -m 0755 "$TMP/nefo-linux-$ARCH" "$BIN"
install -d -m 0700 "$CONF_DIR" "$DATA_DIR"
if [ "$#" -gt 0 ]; then
  "$BIN" setup --check "$@" || die "the setup options are not valid (see above): nothing was configured"
fi

# ---- configure ----
step "Creating the configuration and the admin account"
HOST_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
INIT_OUTPUT="$("$BIN" init --config "$CONF" --data "$DATA_DIR" --host "${HOST_IP:-SERVER_IP}")"
PORT="$("$BIN" info --config "$CONF" --port)"

step "Installing the systemd service"
cat >"$UNIT" <<EOF
[Unit]
Description=Nefo server control panel
Documentation=https://nefo.io
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=$BIN serve --config $CONF
Restart=on-failure
RestartSec=3
UMask=0077
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable --now nefo.service

# ---- firewall ----
if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "Status: active"; then
  step "Opening port $PORT/tcp in ufw"
  ufw allow "$PORT/tcp" comment "Nefo" >/dev/null
elif command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
  step "Opening port $PORT/tcp in firewalld"
  firewall-cmd --permanent --add-port="$PORT/tcp" >/dev/null
  firewall-cmd --reload >/dev/null
fi

# ---- server setup (only with setup options) ----
# The running panel does the installs as one of its tasks, so they go on
# even if this session ends; the panel's Software page shows them too.
SETUP_STATUS=0
if [ "$#" -gt 0 ]; then
  step "Setting up the server: nefo setup $*"
  "$BIN" setup --config "$CONF" --yes --wait 60s "$@" || SETUP_STATUS=$?
fi

echo "$INIT_OUTPUT"
echo "If your provider has a cloud firewall (security group), allow TCP port $PORT there too."
echo "Manage the service with: systemctl status|restart nefo   Logs: journalctl -u nefo"
if [ "$SETUP_STATUS" -ne 0 ]; then
  echo "The panel is installed, but the server setup stopped (see above). Fix the problem, then run: nefo setup $*" >&2
  exit "$SETUP_STATUS"
fi
