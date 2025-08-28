#!/usr/bin/env bash

set -euo pipefail

# List all artifacts via ListArtifacts query
# Usage:
#   scripts/list-artifacts.sh [-c <channelName>] [-n <chaincodeName>]
# Defaults:
#   channelName = mychannel
#   chaincodeName = oscis

ROOTDIR=$(cd "$(dirname "$0")/.." && pwd)
TESTNET_DIR="${ROOTDIR}/test-network"

CHANNEL_NAME="mychannel"
CC_NAME="oscis"

# Prevent "unbound variable" errors inside sourced test-network scripts
export OVERRIDE_ORG=""
# Default verbose flag expected by test-network scripts
export VERBOSE=false

while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
    -c) CHANNEL_NAME="$2"; shift ;;
    -n) CC_NAME="$2"; shift ;;
    -h|--help)
      sed -n '1,12p' "$0"; exit 0 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
  shift
done

cd "$TESTNET_DIR"

# Load environment helpers from test-network
set +u
source ./scripts/envVar.sh
set -u

# Ensure jq is available for pretty-printing
if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required but not installed. Please install jq and retry." >&2
  exit 1
fi

# Query from Org1 by default
setGlobals 1

echo "Querying ListArtifacts..."
QUERY_CALL='{"Args":["ListArtifacts"]}'
peer chaincode query -C "$CHANNEL_NAME" -n "$CC_NAME" -c "$QUERY_CALL" | jq .

echo "Done."


