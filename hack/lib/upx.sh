#!/usr/bin/env bash

function autok3s::upx::validate() {
  if [[ -n "$(command -v upx)" ]]; then
    return 0
  fi
  return 1
}

function autok3s::upx::run() {
  if ! autok3s::upx::validate; then
    autok3s::log::warn "upx hasn't been installed, skip compressing binaries"
    return
  fi

  autok3s::log::info "compressing binaries"
  for file in `ls ${ROOT_DIR}/dist`; do
    if [[ "$file" == "autok3s_darwin_arm64" ]]; then
      autok3s::log::info "darwin arm64 binary doesn't work well with upx, skipping compressing this binary."
      continue
    fi
    upx -1 ${ROOT_DIR}/dist/$file;
  done
}

