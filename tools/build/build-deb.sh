#!/bin/bash
set -eu

# Go to project_root/wsl-pro-service
cd $(dirname $(realpath "$0"))/../../wsl-pro-service

# Install dependencies
apt update
apt install -y devscripts equivs

# Build
mk-build-deps --install --tool="apt -y" --remove
DEB_BUILD_OPTIONS=nocheck debuild
