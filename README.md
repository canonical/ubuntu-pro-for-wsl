# ubuntu-pro-for-windows

[![License](https://img.shields.io/badge/License-GPL3.0-blue.svg)](https://github.com/canonical/ubuntu-pro-for-windows/blob/main/LICENSE)

Ubuntu Pro for Windows is a set of applications to manage Ubuntu WSL instances to allow to:

* grants ‘pro-enabled’ status to any Ubuntu instance on the device,
* orchestrates instances for Landscape,
* manages instance states (spin up/down to apply policies/patches).

It bridges Ubuntu WSL instances to Ubuntu Pro.

## System Components

The system consists of the following components:

* A MS Windows application made of an agent with its user interface. See [Windows Agent](windows-agent/README.md).
* A Ubuntu WSL service and its associated API. This interface controls the Pro and Landscape status between the agent running on Windows and the WSL instance. See [WSL Pro Service](wsl-pro-service/README.md).
* An interface between the agent and Ubuntu Pro to handle the transactions with the contract server.
* An interface between the agent and Landscape to manage the WSL instances from Landscape.
* A WSL management API. This interface controls the lifecycle of the WSL instances, like provisioning, updates, and starting or stopping the WSL instances.
* cloud-init is used to customise the images on first boot or to reconfigure an image.
