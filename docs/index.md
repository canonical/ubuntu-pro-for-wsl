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

Ubuntu Pro for WSL is an automation tool for managing
instances of Ubuntu on WSL. If you are responsible for a fleet of Windows
devices, Pro for WSL will help you to monitor, customise and secure WSL at scale.

Ubuntu on WSL provides a fully-featured Ubuntu experience on Windows, suitable
for learning Linux, developing a personal open-source project or building for
production in an enterprise environment.

## In this documentation

* **Getting started**: [Setting up a development environment from scratch with Ubuntu on WSL](/tutorials/develop-with-ubuntu-wsl/)
* **Installation**: [Install Ubuntu on WSL](/howto/install-ubuntu-wsl2.md) • [Install Pro for WSL](/howto/set-up-up4w)
* **Releases**: [Distribution release reference](/reference/distributions/) • [Release notes for the Ubuntu distro and Pro app](/reference/release_notes)
* **WSL for enterprise:** [Remote deployment of WSL with Landscape](/tutorials/deployment/) • [Attaching a Pro subscription using the Windows registry](/howto/set-up-up4w/) • [Configuring the Landscape client](/howto/set-up-landscape-client/) • [Using the Landscape API](/howto/custom-rootfs-multiple-targets/) • [Enforcing Pro agent startup](/howto/enforce-agent-startup-remotely-registry/) • [Starting the agent remotely with InTune](/howto/start-agent-remotely/)
* **Security and Ubuntu Pro**: [Security overview](/explanation/security-overview/) • [Securing WSL with Pro](/tutorials/getting-started-with-up4w/) • [Firewall requirements](/reference/firewall_requirements/)
* **Configuration and customisation**: [Instance configuration reference](/reference/instance_configuration/) • [Automating configuration with cloud-init](/howto/cloud-init/) • [Customising an Ubuntu image for WSL](/howto/custom-ubuntu-distro/) • [Differences between WSL 1 and WSL 2](/explanation/compare-wsl-versions/)
* **GPU and graphics**: [Enabling GPU acceleration with CUDA](/howto/gpu-cuda/) • [Creating data visualisations](/howto/data-science-and-engineering/)
* **DevOps**:  [GitHub actions for WSL](/reference/actions/) • [Running a WSL GitHub workflow on Azure](/howto/run-workflows-azure/)  
* **Contributing**: [General contribution guidelines](/howto/contributing/) • [Developer guidelines](howto::dev-contrib)


## How the documentation is organised

This documentation uses the [Diátaxis structure](https://diataxis.fr/).

* [Tutorials](/tutorials/index) take you through practical, end-to-end learning experiences.
* [How-to guides](/howto/index) provide you with the steps necessary for completing specific tasks.
* [References](/reference/index) give you concise and factual information to support your understanding.
* [Explanations](/explanation/index) include topic overviews and additional context on the software.

## Project and community

Ubuntu on WSL is a member of the Ubuntu family. It’s an open-source project
that warmly welcomes community contributions, suggestions, fixes and
constructive feedback. Check out our [contribution
guidelines](/howto/contributing)
on GitHub in order to bring ideas, report bugs, participate in discussions and
much more!

Thinking about using Ubuntu on WSL for your next project? Get in touch!

```{toctree}
:hidden:
:titlesonly:

Ubuntu on WSL <self>
Tutorials </tutorials/index>
How-to guides </howto/index>
Reference </reference/index>
Explanation </explanation/index>
```
