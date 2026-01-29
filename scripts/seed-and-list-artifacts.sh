#!/usr/bin/env bash

set -euo pipefail

# Seed the ledger with 3 artifacts via CreateArtifact and then list them via ListArtifacts
# Usage:
#   scripts/seed-and-list-artifacts.sh [-c <channelName>] [-n <chaincodeName>]
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
      sed -n '1,14p' "$0"; exit 0 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
  shift
done

cd "$TESTNET_DIR"

# Load environment and helper functions from test-network
# Temporarily relax nounset for sourcing test-network helpers
set +u
source ./scripts/envVar.sh
source ./scripts/ccutils.sh
set -u

# Ensure jq is available for JSON construction/pretty-printing
if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required but not installed. Please install jq and retry." >&2
  exit 1
fi

# Build peer connection parameters for Org1 and Org2
parsePeerConnectionParameters 1 2

invoke_create() {
  local payload_json="$1"
  local fcn_call
  fcn_call=$(jq -cn --arg arg1 "CreateArtifact" --arg arg2 "$payload_json" '{Args:[$arg1,$arg2]}')

  echo "Invoking CreateArtifact..."
  peer chaincode invoke -o localhost:17050 \
    --ordererTLSHostnameOverride orderer.example.com \
    --tls --cafile "$ORDERER_CA" \
    -C "$CHANNEL_NAME" -n "$CC_NAME" \
    "${PEER_CONN_PARMS[@]}" \
    -c "$fcn_call" >/dev/null
}

make_payload() {
  local id="$1" title="$2" desc="$3" email="$4" user="$5" footprint="$6"
  jq -cn \
    --arg id "$id" \
    --arg title "$title" \
    --arg desc "$desc" \
    --arg email "$email" \
    --arg user "$user" \
    --arg fp "$footprint" \
    '{
      id:$id,
      title:$title,
      description:$desc,
      manifest:[{algorithm:"sha256", filename:"file.txt", hash:$fp}],
      footprint:$fp,
      submitterEmail:$email,
      submitterUsername:$user
    }'
}

# Generate a lowercase UUID v4 (best-effort across environments)
new_uuid() {
  if command -v uuidgen >/dev/null 2>&1; then
    uuidgen | tr '[:upper:]' '[:lower:]'
  elif [[ -r /proc/sys/kernel/random/uuid ]]; then
    cat /proc/sys/kernel/random/uuid
  else
    # Fallback: derive RFC4122-like from random hex
    local h
    if command -v openssl >/dev/null 2>&1; then
      h=$(openssl rand -hex 16)
    else
      h=$(hexdump -n16 -v -e '/1 "%02x"' /dev/urandom)
    fi
    echo "${h:0:8}-${h:8:4}-4${h:12:3}-a${h:15:3}-${h:18:12}"
  fi
}

# Prepare three valid artifacts
DESC1="Sample artifact one description with sufficient length to pass validation."
DESC2="Sample artifact two description with sufficient length to pass validation."
DESC3="Sample artifact three description with sufficient length to pass validation."

ID1=$(new_uuid)
ID2=$(new_uuid)
ID3=$(new_uuid)

PAYLOAD1=$(make_payload \
  "$ID1" \
  "Genome data package 1" \
  "$DESC1" \
  "user1@example.com" \
  "user1" \
  aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa)

PAYLOAD2=$(make_payload \
  "$ID2" \
  "Genome data package 2" \
  "$DESC2" \
  "user2@example.com" \
  "user2" \
  bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb)

PAYLOAD3=$(make_payload \
  "$ID3" \
  "Genome data package 3" \
  "$DESC3" \
  "user3@example.com" \
  "user3" \
  cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc)

# Submit the three artifacts
invoke_create "$PAYLOAD1"
invoke_create "$PAYLOAD2"
invoke_create "$PAYLOAD3"

echo "All CreateArtifact transactions submitted. Waiting 2s before query..."
sleep 2

echo "Querying ListArtifacts..."
QUERY_CALL='{"Args":["ListArtifacts"]}'
peer chaincode query -C "$CHANNEL_NAME" -n "$CC_NAME" -c "$QUERY_CALL" | jq .

echo "Done."


