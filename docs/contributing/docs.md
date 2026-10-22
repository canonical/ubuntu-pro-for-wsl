---
myst:
  html_meta:
    "description lang=en":
      "How to contribute to the Ubuntu on WSL and Ubuntu Pro for WSL documentation."
---

# Contributing to the documentation

You can submit issues about the documentation by selecting the
{guilabel}`Give feedback` button at the top of any documentation page. Clicking this
button opens an Issue submission page in the GitHub repo. A template will
automatically be loaded that you can modify before submitting the Issue.

To propose changes to the documentation, you need to create a fork
of the GitHub repository, make the changes, and create a PR using
the [fork-and-pull](contribute::pull-requests) method.

## Working on a documentation change

If making significant changes to the documentation you should work on your own
fork of the GitHub repository:

- [github.com/canonical/ubuntu-pro-for-wsl](https://github.com/canonical/ubuntu-pro-for-wsl)

After cloning the fork, change into the `/docs/` directory.

A makefile is used to preview and test the documentation locally.
To view all of the possible commands, run:

```text
make
```

The command `make run` serves the documentation to port `8000` on `localhost`.
You can then preview the documentation in your browser and any changes that you
save will automatically be reflected in the preview.

To clean the build environment, run `make clean`. This is a good first step if
you notice that the documentation isn't updating like you might expect.

When you submit a PR, automated checks will run for typos, broken links, and
other problems. Please run the local tests before submitting the PR to save
yourself and your reviewers time.

## Automatic documentation checks

Automatic checks will be run on any PR relating to documentation to verify
spelling and the validity of links. Before submitting a PR, you can check for
issues locally:

- Check the spelling: `make spelling`
- Check the validity of links: `make linkcheck`

Doing these checks locally is good practice. You are less likely to run into
failed CI checks after your PR is submitted and the reviewer of your PR can
more quickly focus on the contribution you have made.

Your PR will generate a preview build of the documentation on Read the Docs.
This preview appears as a check in the CI. Click on the check to open the
preview.

## Note on using code blocks

In the Ubuntu WSL docs, code blocks are used to document:

- Ubuntu terminal commands
- PowerShell terminal commands
- Terminal outputs
- Code and config files

We follow specific conventions when including code blocks so that they
are readable and functional.

### Include prompts when documenting terminal commands

It is common that Ubuntu and PowerShell terminal commands are included in the
same page. We use prompts to ensure that the reader can distinguish between
them.

Here are some examples:

- PowerShell prompt symbol: `>`
- PowerShell prompt symbol with path: `C:\Users\username>`
- PowerShell prompt symbol with path and PowerShell prefix: `PS C:\Users\username>`
- Ubuntu prompt symbol: `$`
- Ubuntu prompt symbol with user and host: `<username>@<host>:~$`

Whether to include the path or username@host depends on whether it is useful in
the context of the documentation being written. For example, if demonstrating
the use of multiple WSL instances, including the user and host can make it
easier to tell the instances apart.

### Exclude prompts from clipboard when using copy button

The WSL docs automatically strips prompts when a user clicks the **copy**
button on a code block. This is to prevent errors when a reader pastes the full
content of a copy block into their terminal.

We use a solution based on regular expressions, which identifies the first
instance of a prompt symbol followed by whitespace on a particular line before
removing the text before that symbol.

There may be edge-cases when this creates problems; for example, you should
include whitespace after a prompt but if you don't it may not be removed during
copying.

Always test code blocks when you include them to ensure that the correct text
is captured during the copy operation. If you encounter a problem or edge-case
contact the maintainers or file an issue.

### Separate input from output and remove copy button from output blocks

Terminal commands are separated from the output that they generate. Explanatory
text can be included to explain to the reader what is being presented:

- "Run the following command..."
- "This will generate the following output..."

Copy buttons are not included in output blocks. This is to prevent an output
being confused for an input. There are also few reasons why someone would copy
an output from documentation.

To exclude a copy button from an output block the `no-copy` CSS class must be
included within the code block:

```text
:class: no-copy
```

:::{note}
A code-block must be labelled with the [code-block
directive](https://mystmd.org/guide/directives#directive-code) for this to
work.
:::

## The Open Documentation Academy

Ubuntu on WSL is a proud member of the [Canonical Open Documentation
Academy](https://github.com/canonical/open-documentation-academy) (CODA).

CODA is an initiative to encourage open source contributions from the
community, and to provide help, advice and mentorship to people making their
first contributions.

A key aim of the initiative is to lower the barrier to successful open-source
software contributions by making documentation into the gateway, and it’s a
great way to make your first open source contributions to projects like Ubuntu
on WSL.

The best way to get started is to take a look at our [project-related
documentation
tasks](https://github.com/canonical/open-documentation-academy/issues) and read
our [Getting started
guide](https://discourse.ubuntu.com/t/getting-started/42769). Tasks typically
include testing and fixing documentation pages, updating outdated content, and
restructuring large documents. We'll help you see those tasks through to
completion.

For tasks related to Ubuntu on WSL:

* View [open issues that have yet to be assigned](https://github.com/canonical/open-documentation-academy/issues?q=is%3Aissue%20state%3Aopen%20label%3Awsl%20no%3Aassignee)
* View [closed issues for examples of previous contributions](https://github.com/canonical/open-documentation-academy/issues?q=is%3Aissue%20state%3Aclosed%20label%3Awsl)
* Create a [new issue for Ubuntu on WSL in the CODA repository](https://github.com/canonical/open-documentation-academy/issues/new)

You can get involved the with the CODA community through:

* The [discussion forum](https://discourse.ubuntu.com/c/community/open-documentation-academy/166) on the Ubuntu Community Hub
* The [Matrix channel](https://matrix.to/#/#documentation:ubuntu.com) for interactive chat
* [Fosstodon](https://fosstodon.org/@CanonicalDocumentation) for the latest updates and events

