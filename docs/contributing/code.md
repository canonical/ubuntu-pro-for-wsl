---
myst:
  html_meta:
    "description lang=en":
      "How to build and test the code for Ubuntu Pro for WSL."
---

# Contributing to the code

Currently, we anticipate that most contributions will be for the Pro for WSL application.
Information helpful for the development of this application is included below.

## Prequisite: a Windows machine

To test and debug Ubuntu on WSL and Pro for WSL you will need a Windows
machine with WSL installed.

It is possible to run Ubuntu on WSL in a VM with nested visualisation but we do
not recommend this as a testing environment.

## The test suite for Ubuntu Pro for WSL

The source code includes a comprehensive test suite made of unit and integration tests. All the tests must pass with and without the race detector.

Each module has its own package tests and you can also find the integration tests at the appropriate end-to-end (e2e) directory.

The test suite must pass before merging the PR to our main branch. Any new feature, change or fix must be covered by corresponding tests.

## Additional dependencies for Ubuntu Pro for WSL

* Ubuntu 24.04 LTS
* Visual Studio Community 2019 or above
* Go
* Flutter
* An Ubuntu Pro token

## Building and running the binaries for Ubuntu Pro for WSL

For building, you can use the following two scripts:

* [Build the WSL Pro Service](https://github.com/canonical/ubuntu-pro-for-wsl/blob/main/tools/build/build-deb.sh)
* [Build the Windows Agent](https://github.com/canonical/ubuntu-pro-for-wsl/blob/main/tools/build/build-appx.ps1)

:::{note}
You'll need to [create a self-signing certificate](https://learn.microsoft.com/en-us/windows/msix/package/create-certificate-package-signing) to build the Windows Agent.
:::
