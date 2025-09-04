#!/usr/bin/env bash

set -euo pipefail
set +x

# Orchestrates the local dev flow:
# 1) Nuke any existing network/artifacts
# 2) Quick start the test network
# 3) Deploy chaincode-as-a-service (CCaaS)
# 4) Seed and list artifacts
# 5) Create service wallets for all orgs
#
# Usage:
#   scripts/run-dev-pipeline.sh [-c <channel>] [-n <ccName>]
# Defaults: channel=mychannel, ccName=oscis

ROOTDIR=$(cd "$(dirname "$0")/.." && pwd)

CHANNEL_NAME="mychannel"
CC_NAME="oscis"

while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
    -c) CHANNEL_NAME="$2"; shift ;;
    -n) CC_NAME="$2"; shift ;;
    -h|--help)
      sed -n '1,40p' "$0"; exit 0 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
  shift
done

info() { echo "[INFO] $*"; }
ok() { echo "[OK]   $*"; }

run_step() {
  local title="$1"; shift
  info "$title"
  "$@"
  ok "$title"
}

run_step "Nuking any existing network and artifacts" \
  "$ROOTDIR/scripts/nuke-all.sh" -n "$CC_NAME"

run_step "Starting test network (channel=$CHANNEL_NAME)" \
  "$ROOTDIR/scripts/quick-start.sh" -c "$CHANNEL_NAME"

run_step "Deploying CCaaS (ccName=$CC_NAME, channel=$CHANNEL_NAME)" \
  "$ROOTDIR/scripts/deploy-ccaas.sh" -c "$CHANNEL_NAME" -n "$CC_NAME" -s auto

run_step "Seeding and listing artifacts (ccName=$CC_NAME, channel=$CHANNEL_NAME)" \
  "$ROOTDIR/scripts/seed-and-list-artifacts.sh" -c "$CHANNEL_NAME" -n "$CC_NAME"

run_step "Sanity: random artifact 20 updates + history (ccName=$CC_NAME, channel=$CHANNEL_NAME)" \
  "$ROOTDIR/scripts/sanity-artifact-update-history.sh" -c "$CHANNEL_NAME" -n "$CC_NAME" -u 20

run_step "Creating service wallets for all orgs" \
  "$ROOTDIR/scripts/create-service-wallets.sh"

info "Dev pipeline completed successfully."


