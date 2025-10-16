---
myst:
  html_meta:
    "description lang=en":
      "Reference information on the QA process used in the development of Ubuntu pro for WSL."
---

# QA Process

```{include} ../includes/dev_docs_notice.txt
    :start-after: <!-- Include start dev -->
    :end-before: <!-- Include end dev -->
```

## Generalities

```{note}
We always use the latest Go version available. Information about specific Go
versions on this page may be outdated, as the version used is periodically
updated.
```

`wsl-pro-service` is seeded only on WSL images.

```
Build-dep: golang-go (\>= 2:1.21\~)
```

At any point in time, only the latest two versions of the Go toolchain receive security patches. Hence, we need to keep backporting new releases to fix vulnerabilities. They follow an approximate 6-month release cycle, so Go 1.21 should fall out of support by August 2024.

## Process

WSL Pro Service follows a robust continuous integration and testing process. It is covered by [a comprehensive automated test suite](https://github.com/canonical/ubuntu-pro-for-wsl/actions/workflows/qa.yaml).

The team applies the following quality criteria:

- All changes are thoroughly reviewed and approved by core team members before integration.
- Each change is thoroughly tested at the unit, integration and system levels. All the tests pass in all supported architectures.
- Releases are reviewed as part of the [SRU exception](https://wiki.ubuntu.com/UbuntuProForWSLUpdates).

The test plan is **completely automated** and runs **every time a change is merged**, as well as **during packaging**. This covers integration and end-to-end tests. Integration tests run on each LTS affected by the SRU to ensure compatibility.

Testing also covers the upgrade from the current version to the proposed version.

Tests are not executed on different versions of Windows due to testing environment limitations.

## Packaging QA

To prepare the release to LTS, the following procedure is being completed to ensure quality:

- All autopkgtests pass. Unit tests are executed as autopkgtests. Running higher-level tests would require a Windows VM. It is not available in autopkgtest at the moment. Even if wsl-pro-service tests could run in a VM, they wouldn't test anything real.
- The package does not break when upgrading.
- The binary is identical to the CI build, with only Debian packaging changes.
- The copyrights and changelog are up to date.
- An upgrade test from the previous package version has been performed using apt install/upgrade.

## Code sanity

Code sanity checks are performed automatically on each build. They verify:

- Code linting.
- Go module files are up to date.
- Generated files are up to date.
- Any binary in the project builds.
- The Debian package builds.
- Vulnerabilities. It is a run of `govulncheck`.

All the layers are tested from APIs to mocks to the service itself

### Example reports

- Code sanity and unit testing: [QA workflow](https://github.com/canonical/ubuntu-pro-for-wsl/actions/workflows/qa.yaml?query=branch%3Amain)
- Integration tests: [end-to-to-end tests workflow](https://github.com/canonical/ubuntu-pro-for-wsl/actions/workflows/qa-azure.yaml?query=branch%3Amain)

<!-- This link is broken because the repo is private -->
![image](https://github.com/canonical/ubuntu-pro-for-wsl/assets/1928546/649084df-1889-471a-a211-df3ae890a8fd)

## Code coverage

There is no Codecov report due to the limitations of private projects.
However, code coverage is calculated and displayed at testing time.
Coverage is manually reviewed by the engineers.

## Bug reporting

The main bug tracker remains on GitHub. [GitHub Templates](https://github.com/canonical/ubuntu-pro-for-wsl/issues/new/choose)
are available to help the user with the bug-reporting process and provide the right information.

Wsl-pro supports ubuntu-bug reporting to Launchpad with an apport hook but we are not collecting any data at the moment.

## References

- [Project Documentation](https://canonical-ubuntu-pro-for-wsl.readthedocs-hosted.com/en/latest/)
- [Ubuntu Pro for WSL SRU exception](https://wiki.ubuntu.com/UbuntuProForWSLUpdates)
- [Ubuntu Pro tools SRU exception](https://wiki.ubuntu.com/UbuntuAdvantageToolsUpdates)
