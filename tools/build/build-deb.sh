#!/bin/bash
set -eu

# Go to project_root/wsl-pro-service
cd $(dirname $(realpath "$0"))/../../wsl-pro-service

# Install dependencies
apt update
apt install -y devscripts
apt -y build-dep .

# Build
DEB_BUILD_OPTIONS=nocheck debuild
