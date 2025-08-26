#!/usr/bin/env bash

set -euo pipefail
# Ensure secrets/args are not echoed even if called with bash -x
set +x

# Create service wallets for all orgs in the Fabric test network.
# For each org (orgN), this script ensures a service identity (svc-orgN) exists:
# - Registers svc-orgN with the org CA (if missing)
# - Enrolls svc-orgN to produce MSP materials
# - Writes a filesystem wallet identity JSON to ./wallets/<MSP_ID>/svc-orgN.id
#
# Assumptions (Fabric test-network layout):
# - Peer orgs under: ./test-network/organizations/peerOrganizations
# - CAs under:       ./test-network/organizations/fabric-ca
# - CA endpoints:    org1 → localhost:7054, org2 → localhost:8054, org3 → 9054, ...
# - Admin user exists at users/Admin@orgN.example.com
#
# Usage:
#   scripts/create-service-wallets.sh [-w <walletDir>] [-v]
# Default walletDir = ./wallets

ROOTDIR=$(cd "$(dirname "$0")/.." && pwd)
PEER_ORGS_DIR="$ROOTDIR/test-network/organizations/peerOrganizations"
CA_ROOT_DIR="$ROOTDIR/test-network/organizations/fabric-ca"
WALLETS_DIR="$ROOTDIR/wallets"
VERBOSE=false

while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
    -w) WALLETS_DIR="$2"; shift ;;
    -v|--verbose) VERBOSE=true ;;
    -h|--help)
      sed -n '1,50p' "$0"; exit 0 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
  shift
done

need_tool() { command -v "$1" >/dev/null 2>&1 || { echo "Missing required tool: $1" >&2; exit 1; }; }
need_tool fabric-ca-client
need_tool jq

mkdir -p "$WALLETS_DIR"

titlecase() { awk '{print toupper(substr($0,1,1)) tolower(substr($0,2))}' <<<"$1"; }

derive_msp_id() {
  local org_domain="$1"; local base=${org_domain%%.*}
  if [[ "$base" =~ ^org([0-9]+)$ ]]; then
    echo "Org${BASH_REMATCH[1]}MSP"
  else
    echo "$(titlecase "$base")MSP"
  fi
}

derive_org_num() {
  local org_domain="$1"; local base=${org_domain%%.*}
  if [[ "$base" =~ ^org([0-9]+)$ ]]; then echo "${BASH_REMATCH[1]}"; else echo "1"; fi
}

# Generate a strong random secret unless provided via env SVC_SECRET_ORG<N>
choose_secret() {
  local org_num="$1"; local env_var="SVC_SECRET_ORG${org_num}"
  local val="${!env_var-}"
  if [[ -n "${val:-}" ]]; then
    echo "$val"
  else
    # Deterministic, URL-safe default to avoid enroll mismatch if already registered
    echo "svc-org${org_num}pw"
  fi
}

