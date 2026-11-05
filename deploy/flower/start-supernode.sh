#!/usr/bin/env bash
# Starts the Flower SuperNode with the config in /opt/fedsbc/etc/fedsbc.env.
# The fedsbc-supernode service runs it; no need to call it by hand.
set -euo pipefail
PREFIX="${FEDSBC_PREFIX:-/opt/fedsbc}"
set -a
# shellcheck disable=SC1091
source <(tr -d '\r' < "$PREFIX/etc/fedsbc.env")   # accept Windows line endings (CRLF)
set +a

# Flower starts helper processes (flower-superexec) by name: the venv must be on PATH
VENV_BIN="$PREFIX/frameworks/flower/.venv/bin"
export PATH="$VENV_BIN:$PATH"

# ID_DEVICE is the node identity for telemetry and FL
exec "$VENV_BIN/flower-supernode" --insecure \
  --superlink "$SUPERLINK" \
  --node-config "partition-id=${PARTITION_ID} num-partitions=${NUM_PARTITIONS} device-id=\"${ID_DEVICE}\""