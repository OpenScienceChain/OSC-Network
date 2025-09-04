#!/usr/bin/env bash

set -euo pipefail

# Sanity script: pick a random artifact (from first 20), update its footprint 20 times, then fetch history
# Usage:
#   scripts/sanity-artifact-update-history.sh [-c <channelName>] [-n <chaincodeName>] [-u <numUpdates>]
# Defaults:
#   channelName = mychannel
#   chaincodeName = oscis
#   numUpdates = 20

ROOTDIR=$(cd "$(dirname "$0")/.." && pwd)
TESTNET_DIR="${ROOTDIR}/test-network"

CHANNEL_NAME="mychannel"
CC_NAME="oscis"
NUM_UPDATES=20

# Prevent "unbound variable" errors inside sourced test-network scripts
export OVERRIDE_ORG=""
export VERBOSE=false

while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
    -c) CHANNEL_NAME="$2"; shift ;;
    -n) CC_NAME="$2"; shift ;;
    -u) NUM_UPDATES="$2"; shift ;;
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

# Ensure required tools exist
for cmd in jq shuf od tr; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Required tool '$cmd' is not installed. Please install it and retry." >&2
    exit 1
  fi
done

# Query from Org1 by default
setGlobals 1

# Capture Org1 peer address and TLS cert
ORG1_ADDR="$CORE_PEER_ADDRESS"
ORG1_TLS="$CORE_PEER_TLS_ROOTCERT_FILE"

# Capture Org2 peer address and TLS cert (many networks require 2 org endorsements)
setGlobals 2
ORG2_ADDR="$CORE_PEER_ADDRESS"
ORG2_TLS="$CORE_PEER_TLS_ROOTCERT_FILE"

# Restore Org1 as the active CLI context
setGlobals 1

echo "Listing artifacts (first 20) on channel '$CHANNEL_NAME' for chaincode '$CC_NAME'..."
QUERY_CALL='{"Args":["ListArtifacts"]}'
RAW_JSON=$(peer chaincode query -C "$CHANNEL_NAME" -n "$CC_NAME" -c "$QUERY_CALL")

# Normalize artifacts array shape
ARTIFACTS_ARRAY=$(echo "$RAW_JSON" | jq -c 'if type=="array" then . 
  elif (type=="object" and has("records") and (.records|type=="array")) then .records 
  elif (type=="object" and has("items") and (.items|type=="array")) then .items 
  elif (type=="object" and has("result") and (.result|type=="array")) then .result 
  elif (type=="object" and has("data") and (.data|type=="array")) then .data 
  else [] end')

COUNT=$(echo "$ARTIFACTS_ARRAY" | jq 'length')
if [[ "$COUNT" -eq 0 ]]; then
  echo "No artifacts found. Please create at least one artifact first." >&2
  exit 1
fi

LIMITED=$(echo "$ARTIFACTS_ARRAY" | jq '.[0:20]')
echo "Found $COUNT artifacts; sampling from first $(echo "$LIMITED" | jq 'length') entries."

# Pick a random artifact id
ARTIFACT_ID=$(echo "$LIMITED" | jq -r '.[].id' | shuf -n 1)
if [[ -z "$ARTIFACT_ID" || "$ARTIFACT_ID" == "null" ]]; then
  echo "Failed to select an artifact id." >&2
  exit 1
fi

echo "Selected artifact id: $ARTIFACT_ID"

# Helper: generate a 64-char lowercase hex string
generate_hex64() {
  # 32 random bytes -> 64 hex chars
  od -An -tx1 -N32 /dev/urandom | tr -d ' \n'
}

# Build common invoke flags for Org1
ORDERER=localhost:7050
ORDERER_TLS="--ordererTLSHostnameOverride orderer.example.com --tls --cafile $ORDERER_CA"
PEER_ADDRS="--peerAddresses $ORG1_ADDR --tlsRootCertFiles $ORG1_TLS --peerAddresses $ORG2_ADDR --tlsRootCertFiles $ORG2_TLS"

echo "Performing $NUM_UPDATES footprint updates on artifact $ARTIFACT_ID..."
SUCCEEDED=0
FAILED=0
for ((i=1; i<=NUM_UPDATES; i++)); do
  FP=$(generate_hex64)
  BODY_JSON=$(jq -nc --arg fp "$FP" '{footprint:$fp}')
  INVOKE_PAYLOAD=$(jq -nc --arg id "$ARTIFACT_ID" --arg body "$BODY_JSON" '{Args:["UpdateArtifactDetails", $id, $body]}')

  echo "[$i/$NUM_UPDATES] Updating footprint to $FP"
  if peer chaincode invoke -o "$ORDERER" $ORDERER_TLS -C "$CHANNEL_NAME" -n "$CC_NAME" \
    $PEER_ADDRS \
    -c "$INVOKE_PAYLOAD" \
    --waitForEvent >/dev/null 2>&1; then
    SUCCEEDED=$((SUCCEEDED+1))
  else
    echo "Invoke failed for update $i" >&2
    FAILED=$((FAILED+1))
  fi
done

echo "Fetching history for artifact $ARTIFACT_ID..."
HIST_QUERY=$(jq -nc --arg id "$ARTIFACT_ID" '{Args:["GetArtifactHistory", $id]}')
HISTORY_RAW=$(peer chaincode query -C "$CHANNEL_NAME" -n "$CC_NAME" -c "$HIST_QUERY")

echo "History entries:"
echo "$HISTORY_RAW" | jq '. | map({txId, timestamp, isDelete, value: ( .value // {} )})'

# Metrics summary
TOTAL_HISTORY=$(echo "$HISTORY_RAW" | jq 'length')
WRITES_HISTORY=$(echo "$HISTORY_RAW" | jq '[.[] | select(.isDelete == false)] | length')
DELETES_HISTORY=$(echo "$HISTORY_RAW" | jq '[.[] | select(.isDelete == true)] | length')
# Exclude initial create from updates if there was at least one write
UPDATES_HISTORY=$(( WRITES_HISTORY > 0 ? WRITES_HISTORY - 1 : 0 ))

echo
echo "Summary:"
echo "- Artifact id: $ARTIFACT_ID"
echo "- Attempted updates: $NUM_UPDATES"
echo "- Successful updates: $SUCCEEDED"
echo "- Failed updates: $FAILED"
echo "- History entries total: $TOTAL_HISTORY"
echo "- History writes (incl. create): $WRITES_HISTORY"
echo "- History deletes: $DELETES_HISTORY"
echo "- History updates (excluding create): $UPDATES_HISTORY"

echo "Done."


