#!/bin/bash
set -eu

# Go to project_root/wsl-pro-service
cd $(dirname $(realpath "$0"))/../../wsl-pro-service

# Enable source repos
sudo sed -i 's/# deb-src/deb-src/' /etc/apt/sources.list

# Install dependencies
sudo DEBIAN_FRONTEND=noninteractive apt update
sudo DEBIAN_FRONTEND=noninteractive apt install -y devscripts
sudo DEBIAN_FRONTEND=noninteractive apt -y build-dep .

# Build
DEB_BUILD_OPTIONS=nocheck debuild
