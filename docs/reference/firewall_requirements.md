---
myst:
  html_meta:
    "description lang=en":
      "Reference information on firewall rules required for the operation of Ubuntu Pro for WSL."
---

(ref::firewall)=
# Firewall requirements for Ubuntu on WSL

```{include} ../includes/pro_content_notice.txt
    :start-after: <!-- Include start pro -->
    :end-before: <!-- Include end pro -->
```

Firewall rules must be configured for Ubuntu Pro for WSL to operate fully.

The following figure shows the possible connections between the different components and their default ports and protocols:

![Firewall considerations.](../diagrams/structurizr-Firewall.png)

```{admonition} Feature in development
:class: important
Landscape integration is an in-development feature of Ubuntu Pro for WSL.

The application currently does not support establishing a connection with a Landscape server.
```

The following table lists the default ports and protocols used by Ubuntu Pro for WSL:

| Description | Client System | Server System | Protocol | Default Port | Target address |
|-------------|---------------|---------------|----------|--------------|----------------|
| Required for online installation of WSL instances[^1].|Windows Host / Pro Agent |MS Store | tcp | https (443) | See [Microsoft documentation](https://learn.microsoft.com/en-us/windows/privacy/manage-windows-11-endpoints) for a list of addresses to allow. |
| Ubuntu Pro enablement[^2] | Windows Host / Pro Agent |Canonical Contract Server |tcp | https (443) | `contracts.canonical.com` |
| Landscape management[^2] | Windows Host / Pro Agent | Landscape Server | tcp | grpc (6554) | On-premise Landscape address |
| WSL instance management on the Windows host. Firewall rules set up at installation time of the WSL Pro agent. | WSL Instance / wsl-pro-service | Windows Host / Pro Agent | tcp | grpc (dynamic:49152-65535) | Hyper-V Virtual Ethernet Adapter IP |
| Ubuntu Pro[^2][^3]. | WSL Instance / Ubuntu Pro client | Canonical Contract Server | tcp | https (443) | `contracts.canonical.com` |
| Landscape[^2]. |  WSL Instance / Ubuntu Pro client | Landscape Server | tcp | https (443) | On-premise Landscape address |

If the client system is behind a proxy, ensure that the proxy is configured to allow the required connections.

[^1]: [Access to the Microsoft Store](https://learn.microsoft.com/en-us/windows/privacy/manage-windows-11-endpoints) is only required for online installations of Ubuntu that use the Microsoft Store. Without the Microsoft Store, Pro for WSL will still be functional. As outlined in the [installation guide](howto::install-ubuntu-wsl), you can install Ubuntu 24.04 LTS (Noble Numbat) or later from the terminal without the Microsoft Store and you can install official instances of Ubuntu downloaded from [releases.ubuntu.com](https://releases.ubuntu.com). You can also manage installations centrally from Landscape using [custom tarballs](howto::custom-distro).

[^2]: Access to the contract server and Landscape server is required for proper operation of Pro for WSL.

[^3]: For air-gapped installation refer to the [Ubuntu Pro documentation](https://canonical-ubuntu-pro-client.readthedocs-hosted.com/en/latest/explanations/using_pro_offline/).
