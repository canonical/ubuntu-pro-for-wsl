---
myst:
  html_meta:
    "description lang=en":
      "A top-level explanation of the Ubuntu Pro for WSL application."
---

# What is Ubuntu Pro for WSL?

Ubuntu Pro for WSL is a Windows application for securing and managing instances
of Ubuntu on WSL.

It serves two primary functions:

* Attaching Ubuntu instances to a Pro subscription
* Configuring Ubuntu instances for management by Landscape

This page includes a top-level explanation of Pro for WSL.
More technical detail is provided in the page on Pro for WSL's
[architecture](./ref-arch-explanation.md).

## Ubuntu Pro is a subscription service

For individuals, [Ubuntu Pro](https://ubuntu.com/pro) is free for use with up
to five machines. Enterprise users can pay for subscriptions that cover larger
numbers of machines.

A [Pro token](https://ubuntu.com/pro/subscribe) entitles you to extended
security updates, compliance features, and
[support](https://documentation.ubuntu.com/landscape/how-to-guides/wsl-integration/get-support/).
It is also bundled with [Landscape](https://ubuntu.com/landscape) SaaS, which
can be used to centrally manage remote Ubuntu instances with minimal setup.

```{tip} 
Learn how to deploy WSL with Landscape SaaS in our [deployment
tutorial](tut::deploy).
```

In an enterprise scenario, the Pro for WSL application enables you to automate
Pro-attachment and Landscape-enrollment of Ubuntu instances organisation-wide.

## Pro for WSL automates Pro-attachment

WSL users often create multiple instances. For example, they might have an
Ubuntu instance for web development projects and another instance for machine
learning. They could be running a specific project on an older version of
Ubuntu for compatibility reasons. Users may also create temporary short-lived
instances for quick experiments.

Within an organization, there are typically multiple Windows machines being
operated by different users, and each of those machines could potentially be
running several Ubuntu instances. This can be a concern for IT managers, as
it is difficult to ensure that WSL instances are secure and compliant using
conventional tooling for remote management.

A Pro subscription addresses these issues by offering enhanced security
maintenance, updates, and patching. However, it may be impractical to send
individual users a Pro token and ask them to run `pro attach <token>` each time
they create an instance. Organisations could limit or prevent the creation of
WSL instances, but then users will not have access to a powerful development
tool on their Windows machine.

When Pro for WSL is set up on a fleet of Windows machines, it means that any
Ubuntu instance in that fleet is automatically Pro-attached, and automatically
gets the benefits associated with the Pro subscription. IT managers can then
empower users to securely create as many WSL instances as they need.

## Landscape can manage Pro-attached instances

When Pro for WSL is installed on a Windows machine, and a Pro subscription is
attached, the application can be used to [configure
Landscape](../howto/set-up-landscape-client.md).

If you are using Landscape SaaS, the configuration is as simple as supplying
the account name for the Landscape dashboard.

With Landscape configured, a central administrator can create WSL profiles on
Landscape that can then be used to [remotely deploy instances](tut::deploy)
on Windows machines, with specific permissions enabled and packages installed.

```{tip}
For examples of other common tasks that you can perform with landscape for
managed WSL instances, refer to the [Landscape
documentation](https://documentation.ubuntu.com/landscape/how-to-guides/wsl-integration/perform-common-tasks/).
```

## Pro for WSL has a GUI interface for individual users

The Windows application has a GUI interface that is convenient for adding a Pro
subscription and entering a Landscape configuration.

The GUI is especially useful for individuals who want to secure and manage a
small number of machines, or for the testing of Pro for WSL features.

Enterprise users, especially system administrators, may wish to bypass the GUI
completely, and instead configure Pro for WSL using the Windows registry.

## Pro for WSL has registry keys for enterprise users

Pro tokens and Landscape configurations can also be applied to machines [using
the Windows registry](../reference/windows_registry.md). Using the registry
makes it possible to Pro-attach and Landscape-enroll Windows machines at scale
using a Windows remote management solution, like
[Intune](https://www.microsoft.com/en-ie/security/business/microsoft-intune).

When setting up Pro for WSL remotely, you need to ensure that Pro for WSL agent
is running in the background. This can be done in a variety of ways, and we
provide examples using Intune for [starting the agent remotely using the
registry](../howto/enforce-agent-startup-remotely-registry.md) or by [through
the use of a platform script](../howto/start-agent-remotely.md).

## Do I need to use Pro for WSL?

Pro for WSL is useful when you need to automate Pro-attachment for multiple
Ubuntu instance, and/or you are interested in managing multiple instances of
Ubuntu remotely.

For everyone else, Ubuntu on WSL remains a fully-featured Ubuntu environment 
whether Pro for WSL is installed or not.

If you want to leverage a free Pro subscription for a small number of
instances, you can still [pro-attach Ubuntu instances
manually](https://documentation.ubuntu.com/pro-client/en/latest/howtoguides/how_to_attach/)
without the app:

```{code-block} text
pro attach <token>
```

This, however, may not be scalable.
