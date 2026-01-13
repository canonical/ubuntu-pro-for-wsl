## Ubuntu Pro for WSL

<p align="center">
  <a href="https://documentation.ubuntu.com/wsl/stable/tutorials/getting-started-with-up4w/">Get started with Ubuntu Pro for WSL</a> •
  <a href="https://documentation.ubuntu.com/wsl/stable/howto/enforce-agent-startup-remotely-registry/">Remotely enforce startup of Pro agent</a> •
  <a href="https://ubuntu.com/pro/subscribe">Get an Ubuntu Pro subscription</a>
</p>

**Ubuntu Pro for WSL** is a Windows application that automates the
attachment of your Ubuntu Pro subscription. It solves the problem of needing to
manually Pro-attach each new Ubuntu instance created on a Windows machine when
you want the security benefits of Ubuntu Pro. For organisations, it enables
automated Pro-attachment at scale for fleets of devices.

## Basic usage of Ubuntu Pro for WSL

Find and install the Ubuntu Pro for WSL application in the Microsoft Store.

After installation, open the application and enter your Ubuntu Pro token.

> [!TIP]
> If you are a system administrator, you can also use the Windows registry to
> add a Pro token. After the Ubuntu Pro app has run at least once, the relevant
> key will be available as `HKEY_CURRENT_USER\Software\Canonical\UbuntuPro`. The
> Pro token can be added as data for the `UbuntuProToken` value.

Now, all instances of Ubuntu on WSL will be automatically Pro-attached on your machine.

You can confirm this in any Ubuntu instance with:

```
pro status
```

## Documentation

Our [official documentation](https://documentation.ubuntu.com/wsl/stable/)
includes tutorials, guides, references, and explanations on both:

* The Ubuntu on WSL distribution
* The Ubuntu Pro for WSL application

Documentation is maintained in the `docs` directory of this repository. It is
written in markdown, built with Sphinx, and published on [Read the Docs](https://about.readthedocs.com/).

## Contribute

This is an open source project and we warmly welcome community contributions, suggestions, and constructive feedback. If you're interested in contributing, please take a look at our [Contribution guidelines](CONTRIBUTING.md) first.

* To report a bug, please create a new issue in this repository, using the Report an issue template.
* For suggestions and constructive feedback, use the Request a feature template.

## Project and community

We're friendly! We have a community forum at [https://discourse.ubuntu.com](https://discourse.ubuntu.com) where we discuss feature plans, development news, issues, updates and troubleshooting.

