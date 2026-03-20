---
myst:
  html_meta:
    "description lang=en":
      "Back up, name, and configure instances of Ubuntu on WSL."
---

(howto::manage-and-configure)=
# Manage and configure instances of Ubuntu on WSL

This page includes guidance on managing instances of Ubuntu on WSL.

(howto::instance-management-resources)=
## Additional resources

We provide a reference for instance configuration to support quick lookup of
configuration methods, including: [WSL Settings](ref::wsl-settings),
[`.wslconfig`](ref::.wslconfig), [`wsl.conf`](ref::wsl.conf),
[Cloud-init](ref::cloud-init), and [Ubuntu Pro for WSL](ref::up4w):

* [WSL instance configuration reference](ref::instance-config)

Microsoft's official documentation has general guidelines for:

* [Managing WSL instances using `wsl.exe` commands](https://learn.microsoft.com/en-us/windows/wsl/basic-commands)
* [Configuring WSL with global and per-instance settings](https://learn.microsoft.com/en-us/windows/wsl/wsl-config)

(howto::backup)=
## Backing up an instance

To backup an Ubuntu-24.04 instance, first make a `backup` folder:

```{code-block} text
:caption: C:\Users\\\<username>
> mkdir backup
```

You then need to create a compressed version of the Ubuntu instance in that backup directory:

```{code-block} text
:caption: C:\Users\\\<username>
> wsl --export Ubuntu-24.04 .\backup\Ubuntu-24.04.tar.gz
```

(howto::removal)=
### Removing and deleting the instance

Once you have created a backup of your Ubuntu instance it is safe to
Once you have created a backup of your Ubuntu instance, it is safe to
remove it from WSL and delete all associated data.

This can be achieved with the following command:

```{code-block} text
> wsl --unregister Ubuntu-24.04
```

(howto::restoring)=
### Restoring the backed-up instance

To restore the Ubuntu-24.04 instance that you have previously backed up,
you need to pass the name of the instance, the install location, and the name of the
backup to `wsl --import`:

```{code-block} text
:caption: C:\Users\\\<username>
> wsl --import Ubuntu-24.04 .\backup\Ubuntu2404\ .\backup\Ubuntu-24.04.tar.gz
```

This will import your previous data and if you run `wsl -d Ubuntu-24.04`, an Ubuntu WSL instance
should be restored with your previous configuration intact.

To login as a user `k`, created with the original instance, run: 
To log in as a user `k`, created with the original instance, run: 

```{code-block} text
> wsl -d Ubuntu-24.04 -u k
```

Alternatively, add the following to `/etc/wsl.conf` in the instance:

```text
[user]
default=k
```

Without specifying a user you will be logged in as the root user.

(howto::naming)=
## Naming instances

Assigning unique names to instances can be helpful if you want to install the
same base distribution for different projects and/or with different
configurations.

(howto::duplication)=
### Creating duplicate instances with unique names

It is also possible to create multiple instances from a base instance.
Here, the restore process is repeated but the new instances are assigned
different names than the original backup:

```{code-block} text
:caption: C:\Users\\\<username>
> wsl --import ubuntu2404b .\backup\Ubuntu2404b\ .\backup\Ubuntu-24.04.tar.gz
> wsl --import ubuntu2404c .\backup\Ubuntu2404c\ .\backup\Ubuntu-24.04.tar.gz
```

This will create two additional instances of Ubuntu 24.04 that can be launched and configured independently.
This will create two additional instances of Ubuntu 24.04 with unique names
that can be launched and configured independently.

In PowerShell, running `wsl -l -v` will output the new instances in your list of installed instances:

```{code-block} text
:class: no-copy
NAME            STATE         VERSION
Ubuntu-24.04    Stopped       2
ubuntu2404b     Stopped       2
ubuntu2404c     Stopped       2
```

To launch the first derived instance and log in as the user `k` run:

```{code-block} text
> wsl -d ubuntu2404b -u k
```

(howto::renaming)=
### Renaming an existing instance using the registry

Stop all WSL instances:

```{code-block} text
> wsl --shutdown
```

Open the registry editor and go to [the section on naming instances](howto::naming).

`HKEY_CURRENT_USER\SOFTWARE\Microsoft\Windows\CurrentVersion\Lxss`

Find the instance you want to rename and change the value for
`DistributionName`.

Run `wsl -l -v` to check if the renamed distro is listed.

(howto::new-instance-name)=
### Creating a new instance with a custom name

The `--name` flag can be used with `wsl.exe` to customise
the name of an instance during installation:

```{code-block} text
> wsl --install Ubuntu-24.04 --name UbuntuWebDev
```

The instance can then be launched as normal:

```{code-block} text
> wsl -d UbuntuWebDev
```

(howto::auto-config-instances)=
## Automatic configuration of instances

New instances can be configured automatically using cloud-init.

This can be used for automatic configuration of local or remote instances.

(howto::auto-config-cloud-init)=
### Automatic configuration of local instances

Cloud-init can be used to automatically configure an Ubuntu instance
during installation.

You create the configuration files in the `.cloud-init` directory, which
must be located in your Windows home directory.

Create a `.user-data` file that matches the instances that will be installed
and configured:

```{code-block} text
:class: no-copy
C:\Users\<user>
└── .cloud-init
    ├── Ubuntu-22.04.user-data
    └── Ubuntu-24.04.user-data
```

In this case, `Ubuntu-22.04` and `Ubuntu-24.04` will be automatically configured on installation.

You can create unique cloud-init configuration setups for one distribution (e.g., Ubuntu 24.04),
as long as you are installing instances with a unique name.
You can create multiple unique cloud-init configuration setups for a single distribution (e.g., Ubuntu 24.04),
as long as you are installing instances of that distribution that have been assigned unique names.
as long as you are installing instances of the distribution that have been assigned unique names.

Extending the previous example, you can add instance configurations for specific projects:

```{code-block} text
:class: no-copy
C:\Users\<user>
└── .cloud-init
    ├── Ubuntu-22.04.user-data
    ├── Ubuntu-24.04.user-data
    ├── Ubuntu-web-dev.user-data
    └── Ubuntu-data-science.user-data
```

For details on the contents of these configuration files, read our dedicated [cloud-init guide](howto::cloud-init).
```{admonition} How to write a cloud-init configuration
:class: tip
For details on the contents of cloud-init configuration files, read our dedicated [cloud-init guide](howto::cloud-init).
```

While there is a configuration for a regular `Ubuntu-24.04` installation, there are also configurations
for instances of Ubuntu 24.04 that have been assigned a specific name:

```{code-block} text
> wsl --install Ubuntu-24.04 --name UbuntuWebDev
> wsl --install Ubuntu-24.04 --name UbuntuDataScience
```

For guidance on assigning unique names to instances, go to [the section on
instance naming](howto::naming).

(howto::auto-config-remote)=
### Automatic configuration of remote instances

If you centrally manage remote Pro-attached Ubuntu instances using Landscape,
you can create WSL Profiles to deploy to Windows machines.

These profiles are based on cloud-init.

For more detail, refer to our dedicated tutorial on [Landscape
deployment](tut::deploy).

(howto::custom-image-configuration)=
## Configuration with custom images

If you want to control the default configurations and packages available
at the distro-level, you can create a custom Ubuntu image for WSL that
can be shared and distributed.

For more detail, refer to our dedicated guide on [customizing an Ubuntu
image](howto::custom-distro).
