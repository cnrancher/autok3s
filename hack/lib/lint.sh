#!/usr/bin/env bash

# -----------------------------------------------------------------------------
# Lint variables helpers. These functions need the
# following variables:
#
#    GOLANGCI_LINT_VERSION  -  The golangci-lint version, default is v1.27.0.
#    DIRTY_CHECK            -  Specify to check the git tree is dirty or not.

function autok3s::lint::install() {
  local version=${GOLANGCI_LINT_VERSION:-"v1.27.0"}
  curl -fL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(go env GOPATH)/bin" "${version}"
}

function autok3s::lint::validate() {
  if [[ -n "$(command -v golangci-lint)" ]]; then
    return 0
  fi

  autok3s::log::info "installing golangci-lint"
  if autok3s::lint::install; then
    autok3s::log::info "$(golangci-lint --version)"
    return 0
  fi
  return 1
}

function autok3s::lint::lint() {
  if [[ "${DIRTY_CHECK:-}" == "true" ]]; then
    if [[ "${GIT_TREE_STATE}" == "dirty" ]]; then
      autok3s::log::fatal "the git tree is dirty:\n$(git status --porcelain)"
    fi
  fi

  if autok3s::lint::validate; then
    for path in "$@"; do
      golangci-lint run --timeout 10m "${path}"
    done
  else
    autok3s::log::warn "no golangci-lint available, using go fmt/vet instead"
    for path in "$@"; do
      go fmt "${path}"
      go vet -tags=test "${path}"
    done
  fi
}
