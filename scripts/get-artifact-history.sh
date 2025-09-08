#!/usr/bin/env bash

set -euo pipefail
set +x

# Fetch and pretty-print full history of an artifact by id, and count updates by submissionState
# Usage:
#   scripts/get-artifact-history.sh <artifact-id> [-c <channel>] [-n <ccName>] [-o <orgNum>]
# Defaults: channel=mychannel, ccName=oscis, orgNum=1

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <artifact-id> [-c <channel>] [-n <ccName>]" >&2
  exit 1
fi

ARTIFACT_ID="$1"; shift || true

CHANNEL_NAME="mychannel"
CC_NAME="oscis"
ORG_NUM="1"

while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
    -c) CHANNEL_NAME="$2"; shift ;;
    -n) CC_NAME="$2"; shift ;;
    -o) ORG_NUM="$2"; shift ;;
    -h|--help)
      sed -n '1,40p' "$0"; exit 0 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
  shift
done

ROOTDIR=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOTDIR/test-network"

# Load Fabric env and set peer context for the chosen org
# Provide defaults expected by test-network scripts
export OVERRIDE_ORG=""
export VERBOSE=false
set +u
source ./scripts/envVar.sh
set -u
setGlobals "$ORG_NUM"

# Query chaincode history function
RAW=$(peer chaincode query -C "$CHANNEL_NAME" -n "$CC_NAME" -c "{\"Args\":[\"GetArtifactHistory\",\"$ARTIFACT_ID\"]}")

echo "History for artifact $ARTIFACT_ID:"
echo "$RAW" | jq '.'

echo
echo "Counts:"
SUCCESS=$(echo "$RAW" | jq '[.[] | select(.value and .value.submissionState=="SUCCESS")] | length')
FAILED=$(echo "$RAW" | jq '[.[] | select(.value and .value.submissionState=="FAILED")] | length')
PENDING=$(echo "$RAW" | jq '[.[] | select(.value and .value.submissionState=="PENDING")] | length')
UPDATES=$(echo "$RAW" | jq '[.[] | select(.value)] | length')
DELETES=$(echo "$RAW" | jq '[.[] | select(.isDelete==true)] | length')

printf 'updates=%s success=%s failed=%s pending=%s deletes=%s\n' "$UPDATES" "$SUCCESS" "$FAILED" "$PENDING" "$DELETES"


