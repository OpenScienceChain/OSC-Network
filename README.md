# OSC-Network

> The **test / development Hyperledger Fabric network** for OSC-IS: a disposable local Fabric network used to deploy and exercise [OSC-Chaincode](../OSC-Chaincode) without touching production.

This repository holds the Hyperledger Fabric artifacts necessary to spin up **test networks** to validate chaincode deployments. It is the lightweight, throwaway counterpart to [OSC-Docker](../OSC-Docker), which provisions the *production* network.

---

## Executive summary

Changing a smart contract is risky: a bad deployment to the production ledger is expensive and hard to undo. OSC-Network exists so that chaincode changes can be deployed and tested against a **real but ephemeral** Fabric network first — on a developer machine or a simple AWS test host — and torn down freely.

It is derived from Hyperledger's reference `test-network`, adapted for the OSC-IS chaincode. Both a Docker-Compose (`test-network`) and a Kubernetes (`test-network-k8s`) variant are included, along with the OSC chaincode packaging (`OSC-IS-CC`) and helper scripts.

> **Production vs. test.** This network is **not** the production OSC-IS ledger. Use it for development and CI-style validation; the production network is provisioned by [OSC-Docker](../OSC-Docker).

---

## What's here

| Path | Purpose |
|---|---|
| `test-network/` | Docker-Compose based local Fabric test network (peers, orderer, CAs, channel scripts). |
| `test-network-k8s/` | Kubernetes variant of the test network. |
| `OSC-IS-CC/` | OSC-IS chaincode packaging/deployment assets for the test network. |
| `scripts/`, `ci/` | Helper and CI automation scripts. |
| `config/` | Fabric configuration (core, orderer, configtx). |
| `wallets/` | Test identity material (development only — not production credentials). |

---

## Typical use

```bash
# Bring up a local test network and deploy the OSC chaincode, then run
# OSC-Chaincode tests or point a local OSC-API/adapter at it.
cd test-network
./network.sh up createChannel
./network.sh deployCC -ccn osc-is -ccp <path-to-OSC-Chaincode> -ccl go
# ... exercise the chaincode ...
./network.sh down
```

> On Windows, run under **WSL** — the Fabric test-network tooling is far smoother there than in plain PowerShell.

This gives a real Fabric environment (orderer, peers, CouchDB, channel, deployed chaincode) for end-to-end validation of submission/history flows without cloud cost or production risk.

---

## Documentation

| Document | Contents |
|---|---|
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | How the test network is structured, how it relates to production, and its role in the OSC-IS test strategy. |

---

*Part of the OSC-IS platform: [OSC-Chaincode](../OSC-Chaincode) · [OSC-Docker](../OSC-Docker) · [OSC-API](../OSC-API) · [OSC-APIGateway](../OSC-APIGateway) · [OSC-Artifact-Submission](../OSC-Artifact-Submission) · [OSC-WebApp](../OSC-WebApp) · [OSC-IS-Infra](../OSC-IS-Infra).*
