# Welcome to Ubuntu Pro For Windows

[actions-image]: https://github.com/canonical/ubuntu-pro-for-windows/actions/workflows/qa.yaml/badge.svg?branch=main
[actions-url]: https://github.com/canonical/ubuntu-pro-for-windows/actions?query=branch%3Amain+event%3Apush

[license-image]: https://img.shields.io/badge/License-GPL3.0-blue.svg

[codecov-image]: https://codecov.io/gh/canonical/ubuntu-pro-for-windows/branch/master/graph/badge.svg
[codecov-url]: https://codecov.io/gh/canonical/ubuntu-pro-for-windows

[goreport-image]: https://goreportcard.com/badge/github.com/canonical/ubuntu-pro-for-windows
[goreport-url]: https://goreportcard.com/report/github.com/canonical/ubuntu-pro-for-windows

[![Code quality][actions-image]][actions-url]
[![License][license-image]](LICENSE)

<!--
Disabled while the repo is private

[![Code coverage][codecov-image]][codecov-url]
[![Go Report Card][goreport-image]][goreport-url]
 -->

This is the code repository for Ubuntu Pro for Windows, the bridge from Ubuntu WSL instances to Ubuntu Pro.

It contains the set of applications to manage Ubuntu WSL instances that allows you to:

* Grant ‘pro-enabled’ status to any Ubuntu instance on the device
* Orchestrate instances for Landscape
* Manages instance states (spin up/down to apply policies/patches).

### System Components

The system consists of the following components:

* A Windows AppxPackage consisting of an agent with its user interface. See [Windows Agent](windows-agent/README.md).
* An Ubuntu WSL Pro Service and its associated API. This interface controls the Pro and Landscape status between the agent running on Windows and the WSL instance. See [WSL Pro Service](wsl-pro-service/README.md).
* An interface between the agent and Ubuntu Pro to handle the transactions with the contract server.
* An interface between the agent and Landscape to manage the WSL instances from Landscape.
* A WSL management API. This interface controls the lifecycle of the WSL instances, like provisioning, updates, and starting or stopping the WSL instances.
* cloud-init is used to customize the images on first boot or to reconfigure an image.

## Get involved

This is an [open source](LICENSE) project and we warmly welcome community contributions, suggestions, and constructive feedback. If you're interested in contributing, please take a look at our [Contribution guidelines](CONTRIBUTING.md) first.

- to report an issue, please file a bug report against our repository, using a bug template.
- for suggestions and constructive feedback, report a feature request bug report, using the proposed template.

## Get in touch

We're friendly! We have a community forum at [https://discourse.ubuntu.com](https://discourse.ubuntu.com) where we discuss feature plans, development news, issues, updates and troubleshooting.

For news and updates, follow the [Ubuntu twitter account](https://twitter.com/ubuntu) and on [Facebook](https://www.facebook.com/ubuntu).

## Troubleshooting

//TODO: fill troubleshooting section
