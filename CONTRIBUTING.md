# Contributing

## Ubuntu on WSL and Ubuntu Pro for WSL

<!-- Include start contributing intro -->

To ensure that making a contribution is a positive experience for both
contributor and reviewer we ask that you read and follow these community
guidelines.

This communicates that you respect our time as developers. We will return
that respect by addressing your issues, assessing proposed changes and
finalising your pull requests, as helpfully and efficiently as possible.

These are mostly guidelines, not rules. Use your best judgement and feel free to
propose changes to this document in a pull request.

<!-- Include end contributing intro -->

## Quicklinks

- [Prerequisites](#prerequisites)
- [Where to find the code and the documentation](#where-to-find-the-code-and-the-documentation)
- [Creating Issues and Pull Requests](#creating-issues-and-pull-requests)
- [Contributing to the code](#contributing-to-the-code)
- [Contributing to the documentation](#contributing-to-the-documentation)
- [Getting Help](#getting-help)

<!-- Include start contributing main -->

## Prerequisites

### Code of conduct

We take our community seriously and hold ourselves and other contributors to high standards of communication. By participating and contributing, you agree to uphold the Ubuntu Community [Code of Conduct](https://ubuntu.com/community/ethos/code-of-conduct).

### GitHub

You need a GitHub account to create issues, comment, reply or submit contributions.

You don’t need to know git before you start, and you definitely don’t need to work on the command line if you don’t want to. Many documentation tasks can be done using GitHub’s web interface. On the command line, we use the standard “[fork and pull](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/creating-a-pull-request-from-a-fork)” process.

### Contributor License Agreement

You need to sign the [Contributor License
Agreement](https://ubuntu.com/legal/contributors) to contribute.
You only need to sign this once and if you have previously signed the
agreement when contributing to other Canonical projects you will not need to
sign it again.

An automated test is executed on PRs to check if it has been accepted.

Please refer to the licences for Ubuntu WSL and Ubuntu Pro for WSL below.

- [Ubuntu WSL](https://github.com/ubuntu/WSL/blob/main/LICENSE)
- [Ubuntu Pro for WSL](https://github.com/canonical/ubuntu-pro-for-wsl/blob/main/LICENSE)

### A Windows machine

To test and debug Ubuntu on WSL and Ubuntu Pro for WSL you will need a Windows
machine with WSL installed.

It is possible to run Ubuntu on WSL in a VM with nested visualisation but we do
not recommend this as a testing environment.

## Where to find the code and the documentation

Currently, there are two repositories maintained by Canonical that relate to Ubuntu on WSL.
Microsoft maintains a separate repository for the WSL technology itself.

### Code for the distro and the app are in different repositories

The source code for the Ubuntu on WSL **distribution** and the Ubuntu Pro for WSL **Windows application** can be found on GitHub:

- Distribution: [Ubuntu on WSL repo](https://github.com/ubuntu/WSL)
- Windows application: [Ubuntu Pro for WSL repo](https://github.com/canonical/ubuntu-pro-for-wsl)

### Issues with WSL itself should be directed to Microsoft

We accept any contributions relating to Ubuntu WSL and Ubuntu Pro for WSL.
However, we do not directly maintain WSL itself, which is a Microsoft product.
If you have identified a problem or bug in WSL then file an issue in
[Microsoft's WSL project repository](https://github.com/microsoft/WSL/issues/).

For example, the kernel used for Linux distributions -- including Ubuntu -- that run on WSL is maintained by Microsoft.
If you have an issue relating to the WSL kernel then you should direct your communication to Microsoft.

If you are unsure whether your problem relates to an Ubuntu project or the Microsoft project then familiarise yourself with their respective documentation.

- [Ubuntu on WSL docs](https://documentation.ubuntu.com/wsl/en/latest/)
- [Microsoft WSL docs](https://learn.microsoft.com/en-us/windows/wsl/)

At this point, if you are still not sure, try to contact a maintainer of one of the projects, who will advise you where best to submit your Issue.

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

### Pull Requests

PRs are always welcome and can be a quick way to get your fix or improvement slated for the next release. In general, PRs should:

* Only fix/add the functionality in question **OR** address wide-spread whitespace/style issues, not both.
* Add unit or integration tests for fixed or changed functionality.
* Address a single concern in the least number of changed lines as possible.
* Include documentation in the repo or on our [docs site](https://github.com/canonical/ubuntu-pro-for-wsl/wiki).
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

> PRs will trigger unit and integration tests with and without race detection, linting and formatting validations, static and security checks, freshness of generated files verification. All the tests must pass before anything is merged into the main branch.

Once merged to the main branch, `po` files will be automatically updated and are therefore not necessary to update in the pull request itself, which helps minimise diff review.

## Contributing to the code

Currently, we anticipate that most contributions will be for the Ubuntu Pro for WSL application.
Information helpful for the development of this application is included below.

### The test suite for Ubuntu Pro for WSL

The source code includes a comprehensive test suite made of unit and integration tests. All the tests must pass with and without the race detector.

Each module has its own package tests and you can also find the integration tests at the appropriate end-to-end (e2e) directory.

The test suite must pass before merging the PR to our main branch. Any new feature, change or fix must be covered by corresponding tests.

### Additional dependencies for Ubuntu Pro for WSL

* Ubuntu 24.04 LTS
* Visual Studio Community 2019 or above
* Go
* Flutter
* An Ubuntu Pro token

### Building and running the binaries for Ubuntu Pro for WSL

For building, you can use the following two scripts:

* [Build the WSL Pro Service](https://github.com/canonical/ubuntu-pro-for-wsl/blob/main/tools/build/build-deb.sh)
* [Build the Windows Agent](https://github.com/canonical/ubuntu-pro-for-wsl/blob/main/tools/build/build-appx.ps1)

Note that you'll need to [create a self-signing certificate](https://learn.microsoft.com/en-us/windows/msix/package/create-certificate-package-signing) to build the Windows Agent.

## Contributing to the documentation

The documentation for the Ubuntu WSL distro and Ubuntu Pro for WSL is maintained [here](https://github.com/canonical/ubuntu-pro-for-wsl/tree/main/docs).

You can contribute to the documentation in various different ways. If you are not a developer but want to help make the product better then helping us to improve the documentation is a way to achieve that.

At the top of each page in the documentation, you will find a **feedback** button.
Clicking this button will open an Issue submission page in the GitHub repo.
A template will automatically be loaded that you can modify before submitting the Issue.

For minor changes, such as fixing a single typo, you can click the **pencil** icon at the top of any page. This will open up the source file in GitHub so that you can make changes before committing them and submitting a PR.

For more significant changes to the content or organisation of the documentation, it is better to create your own fork of the repository to make the changes before then generating a PR.

Lastly, at the bottom of each page you will find various links, including a link to the Discourse forum for Ubuntu on WSL, where you can ask questions and participate in discussions.

### Types of contribution

Some common contributions to documentation are:

- Add or update documentation for new features or feature improvements by submitting a PR
- Add or update documentation that clarifies any doubts you had when working with the product by submitting a PR
- Request a fix to the documentation, by opening an issue on GitHub
- Post a question or suggestion on the forum

### Working on the documentation

If making significant changes to the documentation you should work on your own fork.
After cloning the fork, change into the `/docs/` directory.

A makefile is used to preview and test the documentation locally.
To view all the possible commands, run:

```text
make
```

The command `make run` will serve the documentation to port `8000` on `localhost`.
You can then preview the documentation in your browser and any changes that you save
will automatically be reflected in the preview.

To clean the build environment, run `make clean`.

When you submit a PR, there are automated checks for typos and broken links.
Please run the local tests before submitting the PR to save yourself and your reviewers time.

### Automatic documentation checks

Automatic checks will be run on any PR relating to documentation to verify spelling and the validity of links.
Before submitting a PR, you can check for issues locally:

- Check the spelling: `make spelling`
- Check the validity of links: `make linkcheck`

Doing these checks locally is good practice. You are less likely to run into
failed CI checks after your PR is submitted and the reviewer of your PR can
more quickly focus on the contribution you have made.

Your PR will generate a preview build of the documentation on Read the Docs.
This preview appears as a check in the CI.
Click on the check to open the preview.

### Note on using code blocks

In the Ubuntu WSL docs, code blocks are used to document:

- Ubuntu terminal commands
- PowerShell terminal commands
- Terminal outputs
- Code and config files

We follow specific conventions when including code blocks so that they
are readable and functional.

#### Include prompts when documenting terminal commands

It is common that Ubuntu and PowerShell terminal commands are included in the same page.
We use prompts to ensure that the reader can distinguish between them.

Here are some examples:

- PowerShell prompt symbol: `>`
- PowerShell prompt symbol with path: `C:\Users\myuser>`
- PowerShell prompt symbol with path and PowerShell prefix: `PS C:\Users\myuser>`
- Ubuntu prompt symbol: `$`
- Ubuntu prompt symbol with user and host: `user@host:~$`

Whether to include the path or user@host depends on whether it is useful in the context
of the documentation being written.
For example, if demonstrating the use of multiple WSL instances, including the user and host
can make it easier to tell the instances apart.

#### Exclude prompts from clipboard when using copy button

The WSL docs automatically strips prompts when a user clicks the **copy** button on a code block.
This is to prevent errors when a reader pastes the full content of a copy block into their terminal.

We use a solution based on regular expressions, which identifies the first instance of a prompt symbol followed by whitespace on a particular line before removing the text before that symbol.

There may be edge-cases when this creates problems; for example, you should include whitespace after a prompt but if you don't it may not be removed during copying.

Always test code blocks when you include them to ensure that the correct text is captured during the copy operation.
If you encounter a problem or edge-case contact the maintainers or file an issue.

#### Separate input from output and remove copy button from output blocks

Terminal commands are separated from the output that they generate.
Explanatory text can be included to explain to the reader what is being presented:

- "Run the following command..."
- "This will generate the following output..."

Copy buttons are not included in output blocks.
This is to prevent an output being confused for an input.
There are also few reasons why someone would copy an output from documentation.

To exclude a copy button from an output block the `no-copy` CSS class must be included
within the code block:

```text
:class: no-copy
```

Note: a code-block must be labelled with the [code-block directive](https://mystmd.org/guide/directives#directive-code) for this to work.

### The Open Documentation Academy

Ubuntu on WSL is a proud member of the [Canonical Open Documentation Academy](https://github.com/canonical/open-documentation-academy) (CODA).

CODA is an initiative to encourage open source contributions from the community, and to provide help, advice and mentorship to people making their first contributions.

A key aim of the initiative is to lower the barrier to successful open-source software contributions by making documentation into the gateway, and it’s a great way to make your first open source contributions to projects like Ubuntu on WSL.

The best way to get started is to take a look at our [project-related documentation tasks](https://github.com/canonical/open-documentation-academy/issues) and read our [Getting started guide](https://discourse.ubuntu.com/t/getting-started/42769). Tasks typically include testing and fixing documentation pages, updating outdated content, and restructuring large documents. We'll help you see those tasks through to completion.

For tasks related to Ubuntu on WSL:

* View [open issues that have yet to be assigned](https://github.com/canonical/open-documentation-academy/issues?q=is%3Aissue%20state%3Aopen%20label%3Awsl%20no%3Aassignee)
* View [closed issues for examples of previous contributions](https://github.com/canonical/open-documentation-academy/issues?q=is%3Aissue%20state%3Aclosed%20label%3Awsl)
* Create a [new issue for Ubuntu on WSL in the CODA repository](https://github.com/canonical/open-documentation-academy/issues/new)

You can get involved the with the CODA community through:

* The [discussion forum](https://discourse.ubuntu.com/c/community/open-documentation-academy/166) on the Ubuntu Community Hub
* The [Matrix channel](https://matrix.to/#/#documentation:ubuntu.com) for interactive chat
* [Fosstodon](https://fosstodon.org/@CanonicalDocumentation) for the latest updates and events

## Getting help

Join us in the [Ubuntu Community](https://discourse.ubuntu.com/c/wsl/27) and post your question there with a descriptive tag.

<!-- Include end contributing main -->
