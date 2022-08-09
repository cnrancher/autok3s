#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

CURR_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

# The root of the autok3s directory
ROOT_DIR="${CURR_DIR}"
CROSS=${CROSS:-}
UI_VERSION="v0.5.2-rc2"

source "${ROOT_DIR}/hack/lib/init.sh"
source "${CURR_DIR}/hack/lib/constant.sh"

mkdir -p "${CURR_DIR}/bin"
mkdir -p "${CURR_DIR}/dist"

function mod() {
  [[ "${1:-}" != "only" ]]
  pushd "${ROOT_DIR}" >/dev/null || exist 1
  autok3s::log::info "downloading dependencies for autok3s..."

  if [[ "$(go env GO111MODULE)" == "off" ]]; then
    autok3s::log::warn "go mod has been disabled by GO111MODULE=off"
  else
    autok3s::log::info "tidying"
    go mod tidy
  fi

  autok3s::log::info "...done"
  popd >/dev/null || return
}

function ui() {
  autok3s::log::info "downloading autok3s ui"
  cd pkg/server/ui
  curl -sL https://autok3s-ui.s3-ap-southeast-2.amazonaws.com/${UI_VERSION}.tar.gz | tar xvzf -
  cd ${CURR_DIR}
}

function lint() {
  [[ "${1:-}" != "only" ]] && mod
  autok3s::log::info "linting autok3s..."

  local targets=(
    "${CURR_DIR}/cmd/..."
    "${CURR_DIR}/pkg/..."
    "${CURR_DIR}/test/..."
  )
  autok3s::lint::lint "${targets[@]}"

  autok3s::log::info "...done"
}

function build() {
  [[ "${1:-}" != "only" ]] && lint
  ui
  autok3s::log::info "building autok3s(${GIT_VERSION},${GIT_COMMIT},${GIT_TREE_STATE},${BUILD_DATE})..."

  local version_flags="
    -X main.gitVersion=${GIT_VERSION}
    -X main.gitCommit=${GIT_COMMIT}
    -X main.buildDate=${BUILD_DATE}
    -X k8s.io/client-go/pkg/version.gitVersion=${GIT_VERSION}
    -X k8s.io/client-go/pkg/version.gitCommit=${GIT_COMMIT}
    -X k8s.io/client-go/pkg/version.gitTreeState=${GIT_TREE_STATE}
    -X k8s.io/client-go/pkg/version.buildDate=${BUILD_DATE}
    -X k8s.io/component-base/version.gitVersion=${GIT_VERSION}
    -X k8s.io/component-base/version.gitCommit=${GIT_COMMIT}
    -X k8s.io/component-base/version.gitTreeState=${GIT_TREE_STATE}
    -X k8s.io/component-base/version.buildDate=${BUILD_DATE}"
  local flags="
    -w -s"
  local ext_flags="
    -extldflags '-static'"

  local platforms=""
  if [ -z "${CROSS}" ]; then 
    local os="${OS:-$(go env GOOS)}"
    local arch="${ARCH:-$(go env GOARCH)}"
    platforms="$os/$arch"
  else
    autok3s::log::info "crossed building"
    platforms=("${SUPPORTED_PLATFORMS[@]}")
  fi

  for platform in "${platforms[@]}"; do
    autok3s::log::info "building ${platform}"

    local os="${platform%/*}"
    local arch="${platform#*/}"
    local suffix=""
    if [[ "$os" == "windows" ]]; then
      suffix=".exe"
    fi
    export GOARM=
    if [[ "$arch" == "arm" && "$os" != "windows" ]]; then
      export GOARM=7
    fi

    GOOS=${os} GOARCH=${arch} CGO_ENABLED=0 go build \
      -ldflags "${version_flags} ${flags} ${ext_flags}" \
      -tags netgo,prod \
      -o "${CURR_DIR}/bin/${platform}/autok3s${suffix}" \
      "${CURR_DIR}/main.go"
    cp -f "${CURR_DIR}/bin/${platform}/autok3s${suffix}" "${CURR_DIR}/dist/autok3s_${os}_${arch}${suffix}"
  done

  [[ "${1:-}" != "only" ]] && autok3s::upx::run

  autok3s::log::info "...done"
}

