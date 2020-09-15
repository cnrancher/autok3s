#!/usr/bin/env bash

function autok3s::util::find_subdirs() {
  local path="$1"
  if [[ -z "$path" ]]; then
    path="./"
  fi
  # shellcheck disable=SC2010
  ls -l "$path" | grep "^d" | awk '{print $NF}'
}

function autok3s::util::is_empty_dir() {
  local path="$1"
  if [[ ! -d "${path}" ]]; then
    return 0
  fi

  # shellcheck disable=SC2012
  if [[ $(ls "${path}" | wc -l) -eq 0 ]]; then
    return 0
  fi
  return 1
}

function autok3s::util::join_array() {
  local IFS="$1"
  shift 1
  echo "$*"
}

function autok3s::util::get_os() {
  local os
  if go env GOOS >/dev/null 2>&1; then
    os=$(go env GOOS)
  else
    os=$(echo -n "$(uname -s)" | tr '[:upper:]' '[:lower:]')
  fi

  case ${os} in
  cygwin_nt*) os="windows" ;;
  mingw*) os="windows" ;;
  msys_nt*) os="windows" ;;
  esac

  echo -n "${os}"
}

function autok3s::util::get_arch() {
  local arch
  if go env GOARCH >/dev/null 2>&1; then
    arch=$(go env GOARCH)
    if [[ "${arch}" == "arm" ]]; then
      arch="${arch}v$(go env GOARM)"
    fi
  else
    arch=$(uname -m)
  fi

  case ${arch} in
  armv5*) arch="armv5" ;;
  armv6*) arch="armv6" ;;
  armv7*)
    if [[ "${1:-}" == "--full-name" ]]; then
      arch="armv7"
    else
      arch="arm"
    fi
    ;;
  aarch64) arch="arm64" ;;
  x86) arch="386" ;;
  i686) arch="386" ;;
  i386) arch="386" ;;
  x86_64) arch="amd64" ;;
  esac

  echo -n "${arch}"
}
