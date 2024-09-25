#!/usr/bin/env sh

set -e

GIT_VERSION=${DRONE_TAG:-${GITHUB_REF#refs/tags/}}
TARGET_PATH="$1"
RC_RELEASE="false"

if [ -z "$(command -v release-notary)" ]; then
    echo "release-notary is not found, skip generating release notes."
    exit 0
fi

if [ -z "${GIT_VERSION}" ]; then
    echo "running this scrpit without tag, skip generating release notes."
    exit 0
fi

GIT_VERSION=$(echo "${GIT_VERSION}" | grep -E "^v([0-9]+)\.([0-9]+)(\.[0-9]+)?(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$") || true

if [ "${GIT_VERSION}" = "" ]; then
    echo "git version is not validated, skip generating release notes."
    exit 0
fi

for tag in $(git tag -l --sort=-v:refname); do
    if [ "${tag}" = "${GIT_VERSION}" ]; then
        continue
    fi
    filterred=$(echo "${tag}" | grep -E "^v([0-9]+)\.([0-9]+)(\.[0-9]+)?(-rc[0-9]*)$") || true
    if [ "${filterred}" = "" ]; then
        echo "get real release tag ${tag}, stopping untag"
        break
    fi
    git tag -d ${tag}
done

echo "following release notes will be published..."
release-notary publish -d 2>/dev/null | sed '1d' | sed '$d' > "$TARGET_PATH"
cat "$TARGET_PATH"
