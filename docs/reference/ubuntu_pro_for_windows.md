# Ubuntu Pro for Windows

> See also [Ubuntu Pro](https://discourse.ubuntu.com/t/ubuntu-pro-faq/34042) and [Landscape](https://ubuntu.com/landscape/docs)

Ubuntu Pro for Windows is an automation tool running on Windows hosts to manage
Ubuntu WSL instances, providing them with compliance by attaching them to
Ubuntu Pro and enrolling them into Landscape, made available through the
Microsoft Store.

Ubuntu Pro for Windows is made of:
 - a [background agent](windows_agent) providing the automation services and
 - a [graphical user interface](ubuntu_pro_for_windows_gui) for end users to configure Landscape and acquire or set their Ubuntu Pro Token

Additionally, only instances having the wsl-pro-service component installed are
capable of interacting with this application.
