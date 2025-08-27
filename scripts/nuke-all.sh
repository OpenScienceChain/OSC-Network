#!/usr/bin/env bash

set -euo pipefail
set +x

# Tear down Fabric test network, stop/remove any CCAAS containers, and purge wallets and service users
# Usage:
#   scripts/nuke-all.sh [-n <ccName>] [--keep-users]
# Defaults:
#   ccName = oscis
#   keep-users = false

ROOTDIR=$(cd "$(dirname "$0")/.." && pwd)
TESTNET_DIR="$ROOTDIR/test-network"
CC_NAME="oscis"
KEEP_USERS=false

while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
    -n) CC_NAME="$2"; shift ;;
    --keep-users) KEEP_USERS=true ;;
    -h|--help)
      sed -n '1,40p' "$0"; exit 0 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
  shift
done

echo "[1/4] Bringing down test network (if running)"
if [[ -d "$TESTNET_DIR" && -x "$TESTNET_DIR/network.sh" ]]; then
  (cd "$TESTNET_DIR" && ./network.sh down) || true
else
  echo "Test network not found at $TESTNET_DIR; skipping"
fi

echo "[2/4] Removing Chaincode-as-a-Service containers"
CLI=${CONTAINER_CLI:-docker}
if command -v "$CLI" >/dev/null 2>&1; then
  # Remove containers with names ending in _ccaas (used by deploy-ccaas) or containing cc name and ccaas
  mapfile -t to_remove < <($CLI ps -a --format '{{.ID}} {{.Names}}' | awk -v cc="$CC_NAME" 'tolower($2) ~ /ccaas/ {print $1}')
  if [[ ${#to_remove[@]} -gt 0 ]]; then
    echo "Removing containers: ${to_remove[*]}"
    $CLI rm -f "${to_remove[@]}" >/dev/null 2>&1 || true
  else
    echo "No CCAAS containers detected"
  fi
else
  echo "Container CLI $CLI not available; skipping container cleanup"
fi

echo "[3/4] Deleting wallets directory"
rm -rf "$ROOTDIR/wallets" || true

echo "[4/4] Deleting service user MSPs (svc-*) in peer orgs: $([[ "$KEEP_USERS" == true ]] && echo "skipped" || echo "enabled")"
if [[ "$KEEP_USERS" == false && -d "$TESTNET_DIR/organizations/peerOrganizations" ]]; then
  find "$TESTNET_DIR/organizations/peerOrganizations" -maxdepth 3 -type d -name 'svc-*' -print -exec rm -rf {} + || true
fi

echo "Nuke complete."


