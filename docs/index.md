---
myst:
  html_meta:
    "description lang=en":
      "A complete Ubuntu environment on Windows using WSL, with enhanced security and remote management provided by Ubuntu Pro for WSL."
---

# Ubuntu on WSL

Windows Subsystem for Linux ([WSL](https://ubuntu.com/desktop/wsl)) enables
developers to run a GNU/Linux environment on Windows. The Ubuntu distribution
for WSL is tightly integrated with the Windows OS, supporting features
including remote development with popular IDEs and cross-OS file management.

[Ubuntu Pro for WSL](./explanation/pro-explanation) is a tool for managing
Ubuntu on WSL. It enables automatic attachment of your Pro subscription
to Ubuntu instances, and remote deployment with Landscape. If you are responsible for a fleet of Windows
devices, Pro for WSL helps you monitor, customize, and secure WSL at scale.

Ubuntu on WSL provides a fully-featured Ubuntu experience on Windows, suitable
for learning Linux, developing a personal open-source project or building for
production in an enterprise environment.

## In this documentation

|                    |                                                                     |
|--------------------|---------------------------------------------------------------------|
|**Get started** | [Set up an development environment on Windows with Ubuntu on WSL](/tutorials/develop-with-ubuntu-wsl/) |
|**Install Ubuntu** | [Install Ubuntu on WSL](/howto/install-ubuntu-wsl2.md) • [Available releases](/reference/distributions/) • [Upgrade your installation](/howto/upgrade-ubuntu) |
|**Configuration** | [Instance configuration methods](/reference/instance_configuration/) • [Automate configuration with cloud-init](/howto/cloud-init/) |
|**Security** | [Enable Pro](/howto/set-up-up4w) • [Security overview](/explanation/security-overview/) • [Firewall requirements](/reference/firewall_requirements/) |
|**Deployment** | [Deployment guides](/howto/index-remote-deployment/) • [Custom images](/howto/custom-ubuntu-distro/) • [Reference architecture](/explanation/ref-arch-explanation)
|**GPU and graphics** | [Enable GPU acceleration with CUDA](/howto/gpu-cuda/) • [Create data visualisations](/howto/data-science-and-engineering/) |
|**DevOps** |  [GitHub actions for WSL](/reference/actions/) • [Run a WSL GitHub workflow on Azure](/howto/run-workflows-azure/) |

## How the documentation is organised

This documentation uses the [Diátaxis structure](https://diataxis.fr/).

* [Tutorials](/tutorials/index) take you through practical, end-to-end learning experiences for WSL and Pro.
* [How-to guides](/howto/index) provide you with the steps necessary for completing specific tasks.
* [References](/reference/index) give you concise and factual information to support your understanding.
* [Explanations](/explanation/index) include topic overviews and additional context on the software.

## Project and community

Ubuntu on WSL is a member of the Ubuntu family. It’s an open-source project
that warmly welcomes community contributions, suggestions, fixes and
constructive feedback.

* [Code of conduct](https://ubuntu.com/community/ethos/code-of-conduct)
* [Contribution guidelines](/howto/contributing)

Thinking about using Ubuntu on WSL for your next project? Get in touch!

```{toctree}
:hidden:
:titlesonly:

Home <self>
Tutorials </tutorials/index>
How-to guides </howto/index>
Reference </reference/index>
Explanation </explanation/index>
```