function package() {
  [[ "${1:-}" != "only" ]] && build
  autok3s::log::info "packaging autok3s..."

  REPO=${REPO:-cnrancher}
  TAG=${TAG:-${GIT_VERSION}}

  local os="$(go env GOOS)"
  if [[ "$os" == "windows" || "$os" == "darwin" ]]; then
    autok3s::log::warn "package into Darwin/Windows OS image is unavailable, use GOOS=linux GOARCH=amd64 env to containerize linux/amd64 image"
    return
  fi

  ARCH=${ARCH:-$(go env GOARCH)}
  SUFFIX="-linux-${ARCH}"
  IMAGE_NAME=${REPO}/autok3s:${TAG}${SUFFIX}

  docker build --build-arg TARGETARCH=${ARCH} -t ${IMAGE_NAME} .

  autok3s::log::info "...done"
}

function unit() {
  [[ "${1:-}" != "only" ]] && build
  autok3s::log::info "running unit tests for autok3s..."

  local unit_test_targets=(
    "${CURR_DIR}/cmd/..."
    "${CURR_DIR}/pkg/..."
  )

  if [[ "${CROSS:-false}" == "true" ]]; then
    autok3s::log::warn "crossed test is not supported"
  fi

  local os="${OS:-$(go env GOOS)}"
  local arch="${ARCH:-$(go env GOARCH)}"
  local race_tag="-race"
  if [[ "${arch}" == "arm" ]]; then
    # NB(thxCode): race detector doesn't support `arm` arch, ref to:
    # - https://golang.org/doc/articles/race_detector.html#Supported_Systems
    race_tag=""
  fi
    GOOS=${os} GOARCH=${arch} CGO_ENABLED=1 go test \
      -tags=test \
      ${race_tag} \
      -cover -coverprofile "${CURR_DIR}/dist/coverage_${os}_${arch}.out" \
      "${unit_test_targets[@]}"

  autok3s::log::info "...done"
}

function verify() {
  [[ "${1:-}" != "only" ]] && unit
  autok3s::log::info "running integration tests for autok3s..."

  autok3s::ginkgo::test "${CURR_DIR}/test/integration"

  autok3s::log::info "...done"
}

function e2e() {
  [[ "${1:-}" != "only" ]] && verify
  autok3s::log::info "running E2E tests for autok3s..."

  # execute the E2E testing as ordered.
  #autok3s::ginkgo::test "${CURR_DIR}/test/e2e/installation"
  #autok3s::ginkgo::test "${CURR_DIR}/test/e2e/usability"

  autok3s::log::info "...done"
}

function entry() {
  local stages="${1:-build}"
  shift $(($# > 0 ? 1 : 0))

  IFS="," read -r -a stages <<<"${stages}"
  local commands=$*
  if [[ ${#stages[@]} -ne 1 ]]; then
    commands="only"
  fi

  for stage in "${stages[@]}"; do
    autok3s::log::info "# make autok3s ${stage} ${commands}"
    case ${stage} in
    m | mod) mod "${commands}" ;;
    l | lint) lint "${commands}" ;;
    b | build) build "${commands}" ;;
    p | pkg | package) package "${commands}" ;;
    u | unit) unit "${commands}" ;;
    v | ver | verify) verify "${commands}" ;;
    e | e2e) e2e "${commands}" ;;
    cb | cross_build) CROSS=1; build "${commands}" ;;
    *) autok3s::log::fatal "unknown action '${stage}', select from mod,lint,build,unit,verify,package,e2e" ;;
    esac
  done
}

if [[  -z "${AUTOK3S_DEV_MODE:-}" ]]; then
  autok3s::dapper::run -C "${CURR_DIR}" -f ${DAPPER_FILE:-Dockerfile.dapper} "$@"
else
  entry "$@"
fi
