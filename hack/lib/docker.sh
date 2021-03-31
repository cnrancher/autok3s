#!/usr/bin/env bash

# -----------------------------------------------------------------------------
# Docker variables helpers. These functions need the
# following variables:
#
#    DOCKER_VERSION  -  The docker version for running, default is 19.03.

function autok3s::docker::install() {
  local version=${DOCKER_VERSION:-"19.03"}
  curl -SfL "https://get.docker.com" | sh -s VERSION="${version}"
}

function autok3s::docker::validate() {
  if [[ -n "$(command -v docker)" ]]; then
    return 0
  fi

  autok3s::log::info "installing docker"
  if autok3s::docker::install; then
    autok3s::log::info "docker: $(docker version --format '{{.Server.Version}}' 2>&1)"
    return 0
  fi
  autok3s::log::error "no docker available"
  return 1
}

function autok3s::docker::login() {
  if [[ -n ${DOCKER_USERNAME} ]] && [[ -n ${DOCKER_PASSWORD} ]]; then
    if ! docker login -u "${DOCKER_USERNAME}" -p "${DOCKER_PASSWORD}" >/dev/null 2>&1; then
      return 1
    fi
  fi
  return 0
}

function autok3s::docker::prebuild() {
  docker run --rm --privileged multiarch/qemu-user-static --reset -p yes i
  DOCKER_CLI_EXPERIMENTAL=enabled docker buildx create --name multibuilder
  DOCKER_CLI_EXPERIMENTAL=enabled docker buildx inspect multibuilder --bootstrap
  DOCKER_CLI_EXPERIMENTAL=enabled docker buildx use multibuilder
}

function autok3s::docker::build() {
  if ! autok3s::docker::validate; then
    autok3s::log::fatal "docker hasn't been installed"
  fi
  # NB(thxCode): use Docker buildkit to cross build images, ref to:
  # - https://docs.docker.com/engine/reference/builder/#automatic-platform-args-in-the-global-scope#buildkit
  DOCKER_CLI_EXPERIMENTAL=enabled DOCKER_BUILDKIT=1 docker buildx build "$@"
}

function autok3s::docker::manifest() {
  if ! autok3s::docker::validate; then
    autok3s::log::fatal "docker hasn't been installed"
  fi
  if ! autok3s::docker::login; then
    autok3s::log::fatal "failed to login docker"
  fi

  # NB(thxCode): use Docker manifest needs to enable client experimental feature, ref to:
  # - https://docs.docker.com/engine/reference/commandline/manifest_create/
  # - https://docs.docker.com/engine/reference/commandline/cli/#experimental-features#environment-variables
  autok3s::log::info "docker manifest create --amend $*"
  DOCKER_CLI_EXPERIMENTAL=enabled docker manifest create --amend "$@"

  # NB(thxCode): use Docker manifest needs to enable client experimental feature, ref to:
  # - https://docs.docker.com/engine/reference/commandline/manifest_push/
  # - https://docs.docker.com/engine/reference/commandline/cli/#experimental-features#environment-variables
  autok3s::log::info "docker manifest push --purge ${1}"
  DOCKER_CLI_EXPERIMENTAL=enabled docker manifest push --purge "${1}"
}

function autok3s::docker::push() {
  if ! autok3s::docker::validate; then
    autok3s::log::fatal "docker hasn't been installed"
  fi
  if ! autok3s::docker::login; then
    autok3s::log::fatal "failed to login docker"
  fi

  for image in "$@"; do
    autok3s::log::info "docker push ${image}"
    docker push "${image}"
  done
}
