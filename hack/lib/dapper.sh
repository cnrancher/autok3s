#!/usr/bin/env bash

# -----------------------------------------------------------------------------
# Dapper variables helpers. These functions need the
# following variables:
#
#    DAPPER_VERSION   -  The dapper version for running, default is v0.5.8.

function autok3s::dapper::install() {
  local version=${DAPPER_VERSION:-"v0.5.8"}
  curl -fL "https://github.com/rancher/dapper/releases/download/${version}/dapper-$(uname -s)-$(uname -m)" -o ./.dapper
  chmod +x ./.dapper
}

function autok3s::dapper::validate() {
  if [ -x "./.dapper" ]; then
    return 0
  fi

  autok3s::log::info "installing dapper"
  if autok3s::dapper::install; then
    autok3s::log::info "dapper: $(dapper -v)"
    return 0
  fi
  autok3s::log::error "no dapper available"
  return 1
}

function autok3s::dapper::run() {
  if ! autok3s::docker::validate; then
    autok3s::log::fatal "docker hasn't been installed"
  fi
  if ! autok3s::dapper::validate; then
    autok3s::log::fatal "dapper hasn't been installed"
  fi

  ./.dapper "$@"
}
