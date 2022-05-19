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
  upx ${ROOT_DIR}/dist/autok3s_*
}

