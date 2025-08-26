#!/usr/bin/env bash

# Deploy or update chaincode-as-a-service for the OSC-IS-CC Go chaincode
# Usage examples:
#   scripts/deploy-ccaas.sh -c mychannel -n oscis -s auto
#   scripts/deploy-ccaas.sh -c mychannel -n oscis -v 1.1 -s 2
# Flags:
#   -c  Channel name (default: mychannel)
#   -n  Chaincode name (default: oscis)
#   -p  Chaincode path (default: OSC-IS-CC/chaincode-go)
#   -v  Chaincode version (default: 1.0)
#   -s  Chaincode sequence (default: auto)
#   -i  Init function (default: NA)
#   -P  Endorsement policy (default: NA)
#   -g  Collections config file (default: NA)
#   --no-build  Skip Docker image build and container restart
#   --no-run    Build image but do not run CCAAS containers

set -euo pipefail

ROOTDIR=$(cd "$(dirname "$0")/.." && pwd)
TESTNET_DIR="${ROOTDIR}/test-network"
DEFAULT_CC_PATH="${ROOTDIR}/OSC-IS-CC/chaincode-go"

CHANNEL_NAME="mychannel"
CC_NAME="oscis"
CC_SRC_PATH="$DEFAULT_CC_PATH"
CC_VERSION="1.0"
CC_SEQUENCE="auto"
CC_INIT_FCN="NA"
CC_END_POLICY="NA"
CC_COLL_CONFIG="NA"
BUILD_IMAGE=true
RUN_CONTAINERS=true

while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
    -c) CHANNEL_NAME="$2"; shift ;;
    -n) CC_NAME="$2"; shift ;;
    -p) CC_SRC_PATH="$2"; shift ;;
    -v) CC_VERSION="$2"; shift ;;
    -s) CC_SEQUENCE="$2"; shift ;;
    -i) CC_INIT_FCN="$2"; shift ;;
    -P) CC_END_POLICY="$2"; shift ;;
    -g) CC_COLL_CONFIG="$2"; shift ;;
    --no-build) BUILD_IMAGE=false ;;
    --no-run) RUN_CONTAINERS=false ;;
    -h|--help)
      sed -n '2,25p' "$0"; exit 0 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
  shift
done

if [[ ! -d "$CC_SRC_PATH" ]]; then
  echo "Chaincode path not found: $CC_SRC_PATH" >&2
  exit 1
fi

cd "$TESTNET_DIR"

# Build and/or run flags for deployCCAAS
CCAAS_DOCKER_RUN=true
if [[ "$BUILD_IMAGE" == false && "$RUN_CONTAINERS" == false ]]; then
  CCAAS_DOCKER_RUN=false
fi

# Best-effort stop existing CCAAS containers so package ID can change cleanly
if [[ "$RUN_CONTAINERS" == true ]]; then
  ${CONTAINER_CLI:-docker} rm -f "peer0org1_${CC_NAME}_ccaas" >/dev/null 2>&1 || true
  ${CONTAINER_CLI:-docker} rm -f "peer0org2_${CC_NAME}_ccaas" >/dev/null 2>&1 || true
fi

# Deploy or update via test-network helper (handles install/approve/commit)
./network.sh deployCCAAS \
  -c "$CHANNEL_NAME" \
  -ccn "$CC_NAME" \
  -ccp "$CC_SRC_PATH" \
  -ccv "$CC_VERSION" \
  -ccs "$CC_SEQUENCE" \
  -cci "$CC_INIT_FCN" \
  -ccep "$CC_END_POLICY" \
  -cccg "$CC_COLL_CONFIG" \
  -ccaasdocker "$CCAAS_DOCKER_RUN"

echo "CCAAS deployment complete for '$CC_NAME' on channel '$CHANNEL_NAME'."
echo "Version: $CC_VERSION  Sequence: $CC_SEQUENCE  Path: $CC_SRC_PATH"


