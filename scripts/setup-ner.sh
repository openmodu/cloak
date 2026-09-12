#!/usr/bin/env bash
# 准备本地 NER 所需的 ONNX Runtime 与模型文件。
#
# 用法：
#   scripts/setup-ner.sh              # 装到 ~/.cloak
#   CLOAK_HOME=/opt/cloak scripts/setup-ner.sh
#   HF_ENDPOINT=https://hf-mirror.com scripts/setup-ner.sh    # 走镜像
#
# 可重复执行：已经存在且大小正确的文件会跳过，中断的下载会续传。

set -euo pipefail

CLOAK_HOME="${CLOAK_HOME:-$HOME/.cloak}"
HF_ENDPOINT="${HF_ENDPOINT:-https://huggingface.co}"
ORT_VERSION="${ORT_VERSION:-1.30.0}"

EN_MODEL="${CLOAK_EN_MODEL_ID:-Xenova/bert-base-NER}"
ZH_MODEL="${CLOAK_ZH_MODEL_ID:-Xenova/bert-base-multilingual-cased-ner-hrl}"

MODELS_DIR="$CLOAK_HOME/models"
LIB_DIR="$CLOAK_HOME/lib"

log() { printf '\033[1m==>\033[0m %s\n' "$*"; }

fetch() { # fetch <url> <目标路径>
  local url="$1" dest="$2"
  if [[ -s "$dest" ]]; then
    echo "    已存在，跳过 $(basename "$dest")"
    return
  fi
  mkdir -p "$(dirname "$dest")"
  curl -fL --retry 10 --retry-all-errors --retry-delay 2 -C - \
    --progress-bar -o "$dest" "$url"
}

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
  for f in config.json vocab.txt tokenizer_config.json; do
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
    ./bin/cloakd

两个模型的标签集须是 PER / ORG / LOC 这一套，换模型时一并确认。
EOF
