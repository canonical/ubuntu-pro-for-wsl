---
myst:
  html_meta:
    "description lang=en":
      "Back up, name, and configure instances of Ubuntu on WSL."
---

(howto::manage-and-configure)=
# Manage and configure instances of Ubuntu on WSL

This page includes guidance on managing {term}`instances <instance>` of Ubuntu on WSL.

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

When backing up an instance, you can export a tarball or the virtual hard disk
({term}`VHD`) for the instance.

Using a VHD has the advantage of not requiring a compression/decompression step.

`````{tab-set}
:sync-group: backup

````{tab-item} Using tarballs
:sync: tarballs

To backup an Ubuntu-24.04 instance, first make a `backup` folder:

```{code-block} text
:caption: C:\Users\\\<username>
> mkdir backup
```

Then create a compressed version of the Ubuntu instance in that backup directory:

```{code-block} text
:caption: C:\Users\\\<username>
> wsl --export Ubuntu-24.04 .\backup\Ubuntu-24.04.tar.gz
```

````

````{tab-item} Using VHD
:sync: vhd
To backup an Ubuntu-24.04 instance, first make a `backup` folder:

```{code-block} text
:caption: C:\Users\\\<username>
> mkdir backup
```

Then create a `.vhdx` of the Ubuntu instance in that backup directory:

```{code-block} text
:caption: C:\Users\\\<username>
> wsl --export Ubuntu-24.04 .\backup\Ubuntu-24.04.vhdx --format vhd
```
````
`````

```{admonition} Virtual Hard Disk
:class: tip
To learn more about managing VHD for WSL, read Microsoft's [how to manage WSL disk space](https://learn.microsoft.com/en-us/windows/wsl/disk-space).
```

(howto::removal)=
### Removing and deleting the instance

Once you have created a backup of your Ubuntu instance, it is safe to
remove it from WSL and delete all associated data.

Remove the instance with the following command:

```{code-block} text
> wsl --unregister Ubuntu-24.04
```

(howto::restoring)=
### Restoring the backed-up instance

`````{tab-set}
:sync-group: backup

````{tab-item} Using tarballs
:sync: tarballs

To restore the Ubuntu-24.04 instance that you have previously backed up as a tarball:

```{code-block} text
:caption: C:\Users\\\<username>
> wsl --import Ubuntu-24.04 .\backup\Ubuntu2404\ .\backup\Ubuntu-24.04.tar.gz
```
````

````{tab-item} Using VHD
:sync: vhd
To restore the Ubuntu-24.04 instance that you have previously backed up as a VHD,
create a copy of the VHD:

```{code-block} text
:caption: C:\Users\\\<username>
> wsl --import Ubuntu-24.04 .\backup\Ubuntu2404\ .\backup\Ubuntu-24.04.vhdx --vhd
```

A quicker option is to import the already filled, ready-to-use, virtual hard drive:

```{code-block} text
:caption: C:\Users\\\<username>
wsl --import-in-place Ubuntu-24.04 .\backup\Ubuntu-24.04.vhdx
```
````
`````

### Using the restored backup

After restoring your backup of Ubuntu-24.04, it can be launched as normal.
The instance should be restored with your previous configuration intact.

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
same base {term}`distribution` for different projects and/or with different
configurations.

(howto::duplication)=
### Creating duplicate instances with unique names

It is possible to create multiple instances from a base instance. For example,
to create multiple new instances from an instance that has been backed up as a
tarball:

```{code-block} text
:caption: C:\Users\\\<username>
> wsl --import ubuntu2404b .\backup\Ubuntu2404b\ .\backup\Ubuntu-24.04.tar.gz
> wsl --import ubuntu2404c .\backup\Ubuntu2404c\ .\backup\Ubuntu-24.04.tar.gz
```

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

Open the {term}`registry <Windows registry>` editor and find:

`HKEY_CURRENT_USER\SOFTWARE\Microsoft\Windows\CurrentVersion\Lxss`

Each distribution is represented by a unique ID under `Lxss`.

Go to the WSL instance that you want to rename and change the value for
`DistributionName`.

Select any ID then change the value for `DistributionName` to rename that instance.

To confirm that the instance has been renamed, run

```{code-block} text
> wsl -l -v
```

(howto::new-instance-name)=
### Creating a new instance with a custom name

The `--name` flag can be used with {term}`wsl.exe` to customize
the name of an instance during installation:

```{code-block} text
> wsl --install Ubuntu-24.04 --name UbuntuWebDev
```

Then launch the instance as normal:

```{code-block} text
> wsl -d UbuntuWebDev
```

(howto::auto-config-instances)=
## Automatic configuration of instances

New instances can be configured automatically using {term}`cloud-init`.

This can be used for automatic configuration of local or remote instances.

(howto::auto-config-cloud-init)=
### Automatic configuration of local instances

When adding configuration files for cloud-init, use the `.cloud-init` directory, which
must be located in your Windows home directory.

To automatically configure an Ubuntu instance during installation,
first create a `.user-data` file that matches the instances that will be installed
and configured:

```{code-block} text
:class: no-copy
C:\Users\<user>
└── .cloud-init
    ├── Ubuntu-22.04.user-data
    └── Ubuntu-24.04.user-data
```

In this case, `Ubuntu-22.04` and `Ubuntu-24.04` will be automatically configured on installation.

You can create multiple unique cloud-init configuration setups for a single distribution (e.g., Ubuntu 24.04),
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

```{admonition} How to write a cloud-init configuration
:class: tip
For details on the contents of cloud-init configuration files, read our dedicated [cloud-init guide](howto::cloud-init).
```

Install instances of the Ubuntu-24.04 distribution that will have your unique configurations applied using the `--name` flag:

```{code-block} text
> wsl --install Ubuntu-24.04 --name UbuntuWebDev
> wsl --install Ubuntu-24.04 --name UbuntuDataScience
```

For guidance on assigning unique names to instances, go to [the section on
instance naming](howto::naming).

(howto::auto-config-remote)=
### Automatic configuration of remote instances

If you centrally manage remote {term}`Pro-attached <Pro-attachment>` Ubuntu instances using {term}`Landscape`,
you can create {term}`WSL profiles <WSL profile>` to deploy to Windows machines.

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
