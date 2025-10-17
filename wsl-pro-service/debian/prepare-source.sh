#!/bin/bash
set -eu

# Path to the versioning file.
# We must compute the versiong during the source package build because we still have access to git.
# We can't do this during the binary package, so we cannot use the usual -ldflags.
# Instead, we write down the version to a file during source build, and read the file during binary build.
VERSION_FILE="./version"

export GOWORK=off

has_git=$(git status > /dev/null 2>&1 && echo "1" || true)
# We assume this script triggered by the root of the wsl-pro-service directory tree,
# such as when invoked by the debhelper.
tools_file="../tools/build/compute_version.go"
# Handle vendoring and version detection
if [ -n "${has_git}" ]; then
    if [ -r "${tools_file}" ]; then
       go run "${tools_file}" > ${VERSION_FILE}
       rm -r vendor &> /dev/null || true
       go mod vendor
    fi
fi

version=$(cat ${VERSION_FILE})
echo -ldflags=-X=github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/consts.Version=${version}
