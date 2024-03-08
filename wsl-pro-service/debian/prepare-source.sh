#!/bin/bash
set -eu

# Path to the versioning file.
# We must compute the versiong during the source package build because we still have access to git.
# We can't do this during the binary package, so we cannot use the usual -ldflags.
# Instead, we write down the version to a file during source build, and read the file during binary build.
VERSION_FILE="./version"

export GOWORK=off

is_source_build=$(git status > /dev/null 2>&1 && echo "1" || true)

# Handle vendoring and version detection
if [ -n "${is_source_build}" ]; then
    go run ../tools/build/compute_version.go > ${VERSION_FILE}
    rm -r vendor &> /dev/null || true
    go mod vendor
fi

version=$(cat ${VERSION_FILE})
echo -ldflags=-X=github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/consts.Version=${version}
