---
myst:
  html_meta:
    "description lang=en":
      "General guidelines for contributing to the development of Ubuntu on WSL."
---

# General contribution guidelines

To ensure that making a contribution is a positive experience for both
contributor and reviewer, we ask that you read and follow these community
guidelines.

:::{admonition} Specific contributing guides
:class: tip
If you are interested in specific guidance on contributing to the code
or docs, it is provided in the following pages:

* [Contributing to the code](/contributing/code)
* [Contributing to the documentation](/contributing/docs)
:::

## Code of conduct

We take our community seriously and hold ourselves and other contributors to
high standards of communication. By participating and contributing, you agree
to uphold the Ubuntu Community [Code of
Conduct](https://ubuntu.com/community/ethos/code-of-conduct).

## Contributor License Agreement

You need to sign the [Contributor License
Agreement](https://ubuntu.com/legal/contributors) to contribute. You only need
to sign this once and if you have previously signed the agreement when
contributing to other Canonical projects you will not need to sign it again.

An automated test is executed on PRs to check if it has been accepted.

Please refer to the licences for the Ubuntu distro and Windows application
below:

- [Ubuntu on WSL distro](https://github.com/ubuntu/WSL/blob/main/LICENSE)
- [Pro for WSL app](https://github.com/canonical/ubuntu-pro-for-wsl/blob/main/LICENSE)

## Where to find the code and the documentation

Currently, there are two repositories maintained by Canonical that relate to Ubuntu on WSL.
Microsoft maintains a separate repository for the WSL technology itself.

### Code for the distro and the app are in different repositories

The source code for the Ubuntu on WSL **distribution** and the Pro for WSL
**Windows application** can be found on GitHub:

- Distribution: [Ubuntu on WSL repo](https://github.com/ubuntu/WSL)
- Windows application: [Pro for WSL repo](https://github.com/canonical/ubuntu-pro-for-wsl)

The documentation for both is in the application repo.

### Issues with WSL itself should be directed to Microsoft

We accept any contributions relating to Ubuntu WSL and Pro for WSL. However, we
do not directly maintain WSL itself, which is a Microsoft product. If you have
identified a problem or bug in WSL then file an issue in [Microsoft's WSL
project repository](https://github.com/microsoft/WSL/issues/).

For example, the kernel used for Linux distributions -- including Ubuntu --
that run on WSL is maintained by Microsoft. If you have an issue relating to
the WSL kernel then you should direct your communication to Microsoft.

If you are unsure whether your problem relates to an Ubuntu project or the
Microsoft project then familiarise yourself with their respective
documentation.

- [Ubuntu on WSL docs](https://ubuntu.com/wsl/docs/stable/)
- [Microsoft WSL docs](https://learn.microsoft.com/en-us/windows/wsl/)

At this point, if you are still not sure, try to contact a maintainer of one of
the projects, who will advise you where best to submit your Issue.

## Creating Issues and Pull Requests

Contributions are made via Issues and Pull Requests (PRs).

* Use the advisories page of the repository and not a public bug report to
report security vulnerabilities. 
* Search for existing Issues and PRs before creating your own.
* Give a friendly ping in the comment thread to the submitter or a contributor to draw attention if your issue is blocking — while we work hard to makes sure issues are handled in a timely manner it can take time to investigate the root cause. 
* Read [this Ubuntu discourse post](https://discourse.ubuntu.com/t/contribute/26) for resources and tips on how to get started, especially if you've never contributed before

### Issues

Issues should be used to report problems with the software, request a new feature or to discuss potential changes before a PR is created. When you create a new Issue, a template will be loaded that will guide you through collecting and providing the information that we need to investigate.

If you find an Issue that addresses the problem you're having, please add your own reproduction information to the existing issue rather than creating a new one. Adding a [reaction](https://github.blog/2016-03-10-add-reactions-to-pull-requests-issues-and-comments/) can also help by indicating to our maintainers that a particular problem is affecting more than just the reporter.

(contribute::pull-requests)=
### Pull Requests

PRs are always welcome and can be a quick way to get your fix or improvement slated for the next release. In general, PRs should:

* Only fix/add the functionality in question **OR** address wide-spread whitespace/style issues, not both.
* Add unit or integration tests for fixed or changed functionality.
* Address a single concern in the least number of changed lines as possible.
* Include documentation in the repo or on our [docs site](https://ubuntu.com/wsl/docs).
* Use the complete Pull Request template (loaded automatically when a PR is created).

For changes that address core functionality or would require breaking changes (e.g. a major release), it's best to open an Issue to discuss your proposal first. This is not required but can save time creating and reviewing changes.

In general, we follow the ["fork-and-pull" Git workflow](https://github.com/susam/gitpr):

1. Fork the repository to your own GitHub account.
1. Clone the fork to your machine.
1. Create a branch locally with a succinct yet descriptive name.
1. Commit changes to your branch.
1. Follow any formatting and testing guidelines specific to this repo.
1. Push changes to your fork.
1. Open a PR in our repository and follow the PR template so that we can efficiently review the changes.

PRs will trigger unit and integration tests with and without race detection, linting and formatting validations, static and security checks, freshness of generated files verification. All the tests must pass before anything is merged into the main branch.

## Contributing to the code

See [Contributing to the code](/contributing/code) for the test
suite, additional dependencies, and how to build and run the binaries.

## Contributing to the documentation

See [Contributing to the documentation](/contributing/docs) for how
to work on the documentation locally, our conventions for code blocks, and
how to join the Canonical Open Documentation Academy.

## Getting help

Join us in the [Ubuntu Community](https://discourse.ubuntu.com/c/wsl/27) and post your question there with a descriptive tag.

```{toctree}
:maxdepth: 1
:hidden:

self
code
docs
```

