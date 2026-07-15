(howto::index-remote-deployment)=

# Remote deployment

A key feature of Pro for WSL is that it enables deployment and management
of WSL instances. For example, an administrator in an organization can remotely
deploy custom Ubuntu configurations and images on multiple Windows machines,
using tools like Landscape and Intune.

These guides help you deploy and manage WSL instances using Ubuntu Pro for WSL
with remote management tools. They are not intended to reflect full production
deployments but provide guidance on key steps, which can be adapted to your
specific production environments.

## Landscape

Landscape is a systems administration tool from Canonical. Pro for WSL supports
Landscape natively and a Landscape client is installed in each Ubuntu instance.
The guides below show you how to configure the Landscape client, and deploy WSL
using the Landscape web portal or API.

```{toctree}
:titlesonly:
:maxdepth: 1

Deploy Ubuntu instances with the Landscape web portal <deploy-with-landscape-web-portal>
Deploy instances with the Landscape API <custom-rootfs-multiple-targets>
Configure the Landscape client with the Pro app <set-up-landscape-client>
```

## Intune

Intune is a cloud-based endpoint management tool from Microsoft. The guides
below are focused on helping Intune administrators ensure that that the Ubuntu
Pro for WSL's background agent is operating on remote Windows machines.

```{toctree}
:titlesonly:
:maxdepth: 1

Enforce the Pro agent startup remotely using the Windows Registry <enforce-agent-startup-remotely-registry>
Start the Pro agent remotely with Intune <start-agent-remotely>
```

## Custom images

It may be necessary to customize an Ubuntu image before deploying it within an organization.
The following step-by-step guide shows you how to customize an Ubuntu image for WSL.

```{toctree}
:titlesonly:
:maxdepth: 1

Customize an Ubuntu image for WSL <custom-ubuntu-distro>
```
