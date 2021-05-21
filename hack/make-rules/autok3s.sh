#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

CURR_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

# The root of the autok3s directory
ROOT_DIR="${CURR_DIR}"
UI_VERSION="v0.4.3"

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
  ui
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

  local platforms
  if [[ "${CROSS:-false}" == "true" ]]; then
    autok3s::log::info "crossed building"
    platforms=("${SUPPORTED_PLATFORMS[@]}")
  else
    local os="${OS:-$(go env GOOS)}"
    local arch="${ARCH:-$(go env GOARCH)}"
    platforms=("${os}/${arch}")
  fi

  for platform in "${platforms[@]}"; do
    autok3s::log::info "building ${platform}"

    local os_arch
    IFS="/" read -r -a os_arch <<<"${platform}"

    local os=${os_arch[0]}
    local arch=${os_arch[1]}
    if [[ "$os" == "windows" ]]; then
        if [[ "$arch" == "amd64" ]]; then
            GOOS=${os} GOARCH=${arch} CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc-posix CXX=x86_64-w64-mingw32-g++-posix go build \
              -ldflags "${version_flags} ${flags} ${ext_flags}" \
              -o "${CURR_DIR}/bin/autok3s_${os}_${arch}.exe" \
              "${CURR_DIR}/main.go"
            cp -f "${CURR_DIR}/bin/autok3s_${os}_${arch}.exe" "${CURR_DIR}/dist/autok3s_${os}_${arch}.exe"
        else
            GOOS=${os} GOARCH=${arch} CGO_ENABLED=1 CC=i686-w64-mingw32-gcc-posix CXX=i686-w64-mingw32-g++-posix go build \
              -ldflags "${version_flags} ${flags} ${ext_flags}" \
              -o "${CURR_DIR}/bin/autok3s_${os}_${arch}.exe" \
              "${CURR_DIR}/main.go"
            cp -f "${CURR_DIR}/bin/autok3s_${os}_${arch}.exe" "${CURR_DIR}/dist/autok3s_${os}_${arch}.exe"
        fi
    elif [[ "$arch" == "arm" ]]; then
        GOOS=${os} GOARCH=${arch} CGO_ENABLED=1 GOARM=7 CC=arm-linux-gnueabihf-gcc-5 CXX=arm-linux-gnueabihf-g++-5 CGO_CFLAGS="-march=armv7-a -fPIC" CGO_CXXFLAGS="-march=armv7-a -fPIC" go build \
          -ldflags "${version_flags} ${flags} ${ext_flags}" \
          -o "${CURR_DIR}/bin/autok3s_${os}_${arch}" \
          "${CURR_DIR}/main.go"
        cp -f "${CURR_DIR}/bin/autok3s_${os}_${arch}" "${CURR_DIR}/dist/autok3s_${os}_${arch}"
    elif [[ "$arch" == "arm64" ]]; then
        GOOS=${os} GOARCH=${arch} CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc-5 CXX=aarch64-linux-gnu-g++-5 go build \
          -ldflags "${version_flags} ${flags} ${ext_flags}" \
          -o "${CURR_DIR}/bin/autok3s_${os}_${arch}" \
          "${CURR_DIR}/main.go"
        cp -f "${CURR_DIR}/bin/autok3s_${os}_${arch}" "${CURR_DIR}/dist/autok3s_${os}_${arch}"
    elif [[ "$os" == "darwin" ]]; then
        GOOS=${os} GOARCH=${arch} CGO_ENABLED=1 go build \
          -ldflags "${version_flags} ${flags}" \
          -o "${CURR_DIR}/bin/autok3s_${os}_${arch}" \
          "${CURR_DIR}/main.go"
        cp -f "${CURR_DIR}/bin/autok3s_${os}_${arch}" "${CURR_DIR}/dist/autok3s_${os}_${arch}"
    else
        GOOS=${os} GOARCH=${arch} CGO_ENABLED=1 go build \
          -ldflags "${version_flags} ${flags} ${ext_flags}" \
          -o "${CURR_DIR}/bin/autok3s_${os}_${arch}" \
          "${CURR_DIR}/main.go"
        cp -f "${CURR_DIR}/bin/autok3s_${os}_${arch}" "${CURR_DIR}/dist/autok3s_${os}_${arch}"
    fi
  done

  autok3s::log::info "...done"
}

