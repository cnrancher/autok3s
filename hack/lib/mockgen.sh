#!/usr/bin/env bash

# -----------------------------------------------------------------------------
# Mockgen variables helpers. These functions need the
# following variables:
#
#    MOCKGEN_VERSION  -  The go mockgen version, default is v1.4.3.

function autok3s::mockgen::install() {
  local version=${MOCKGEN_VERSION:-"v1.4.3"}
  tmp_dir=$(mktemp -d)
  pushd "${tmp_dir}" >/dev/null || exit 1
  go mod init tmp
  GO111MODULE=on go get "github.com/golang/mock/mockgen@${version}"
  rm -rf "${tmp_dir}"
  popd >/dev/null || return
}

function autok3s::mockgen::validate() {
  if [[ -n "$(command -v mockgen)" ]]; then
    return 0
  fi

  autok3s::log::info "installing mockgen"
  if autok3s::mockgen::install; then
    autok3s::log::info "mockgen: $(mockgen --version)"
    return 0
  fi
  autok3s::log::error "no mockgen available"
  return 1
}

function autok3s::mockgen::generate_by_source() {
  if ! autok3s::mockgen::validate; then
    autok3s::log::error "cannot execute mockgen as it hasn't installed"
    return
  fi

  local filepath="${1:-}"
  if [[ ! -f ${filepath} ]]; then
    autok3s::log::warn "${filepath} isn't existed"
    return
  fi
  local filedir
  filedir=$(dirname "${filepath}")
  local filename
  filename=$(basename "${filepath}")
  local mocked_dir="${filedir}/mock"
  mkdir -p "${mocked_dir}"

  local mocked_file="${mocked_dir}/${filename}"
  # generate
  mockgen \
    -source="${filepath}" \
    -destination="${mocked_file}"

  # format
  local tmpfile
  tmpfile=$(mktemp)
  sed "2d" "${mocked_file}" >"${tmpfile}" && mv "${tmpfile}" "${mocked_file}"
  cat "${ROOT_DIR}/hack/boilerplate.go.txt" "${mocked_file}" >"${tmpfile}" && mv "${tmpfile}" "${mocked_file}"
  go fmt "${mocked_file}" >/dev/null 2>&1
}
