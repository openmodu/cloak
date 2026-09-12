#!/usr/bin/env bash
# Sourced by setup-ner.sh and exercised against a local HTTP server in tests.
# Partial downloads are never considered installed; interrupted transfers restart.
fetch() (
  set -euo pipefail
  local url="$1" dest="$2" digest
  mkdir -p "$(dirname "$dest")"
  exec 9>"$dest.lock"
  flock 9
  if [[ -f "$dest" && -f "$dest.sha256" && -f "$dest.url" ]] &&
     [[ "$(<"$dest.url")" == "$url" ]] &&
     (cd "$(dirname "$dest")" && sha256sum --check --status "$(basename "$dest").sha256"); then
    echo "    已校验，跳过 $(basename "$dest")"
    return
  fi
  curl -fL --retry "${CLOAK_DOWNLOAD_RETRIES:-3}" --retry-all-errors --retry-delay 2 \
    --connect-timeout 20 --progress-bar -o "$dest.part" "$url"
  [[ -s "$dest.part" ]] || { echo "下载文件为空: $url" >&2; return 1; }
  digest="$(sha256sum "$dest.part")"
  printf '%s  %s\n' "${digest%% *}" "$(basename "$dest")" >"$dest.sha256.part"
  printf '%s' "$url" >"$dest.url.part"
  mv -f -- "$dest.part" "$dest"
  mv -f -- "$dest.sha256.part" "$dest.sha256"
  mv -f -- "$dest.url.part" "$dest.url"
)