function package() {
  [[ "${1:-}" != "only" ]] && build
  autok3s::log::info "packaging autok3s..."

  local repo=${REPO:-cnrancher}
  local image_name=${IMAGE_NAME:-autok3s}
  local tag=${TAG:-${GIT_VERSION}}

  local platforms
  if [[ "${CROSS:-false}" == "true" ]]; then
    autok3s::log::info "crossed packaging"
    autok3s::docker::prebuild
    platforms=("${SUPPORTED_PLATFORMS[@]}")
  else
    local os="${OS:-$(go env GOOS)}"
    local arch="${ARCH:-$(go env GOARCH)}"
    platforms=("${os}/${arch}")
  fi

  pushd "${CURR_DIR}" >/dev/null 2>&1
  for platform in "${platforms[@]}"; do
    if [[ "${platform}" =~ darwin/* || "${platform}" =~ windows/* ]]; then
     autok3s::log::warn "package into Darwin/Windows OS image is unavailable, please use CROSS=true env to containerize multiple arch images or use OS=linux ARCH=amd64 env to containerize linux/amd64 image"
     continue
    fi

    local image_tag="${repo}/${image_name}:${tag}-${platform////-}"
    autok3s::log::info "packaging ${image_tag}"
    autok3s::docker::build \
      --platform "${platform}" \
      -t "${image_tag}" --load .
  done
  popd >/dev/null 2>&1

  autok3s::log::info "...done"
}

function deploy() {
  [[ "${1:-}" != "only" ]] && package
  autok3s::log::info "deploying autok3s..."

  local repo=${REPO:-cnrancher}
  local image_name=${IMAGE_NAME:-autok3s}
  local tag=${TAG:-${GIT_VERSION}}

  local platforms
  if [[ "${CROSS:-false}" == "true" ]]; then
    autok3s::log::info "crossed deploying"
    platforms=("${SUPPORTED_PLATFORMS[@]}")
  else
    local os="${OS:-$(go env GOOS)}"
    local arch="${ARCH:-$(go env GOARCH)}"
    platforms=("${os}/${arch}")
  fi
  local images=()
  for platform in "${platforms[@]}"; do
    if [[ "${platform}" =~ darwin/* || "${platform}" =~ windows/* ]]; then
      autok3s::log::warn "package into Darwin/Windows OS image is unavailable, please use CROSS=true env to containerize multiple arch images or use OS=linux ARCH=amd64 env to containerize linux/amd64 image"
    else
      images+=("${repo}/${image_name}:${tag}-${platform////-}")
    fi
  done

  local only_manifest=${ONLY_MANIFEST:-false}
  local without_manifest=${WITHOUT_MANIFEST:-false}
  local ignore_missing=${IGNORE_MISSING:-false}

  # docker push
  if [[ "${only_manifest}" == "false" ]]; then
    autok3s::docker::push "${images[@]}"
  else
    autok3s::log::warn "deploying images has been stopped by ONLY_MANIFEST"
    # execute manifest forcibly
    without_manifest="false"
  fi

  # docker manifest
  if [[ "${without_manifest}" == "false" ]]; then
    if [[ "${ignore_missing}" == "false" ]]; then
      autok3s::docker::manifest "${repo}/${image_name}:${tag}" "${images[@]}"
    else
      autok3s::manifest_tool::push from-args \
        --ignore-missing \
        --target="${repo}/${image_name}:${tag}" \
        --template="${repo}/${image_name}:${tag}-OS-ARCH" \
        --platforms="$(autok3s::util::join_array "," "${platforms[@]}")"
    fi
  else
    autok3s::log::warn "deploying manifest images has been stopped by WITHOUT_MANIFEST"
  fi

  autok3s::log::info "...done"
}

function unit() {
  [[ "${1:-}" != "only" ]] && build
  ui
  autok3s::log::info "running unit tests for autok3s..."

  local unit_test_targets=(
    "${CURR_DIR}/cmd/..."
    "${CURR_DIR}/pkg/..."
    "${CURR_DIR}/test/..."
  )

  if [[ "${CROSS:-false}" == "true" ]]; then
    autok3s::log::warn "crossed test is not supported"
  fi

  local os="${OS:-$(go env GOOS)}"
  local arch="${ARCH:-$(go env GOARCH)}"
  if [[ "${arch}" == "arm" ]]; then
    # NB(thxCode): race detector doesn't support `arm` arch, ref to:
    # - https://golang.org/doc/articles/race_detector.html#Supported_Systems
    GOOS=${os} GOARCH=${arch} CGO_ENABLED=1 go test \
      -tags=test \
      -cover -coverprofile "${CURR_DIR}/dist/coverage_${os}_${arch}.out" \
      "${unit_test_targets[@]}"
  else
    GOOS=${os} GOARCH=${arch} CGO_ENABLED=1 go test \
      -tags=test \
      -race \
      -cover -coverprofile "${CURR_DIR}/dist/coverage_${os}_${arch}.out" \
      "${unit_test_targets[@]}"
  fi

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
    d | dep | deploy) deploy "${commands}" ;;
    u | unit) unit "${commands}" ;;
    v | ver | verify) verify "${commands}" ;;
    e | e2e) e2e "${commands}" ;;
    *) autok3s::log::fatal "unknown action '${stage}', select from mod,lint,build,unit,verify,package,deploy,e2e" ;;
    esac
  done
}

if [[ ${BY:-} == "dapper" ]]; then
  autok3s::dapper::run -C "${CURR_DIR}" -f "Dockerfile.dapper" "$@"
else
  entry "$@"
fi
