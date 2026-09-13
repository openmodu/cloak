#!/usr/bin/env bash
# Start the local web workspace with NER and address masking enabled.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
export CLOAK_ONNXRUNTIME_LIB="${CLOAK_ONNXRUNTIME_LIB:-$HOME/.cloak/lib/libonnxruntime.so}"
if [[ ! -f "$CLOAK_ONNXRUNTIME_LIB" ]]; then
  echo "ONNX Runtime is missing. Run bash scripts/setup-ner.sh first." >&2
  exit 1
fi
CGO_ENABLED=1 go build -tags cloak_onnx -o bin/cloakd-onnx ./cmd/cloakd
exec ./bin/cloakd-onnx --config configs/cloak.yaml --host 127.0.0.1 "$@"
