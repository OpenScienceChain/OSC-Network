# syntax=docker/dockerfile:1

# You can override these at build-time, but the deployCCAAS script
# already passes CC_SERVER_PORT=9999 by default.
ARG GO_VERSION=1.25
ARG CC_SERVER_PORT=9999

# ---- Build stage ----
FROM golang:${GO_VERSION}-bookworm AS build
WORKDIR /workspace

# Pre-cache modules (handle missing go.sum gracefully)
COPY go.mod ./
RUN go mod download

# Copy the rest of the chaincode source
COPY . .

# Build a static-ish binary (no VCS stamping to avoid git required in container)
ENV CGO_ENABLED=0 GOFLAGS="-buildvcs=false"
RUN go build -trimpath -o /out/chaincode .

# ---- Runtime stage ----
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /chaincode
COPY --from=build /out/chaincode /chaincode/chaincode

# The deployCCAAS script sets CORE_CHAINCODE_ID_NAME in the container.
# Expose and bind the chaincode gRPC server to this port/address.
ARG CC_SERVER_PORT
ENV CHAINCODE_SERVER_ADDRESS=0.0.0.0:${CC_SERVER_PORT}

EXPOSE ${CC_SERVER_PORT}

# Run as non-root (arbitrary unprivileged UID/GID)
USER 65532:65532

ENTRYPOINT ["/chaincode/chaincode"]
