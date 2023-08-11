#!/bin/bash
set -eu

# Go to project_root/wsl-pro-service
cd $(dirname $(realpath "$0"))/../../wsl-pro-service

# Install dependencies
sudo apt update
sudo apt install -y devscripts
sudo apt -y build-dep .

# Build
DEB_BUILD_OPTIONS=nocheck debuild
