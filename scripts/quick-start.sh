#!/usr/bin/env bash

# Quick setup for Fabric test network with Fabric CAs, CouchDB, and a new channel
# Usage:
#   scripts/quick-start.sh [-c <channelName>] [-i <fabricTag>] [-cai <caTag>] [-bft]
# Defaults:
#   channelName = mychannel

set -euo pipefail

ROOTDIR=$(cd "$(dirname "$0")/.." && pwd)
TESTNET_DIR="${ROOTDIR}/test-network"

CHANNEL_NAME="mychannel"
BFT="0"

while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
    -c)
      CHANNEL_NAME="$2"; shift ;;
    -bft)
      BFT="1" ;;
    -i)
      FABRIC_TAG="$2"; shift ;;
    -cai)
      CA_TAG="$2"; shift ;;
    -h|--help)
      echo "Usage: scripts/quick-start.sh [-c <channelName>] [-i <fabricTag>] [-cai <caTag>] [-bft]";
      exit 0 ;;
    *)
      echo "Unknown option: $1"; exit 1 ;;
  esac
  shift
done

cd "${TESTNET_DIR}"

# Bring up network with Fabric CAs and CouchDB, then create channel
if [[ -n "${FABRIC_TAG:-}" ]]; then
  FABRIC_TAG_ARG=( -i "$FABRIC_TAG" )
else
  FABRIC_TAG_ARG=()
fi
if [[ -n "${CA_TAG:-}" ]]; then
  CA_TAG_ARG=( -cai "$CA_TAG" )
else
  CA_TAG_ARG=()
fi

./network.sh up -ca -s couchdb "${FABRIC_TAG_ARG[@]}" "${CA_TAG_ARG[@]}"

# Build arguments for channel creation
CREATE_ARGS=( createChannel -c "$CHANNEL_NAME" )
if [[ "$BFT" == "1" ]]; then
  CREATE_ARGS+=( -bft )
fi
./network.sh "${CREATE_ARGS[@]}"

echo "Network is up with Fabric CAs and CouchDB. Channel '$CHANNEL_NAME' created."


