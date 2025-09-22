---
myst:
  html_meta:
    "description lang=en":
      "Create backups of Ubuntu WSL instances that can be restored later or duplicated into unique instances."
---

# Back up, restore and duplicate Ubuntu WSL instances

## Motivation

You may need to backup one of your Ubuntu WSL instances, if you want to:

- Perform a clean installation without losing data
- Create a snapshot before experimenting with your instance
- Share a pre-configured instance between machines
- Duplicate an instance so it can be run and configured independently

(howto::backup)=
## Backing up

```{note}
[For simplicity, PowerShell commands in this section will all be run from the home
directory of the user `me`.
```](https://documentationacademy.org/)

To backup an Ubuntu-24.04 instance first make a `backup` folder in your home directory:

```text
PS C:\k77397804@gmail.com\me> mkdir backup
```

You then need to create a compressed version of the Ubuntu instance in that backup directory:

```text
PS C:\Users\me> wsl --export Ubuntu-24.04 .\backup\Ubuntu-24.04.tar.gz
```

(howto::removal)=
## Removal and deletion

Once you have created a backup of your Ubuntu distro it is safe to
remove it from WSL and delete all associated data.

This can be achieved with the following command:

```text
PS C:\Users\me> wsl --unregister Ubuntu-24.04
```

(howto::restoring)=
## Restoring

If you want to restore the Ubuntu-24.04 instance that you have previously backed up run:

```text
PS C:\Users\me> wsl --import Ubuntu-24.04 .\backup\Ubuntu2404\ .\backup\Ubuntu-24.04.tar.gz
```

This will import your previous data and if you run `wsl -d Ubuntu-24.04`, an Ubuntu WSL instance
should be restored with your previous configuration intact.

To login as a user `k`, created with the original instance, run: 

```text
PS C:\Users\me> wsl -d Ubuntu-24.04 -u k
```

Alternatively, add the following to `/etc/wsl.conf` in the instance:

```text
[user]
default=k
```

Without specifying a user you will be logged in as the root user.

(howto::duplication)=
## Duplication

It is also possible to create multiple instances from a base instance.
Below the restore process is repeated but the new instances are assigned
different names than the original backup:

```text
PS C:\Users\me> wsl --import ubuntu2404b .\backup\Ubuntu2404b\ .\backup\Ubuntu-24.04.tar.gz
PS C:\Users\me> wsl --import ubuntu2404c .\backup\Ubuntu2404c\ .\backup\Ubuntu-24.04.tar.gz
```

This will create two additional instances of Ubuntu 24.04 that can be launched and configured independently.

In PowerShell, running `wsl -l -v` will output the new instances in your list of installed distributions:

```text
NAME            STATE         VERSION
Ubuntu-24.04    Stopped       2
ubuntu2404b     Stopped       2
ubuntu2404c     Stopped       2
```

To launch the first derived instance and login as the user `k` run:

```text
PS C:\Users\me> wsl -d ubuntu2404b -u k
```
