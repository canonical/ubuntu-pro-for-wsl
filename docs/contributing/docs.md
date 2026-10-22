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

```{terminal}
:prompt: ~/ubuntu-pro-for-wsl/docs$
:copy:

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

## Note on using terminal blocks

In the Ubuntu WSL docs, terminal blocks are used to document:

- Ubuntu terminal commands
- PowerShell terminal commands
- Terminal outputs

Terminal blocks are distinct from typical markdown code blocks and are
described in this section.

### Use the `{terminal}` directive for commands

Use the MyST `{terminal}` directive for any block that represents a command
typed at a prompt, whether or not it also shows output.

A basic example:

````text
```{terminal}
:prompt: $
:copy:

sudo apt update
```
````

In this example, you can see three elements specified, in order:

1. The prompt symbol to be used
1. A copy button that only copies the command
1. The command itself

These docs use two types of prompts frequently:

* PowerShell: `:prompt: PS>`
* Ubuntu/Bash: `:prompt: $`

Use a directory-aware prompt, such as `:prompt: ~/project$`, when the working
directory is useful information; for example, when there are a series of
commands which must be run in a specific directory or multiple specific
directories.

If you want a block that contains more than one command, split it into
consecutive `{terminal}` blocks. These will render as a single block, with each
command on its own line with its own prompt and copy button.

### Handling input and output

The `{terminal}` directive can show a command's output directly below it in the
same block. After the command, any content that follows the first blank line is
treated as output. Again, only the command remains copyable.

```{terminal}
:prompt: $
:copy:

pro status

STATUS
Attached: no
```

In situations where you only want to include output, use `:output-only:`, and
don't add a copy button:

````text
```{terminal}
:output-only:

<output text>
```
````

Standalone output blocks can be useful when you want to give a preamble
that deals with the output specifically:

> In the logs, you should see the name of the packages, for example...

### Code and config files stay as plain code blocks

File content, scripts, and code examples should use regular, fenced code blocks,
not `{terminal}`.

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

