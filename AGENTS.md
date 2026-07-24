# Ubuntu Pro for WSL Guide for Coding Agents

## Description

Ubuntu Pro for WSL is a multi component system that allows Windows users to automate security and
compliance of Ubuntu on WSL by ensuring a single source of truth configuration living on the host is
automatically applied to all existing and future Ubuntu on WSL instances.

The constituent components coexist in this multi language monorepo hosted at
https://github.com/canonical/ubuntu-pro-wsl and are described below.

- **wsl-pro-service**: A systemd unit distributed as a Debian package in the `main` pocket of the
Ubuntu archive. It runs inside each Ubuntu on WSL instance and communicates with the host to apply
configuration and report status. It is installed by default.
- **ubuntu-pro-agent.exe** (a.k.a. the `agent`): A command line program running on the host with
user privileges. It's the core of this system, responsible for managing the configuration and status
of all Ubuntu on WSL instances.
- **ubuntupro.exe** (a.k.a the `gui`): A graphical user interface running on Windows to configure
the `agent` for both end users and to ease testing and debugging for developers.
- **ubuntu-pro-agent-launcher.exe** (a.k.a. the `launcher`): An invisible Win32 window that hosts
the `windows-agent` under a pseudo console, allowing it to run in the background without popping up
console windows when interacting with WSL via API or command line.
- **ubuntu.com/wsl/docs** (a.k.a the `docs`): The official documentation for Ubuntu on WSL and Pro
for WSL, hosted on the Ubuntu website.


## Tech stack

**Go** is the preferred programming language and we strive to work with its latest major version,
constrained by the Ubuntu archive or quality tools like Tiobe TiCS. We deviate from Go only
in the following scenarios:

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
| User documentation | `myST` + `sphinx` + `Read the Docs` | `docs` | Organization standard |

Except of the `wsl-pro-service`, which heavily assumes Linux, and the C++ abstractions, which might
be too tighly coupled to Windows features, all high level components are written in a cross platform
manner, even if targetting only Windows. This allows for better testability.

## Folder structure

- `.github/` - GitHub Actions CI/CD workflows and configuration.
- `.configurations/` - Development environment configuration files to ease onboarding.
- `agentapi/` - protobuf bindings for the `agent`, `gui` and `wsl-pro-service` `gRPC` communication.
- `common/` - Go constants, logic  and test helpers shared between the `agent` and `wsl-pro-service`.
- `contractsapi/` - Canonical Contracts API client in Go used for the integration with Microsoft
    Store subscription management.
- `docs/` - Official documentation for Ubuntu on WSL and Pro for WSL, hosted on the Ubuntu website.
- `docs/internal/` - Specific development documentation, like architectural decision records, domain
    language mapping and coding standards, not published on the website.
- `end-to-end/` - High level end-to-end tests in Go that exercise the entire system, installing test
    versions of the packages and asserting state on Windows and WSL instances.
- `generate/` - Code generation scripts for protobuf and Gettext-based translations.
- `gui/` - Flutter plugin and Graphical user interface for Ubuntu Pro for WSL.
- `img/` - Images used in the README.md file.
- `launcher/` - C++ code for the `ubuntu-pro-agent-launcher.exe` that hosts the `windows-agent` under a pseudo console.
- `mocks/` - Mock implementations of the gRPC and REST services for unit testing.
- `msix/` - MSIX packaging declarations and assets.
- `storeapi/` - C++ sources and test abstracting the native APIs for Microsoft Store subscription management.
- `tools/` - Implements the legacy `tools.go` idiom and some build time helpers.
- `windows-agent/` - Go sources for the `agent`.
- `wsl-pro-service/` - Go sources for the `wsl-pro-service` and its packaging as a Debian package.

Some files in the root of the repository deserve special mention:

- `build.ps1` - PowerShell build orchestration script for Windows, invoked by CI and developers,
   used to compile, layout and package all Windows components.
- `CMakeLists.txt` - CMake build system entry point for the C++ components.
- `CMakePresets.json` - Implements CMake presets for triggering MSVC static code analysis and clang-tidy.
- `go.work` - Implments the multi module Go workspace.
- `Makefile` - only used by the TiCS tool, should mimic all steps taken in CI in a single invocation
  so that TiCS can visualize the entire build process.


## Internal references

To learn more about the design and implementation of this system, consult the following internal
documentation:

- `docs/internal/adr.md` - Architectural decision records (ADRs) describing the true hard-to-change
  core decisions that steer and constrain the desing and implementation of this system.
- `docs/internal/domain.md` - Domain language mapping for the system, describing the meaning of
  terms and their usage in the codebase and, to less extent, user facing text. We use it as a
  glossary to avoid repeating descriptions and keep the terminology sharp and concise.

Those must be kept up-to-date as terms or decisions are introduced or changed to keep them useful.
Only true hard-to-reverse, surprising without context or the result of a real trade-off decisions should be recorded in the ADRs.

When writing code, follow the per language coding standards:

| Language | File |
|----------|------|
| C++ | `docs/internal/cpp-standards.md` |
| Dart | `docs/internal/dart-standards.dart.md` |
| Go | `docs/internal/go-standards.go.md` |

