#!/usr/bin/env bash

# -----------------------------------------------------------------------------
# Ginkgo variables helpers. These functions need the
# following variables:
#
#    GINKGO_VERSION  -  The ginkgo version, default is v1.13.0.

function autok3s::ginkgo::install() {
  local version=${GINKGO_VERSION:-"v1.13.0"}
  tmp_dir=$(mktemp -d)
  pushd "${tmp_dir}" >/dev/null || exit 1
  go mod init tmp
  GO111MODULE=on go get "github.com/onsi/ginkgo/ginkgo@${version}"
  rm -rf "${tmp_dir}"
  popd >/dev/null || return
}

function autok3s::ginkgo::validate() {
  if [[ -n "$(command -v ginkgo)" ]]; then
    return 0
  fi

  autok3s::log::info "installing ginkgo"
  if autok3s::ginkgo::install; then
    autok3s::log::info "ginkgo: $(ginkgo version)"
    return 0
  fi
  autok3s::log::error "no ginkgo available"
  return 1
}

function autok3s::ginkgo::test() {
  if ! autok3s::ginkgo::validate; then
    autok3s::log::error "cannot execute ginkgo as it hasn't installed"
    return
  fi

  local dir_path="${!#}"
  local arg_idx=0
  for arg in "$@"; do
    if [[ "${arg}" == "--" ]]; then
      dir_path="${!arg_idx}"
      break
    fi
    arg_idx=$((arg_idx + 1))
  done

  if autok3s::util::is_empty_dir "${dir_path}"; then
    autok3s::log::warn "${dir_path} is an empty directory"
    return
  fi

  autok3s::log::info "ginkgo -r -v -trace -tags=test -failFast -slowSpecThreshold=60 -timeout=5m $*"
  CGO_ENABLED=0 ginkgo -r -v -trace -tags=test \
    -failFast -slowSpecThreshold=60 -timeout=5m "$@"
}