create_identity_json() {
  local msp_dir="$1" msp_id="$2" label="$3" out_dir="$4"
  local cert_file key_file
  cert_file=$(ls -1 "$msp_dir/signcerts"/*.pem 2>/dev/null | head -n1 || true)
  key_file=$(ls -1 "$msp_dir/keystore"/* 2>/dev/null | head -n1 || true)
  if [[ -z "${cert_file:-}" || -z "${key_file:-}" ]]; then
    echo "Skipping $label ($msp_id): missing cert or key in $msp_dir" >&2
    return 1
  fi
  local cert key
  cert=$(cat "$cert_file")
  key=$(cat "$key_file")
  mkdir -p "$out_dir"
  jq -n \
    --arg cert "$cert" \
    --arg key "$key" \
    --arg msp "$msp_id" \
    '{credentials:{certificate:$cert, privateKey:$key}, mspId:$msp, type:"X.509"}' \
    > "$out_dir/$label.id"
}

num_wallets=0

# Command redirection helper
if $VERBOSE; then
  REDIR=""
else
  REDIR=">/dev/null 2>&1"
fi

ensure_admin_enrolled() {
  local org_num="$1" org_home="$2" ca_port="$3" ca_name="$4" ca_tls_cert="$5"
  if [[ -f "$org_home/msp/signcerts/cert.pem" ]] || ls -1 "$org_home/msp/signcerts"/*.pem >/dev/null 2>&1; then
    $VERBOSE && echo "CA admin already enrolled for org${org_num} at $org_home/msp"
    return 0
  fi
  echo "Enrolling CA admin (admin:adminpw) for org${org_num}"
  export FABRIC_CA_CLIENT_HOME="$org_home"
  export FABRIC_CA_CLIENT_TLS_CERTFILES="$ca_tls_cert"
  eval "fabric-ca-client enroll -u https://admin:adminpw@localhost:${ca_port} --caname $ca_name -M \"$org_home/msp\" --tls.certfiles \"$ca_tls_cert\" $REDIR"
}

for org_path in "$PEER_ORGS_DIR"/*; do
  [[ -d "$org_path" ]] || continue
  org_domain=$(basename "$org_path")                    # org1.example.com
  org_num=$(derive_org_num "$org_domain")               # 1, 2, ...
  msp_id=$(derive_msp_id "$org_domain")
  ca_dir="$CA_ROOT_DIR/org${org_num}"
  ca_tls_cert="$ca_dir/tls-cert.pem"
  ca_name="ca-org${org_num}"
  # test-network ports: 7054 + (n-1)*1000
  ca_port=$((7054 + (org_num-1)*1000))
  ca_url="https://localhost:${ca_port}"

  org_home="$org_path"
  users_dir="$org_path/users"
  svc_name="svc-org${org_num}"
  svc_user_dir="$users_dir/${svc_name}@${org_domain}"
  svc_secret=$(choose_secret "$org_num")

  # Ensure admin MSP exists; required for register
  ensure_admin_enrolled "$org_num" "$org_home" "$ca_port" "$ca_name" "$ca_tls_cert"

  if [[ ! -f "$ca_tls_cert" ]]; then
    echo "CA TLS cert not found for $org_domain at $ca_tls_cert; skipping" >&2
    continue
  fi
  if [[ -d "$svc_user_dir/msp" ]]; then
    echo "Service user already enrolled for $org_domain: $svc_user_dir"
  else
    echo "Registering service user $svc_name with $ca_name at $ca_url"
    export FABRIC_CA_CLIENT_HOME="$org_home"
    export FABRIC_CA_CLIENT_URL="$ca_url"
    export FABRIC_CA_CLIENT_TLS_CERTFILES="$ca_tls_cert"
    eval "fabric-ca-client register --caname \"$ca_name\" --id.name \"$svc_name\" --id.secret \"$svc_secret\" --id.type client --id.affiliation \"org${org_num}.department1\" --tls.certfiles \"$ca_tls_cert\" $REDIR" || true

    echo "Enrolling service user $svc_name"
    eval "fabric-ca-client enroll -u https://$svc_name:$svc_secret@localhost:${ca_port} --caname $ca_name -M \"$svc_user_dir/msp\" --tls.certfiles \"$ca_tls_cert\" $REDIR"

    # Verify enrollment produced MSP materials
    if [[ ! -f "$svc_user_dir/msp/signcerts/cert.pem" ]] && [[ -z $(ls -1 "$svc_user_dir/msp/signcerts"/*.pem 2>/dev/null | head -n1) ]]; then
      echo "Enrollment did not produce signcerts for $svc_name at $svc_user_dir/msp. Trying a fresh service user..." >&2
      # Fallback: create a fresh service user label to avoid unknown prior secret
      fallback_label="svc-org${org_num}-$(date +%s)"
      fallback_secret="${fallback_label}pw"
      fallback_user_dir="$users_dir/${fallback_label}@${org_domain}"
      echo "Registering fallback user $fallback_label with $ca_name"
      eval "fabric-ca-client register --caname \"$ca_name\" --id.name \"$fallback_label\" --id.secret \"$fallback_secret\" --id.type client --id.affiliation \"org${org_num}.department1\" --tls.certfiles \"$ca_tls_cert\" $REDIR" || true
      echo "Enrolling fallback user $fallback_label"
      eval "fabric-ca-client enroll -u https://$fallback_label:$fallback_secret@localhost:${ca_port} --caname $ca_name -M \"$fallback_user_dir/msp\" --tls.certfiles \"$ca_tls_cert\" $REDIR" || true
      if [[ -f "$fallback_user_dir/msp/signcerts/cert.pem" ]] || ls -1 "$fallback_user_dir/msp/signcerts"/*.pem >/dev/null 2>&1; then
        svc_name="$fallback_label"
        svc_user_dir="$fallback_user_dir"
        $VERBOSE && echo "Fallback enrollment succeeded for $svc_name"
      else
        echo "Fallback enrollment also failed for org${org_num}. Skipping wallet creation for this org." >&2
        continue
      fi
    fi
  fi

  out_dir="$WALLETS_DIR/$msp_id"
  echo "Writing wallet identity for $msp_id as $svc_name → $out_dir/$svc_name.id"
  if create_identity_json "$svc_user_dir/msp" "$msp_id" "$svc_name" "$out_dir"; then
    ((num_wallets++))
    $VERBOSE && echo "Wrote wallet: $out_dir/$svc_name.id"
  fi
done

echo "Service wallets created: $num_wallets (under $WALLETS_DIR)"


