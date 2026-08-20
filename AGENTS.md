# Ubuntu Pro for WSL: Guide for coding agents

## Description

**Ubuntu Pro for WSL** is a multi component system that automates enrollment of any **Ubuntu on WSL**
instance into Canonical security and compliance services, such as Landscape and Ubuntu Pro, by
automatically enforcing its configuration to all existing and future instances. It's made of
components running on Ubuntu on WSL (Linux) and on the Windows host, described below:

- **wsl-pro-service**: A systemd unit distributed as a Debian package in the `main` component of the
Ubuntu archive (thus security and maintainability are key values). It runs inside each Ubuntu on WSL
instance and communicates with the host to apply configuration and report status.
- **ubuntu-pro-agent.exe** (a.k.a. the `agent`): A command line program running on the host with
user privileges. It's the core of this system, responsible for managing the configuration and
enrollment status of all Ubuntu on WSL instances.
- **ubuntupro.exe** (a.k.a the `gui`): A graphical user interface running on Windows to configure
the `agent` serving both end users and developers on testing and debugging.
- **ubuntu-pro-agent-launcher.exe** (a.k.a. the `launcher`): An invisible Win32 window that hosts
the `agent` under a pseudo console, allowing it to run in the background without console windows
popping up every time it interacts with WSL via API or command line.
- **ubuntu.com/wsl/docs**: The official documentation for Ubuntu on WSL and Pro for WSL, hosted on
the Ubuntu website.

The source code of all those components is hosted at https://github.com/canonical/ubuntu-pro-for-wsl
as a polyglot monorepo.


## Summary of features

### Ubuntu Pro

When configured with a Pro Token, this system enforces all _instances_ to automatically
attach to Ubuntu Pro using that token, granting access to Extended Support Maintenance (ESM)
repositories and other security features. It supports a hierarchy of sources of a token:

- Organization - deployed via a Windows registry key (friendly to remote management solutions)
- Microsoft Store subscription purchase - triggered via the GUI, a Canonical Contracts backend
deliver a Pro token once the entitlement is validated against the MS Store Graph API.
- Manual input - via the GUI, from which the user can attach and detach in bulk.

### Landscape

If provided, a Landscape client configuration template is expanded and applied to all _instances_,
making them connect to a Landscape server of choice, from where sysadmins can monitor and enforce
compliance. The following sources are hierarchically supported:

- Organization - via registry key as before, with the highest precedence.
- User - manual input via the GUI.

The `agent` communicates with the configured Landscape server and runs commands on its behalf, such
as to keep an _instance_ alive, install or uninstall _instances_.


## Integration with other Canonical tools

To enforce Pro attachment and Landscape configuration at first boot time, we leverage `cloud-init`.
Actions taken after first boot are mediated by `wsl-pro-service` itself.
The `landscape-client` is configured through the Pro for WSL app, but still has the role of
communicating with the server about the instance it is running on.

## Tech stack

**Go** is the preferred programming language and we strive to work with its latest major version,
constrained by the Ubuntu archive or quality tools like Tiobe TiCS. We deviate from Go only in the
following constrained scenarios:

| Scenario | Language / Tools | Components | Reasoning |
|----------|------------------|------------|-----------|
| Graphical user interface is required | Dart / Flutter | `gui` | Well known in the organization |
| Bridge into the Flutter native layer | C++ | `gui` | Imposed by Flutter for Desktop |
| Tight and self-contained integration with Windows and Microsoft Store APIs | C++ | `launcher`, `agent` (via DLLs) and `gui` via Flutter plugins | Easiest to integrate |
| High level IPC | protobuf | `agent`, `gui`, `wsl-pro-service` communicate via `gRPC` | Team's preference |
| Packaging for Ubuntu | `debhelper` | `wsl-pro-service` | Functionality requires it to be a system package |
| Packaging for Windows | XML / `winappCli` | `agent`, `gui` and `launcher` packaged as MSIX | Best user experience and security |
| High level build orchestration on Windows | PowerShell | | Native and powerful enough |
| Low level build system for C++ | CMake | `launcher` and parts of the `agent` and `gui` | Simplest and most widely used for C++ |
| CI/CD | YAML + PowerShell or Bash / GitHub Actions | | Native to GitHub and widely used |
| User documentation | `myST` + `sphinx` + `Python` + `Read the Docs` | `docs` | Organization standard |

Except for the `wsl-pro-service`, which heavily assumes Linux, and the C++ abstractions, which are
allowed to tightly couple to Windows features, **all high level components must be written in a
cross platform manner, even if targetting only Windows**. This allows for better testability.

## Folder structure

- `agentapi/` - protobuf bindings for the `agent` to `gui` and `wsl-pro-service` `gRPC` communication.
- `common/` - Go constants, logic and test helpers shared between the `agent` and `wsl-pro-service`.
- `contractsapi/` - Canonical Contracts API client in Go used for the integration with Microsoft
     Store subscription management.
- `docs/` - Official documentation for Ubuntu on WSL and Pro for WSL, hosted on the Ubuntu website.
- `docs/internal/` - Development documentation, like architectural decision records, domain language
     mapping and coding standards, targetting AI agents and developers, not published on the website.
- `end-to-end/` - High level end-to-end tests in Go that exercise the entire system, installing test
     versions of the packages and asserting state on Windows and WSL instances.
- `generate/` - Code generation support for things like Gettext-based translations in Go.
- `gui/` - Flutter plugin and Graphical user interface for Ubuntu Pro for WSL.
- `img/` - Images used in the README.md file.
- `launcher/` - C++ code for the `launcher` that hosts the `agent` under a pseudo console.
- `mocks/` - Mock implementations of the gRPC and REST services for unit and integration testing.
- `msix/` - MSIX packaging declarations and assets.
- `storeapi/` - C++ sources, its tests and a Go wrapper abstracting the native APIs for Microsoft
    Store subscription management.
- `tools/` - Implements the legacy `tools.go` idiom and some build time helpers, like
    `tools\build\compute_version.go`.
- `windows-agent/` - Go sources for the `agent`.
- `wsl-pro-service/` - Go sources for the `wsl-pro-service` and its packaging as a Debian package.

Some files in the root of the repository deserve special mention:

- `build.ps1` - PowerShell build orchestration script for Windows, invoked by CI and developers,
    used to compile, layout and package all Windows components.
- `CMakeLists.txt` - CMake build system entry point for the C++ components.
- `CMakePresets.json` - Implements presets for triggering MSVC static code analysis and clang-tidy.
- `Makefile` - only used by the TiCS tool, should mimic all steps taken in CI in a single invocation
   so that TiCS can visualize the entire build process, otherwise not used by developers or CI.


## Internal references

To learn more about the design and implementation of this system, consult the following internal
documentation:

- `docs/internal/adr.md` - Architectural decision records (ADRs) describing the true hard-to-change
   core decisions that steer and constrain the design and implementation of this system, grouped by
   different areas of implementation (packaging, security, integration, etc).
- `docs/internal/domain.md` - Defines the ubiquitous language, describing the meaning of terms and
    their usage in the codebase and, to less extent, user facing text, acting as a glossary to avoid
    repeating descriptions and keep the terminology sharp and concise.

Those must be kept up-to-date as terms or decisions are introduced or changed to keep them useful.
Only decisions that are hard-to-reverse, surprising without context, or the result of a trade-off,
should be recorded in the ADRs.

When writing code, follow the per language coding standards:

| Language | File |
|----------|------|
| Go | `docs/internal/go-standards.md` |

