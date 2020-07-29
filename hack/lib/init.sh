#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# Set no_proxy for localhost if behind a proxy, otherwise,
# the connections to localhost in scripts will time out
export no_proxy=127.0.0.1,localhost

# The root of the autok3s directory
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

source "${ROOT_DIR}/hack/lib/logging.sh"
source "${ROOT_DIR}/hack/lib/util.sh"
source "${ROOT_DIR}/hack/lib/version.sh"
source "${ROOT_DIR}/hack/lib/mockgen.sh"
source "${ROOT_DIR}/hack/lib/lint.sh"
source "${ROOT_DIR}/hack/lib/docker.sh"
source "${ROOT_DIR}/hack/lib/dapper.sh"
source "${ROOT_DIR}/hack/lib/manifest-tool.sh"
source "${ROOT_DIR}/hack/lib/ginkgo.sh"

autok3s::log::install_errexit
autok3s::version::get_version_vars
