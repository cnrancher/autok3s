#!/usr/bin/env bash

# -----------------------------------------------------------------------------
# Docker variables helpers. These functions need the
# following variables:
#
#    DOCKER_VERSION  -  The docker version for running, default is 19.03.

function autok3s::docker::validate() {
  if [[ -n "$(command -v docker)" ]]; then
    return 0
  fi

  autok3s::log::error "no docker available"
  return 1
}
