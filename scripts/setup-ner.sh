#!/usr/bin/env bash
# 准备本地 NER 所需的 ONNX Runtime 与模型文件。
#
# 用法：
#   scripts/setup-ner.sh              # 装到 ~/.cloak
#   CLOAK_HOME=/opt/cloak scripts/setup-ner.sh
#   HF_ENDPOINT=https://hf-mirror.com scripts/setup-ner.sh    # 走镜像
#
# 可重复执行：通过完整性校验的文件才跳过；中断的下载会重新下载。

set -euo pipefail

CLOAK_HOME="${CLOAK_HOME:-$HOME/.cloak}"
HF_ENDPOINT="${HF_ENDPOINT:-https://huggingface.co}"
ORT_VERSION="${ORT_VERSION:-1.30.0}"

# These repositories provide ready-to-use ONNX exports. Runtime defaults remain
# the built-in model IDs; export the selected IDs below when launching.
EN_MODEL="${CLOAK_EN_MODEL_ID:-Xenova/bert-base-NER}"
ZH_MODEL="${CLOAK_ZH_MODEL_ID:-Xenova/bert-base-multilingual-cased-ner-hrl}"

MODELS_DIR="$CLOAK_HOME/models"
LIB_DIR="$CLOAK_HOME/lib"

log() { printf '\033[1m==>\033[0m %s\n' "$*"; }

source "$(dirname "${BASH_SOURCE[0]}")/download.sh"

log "安装目录：$CLOAK_HOME"

# ---- ONNX Runtime ----
if [[ -f "$LIB_DIR/libonnxruntime.so" ]]; then
  log "ONNX Runtime 已就位，跳过"
else
  log "下载 ONNX Runtime $ORT_VERSION"
  TMP="$(mktemp -d)"
  trap 'rm -rf "$TMP"' EXIT
  fetch "https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VERSION}/onnxruntime-linux-x64-${ORT_VERSION}.tgz" "$TMP/ort.tgz"
  tar xf "$TMP/ort.tgz" -C "$TMP"
  mkdir -p "$LIB_DIR"
  cp "$TMP"/onnxruntime-linux-x64-*/lib/libonnxruntime.so* "$LIB_DIR/"
fi

# ---- 模型 ----
for model in "$EN_MODEL" "$ZH_MODEL"; do
  log "下载模型 $model"
  dest="$MODELS_DIR/$model"
  for f in config.json vocab.txt tokenizer_config.json tokenizer.json; do
    fetch "$HF_ENDPOINT/$model/resolve/main/$f" "$dest/$f"
  done
  fetch "$HF_ENDPOINT/$model/resolve/main/onnx/model_quantized.onnx" "$dest/onnx/model_quantized.onnx"
done

cat <<EOF

$(log "完成")

    模型目录：$MODELS_DIR
    动态库：  $LIB_DIR/libonnxruntime.so

用法（需要带 ONNX 支持编译的二进制）：

    CGO_ENABLED=1 go build -tags cloak_onnx -o bin/cloakd ./cmd/cloakd

    export CLOAK_ONNXRUNTIME_LIB=$LIB_DIR/libonnxruntime.so
    export CLOAK_MODELS_DIR=$MODELS_DIR
    export CLOAK_EN_MODEL_ID=$EN_MODEL
    export CLOAK_ZH_MODEL_ID=$ZH_MODEL
    ./bin/cloakd

两个模型的标签集须是 PER / ORG / LOC 这一套，换模型时一并确认。
EOF
