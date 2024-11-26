(index-system-components)=

# UP4W components

Some Ubuntu Pro for WSL (UP4W) components run on the Windows host:

- The [UP4W Windows Agent](ref::up4w-windows-agent) that provides automation
services.
- The [UP4W GUI](ref::up4w-gui) for end users to manage their Ubuntu Pro
subscription and Landscape configuration.

UP4W also requires a component running inside each of the WSL distros:

- The [WSL Pro Service](ref::up4w-wsl-pro-service) communicates with the
Windows Agent to provide automation services.

Find additional detail on the individual components of UP4W below:

```{toctree}
:titlesonly:
:maxdepth: 1

up4w-windows_agent
up4w-gui
up4w-wsl_pro_service
landscape_client
ubuntu_pro_client
```

```{admonition} Supporting technologies
Windows Subsystem for Linux (**WSL**) makes it possible to run Linux
distributions on Windows. UP4W runs on Windows hosts to manage **Ubuntu WSL**
instances, automatically attaching them to an **Ubuntu Pro** subscription and
enrolling them into **Landscape**. For more information on these technologies,
you can refer to their official documentation:

* [Official Ubuntu WSL documentation](https://documentation.ubuntu.com/wsl/en/latest/)
* [Official Microsoft WSL documentation](https://learn.microsoft.com/en-us/windows/wsl/)
* [Official Landscape documentation](https://ubuntu.com/landscape/docs)

```
