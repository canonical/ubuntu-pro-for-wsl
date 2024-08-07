# Back up, restore and duplicate Ubuntu WSL instances

## Motivation

You may need to backup one of your Ubuntu WSL instances, if you want to:

- Perform a clean installation without losing data
- Create a snapshot before experimenting with your instance
- Share a pre-configured instance between machines
- Duplicate an instance so it can be run and configured independently

(howto::backup)=
## Backing up

To backup an Ubuntu-24.04 instance first make a `backup` directory:

```text
PS C:\Users\me\howto> mkdir C:\Users\me\howto\backup
```

You then need to create a compressed version of the Ubuntu instance in that backup directory:

```text
PS C:\Users\me\howto> wsl --export Ubuntu-24.04 .\backup\Ubuntu-24.04.tar.gz
```

(howto::removal)=
## Removal and deletion

Once you have created a backup of your Ubuntu distro it is safe to
remove it from WSL and delete all associated data.

This can be achieved with the following command:

```text
PS C:\Users\me\howto> wsl --unregister Ubuntu-24.04
```

(howto::restoring)=
## Restoring

If you want to restore the Ubuntu-24.04 instance that you have previously backed up run:

```text
PS C:\Users\me\tutorial> wsl --import Ubuntu-24.04 .\backup\Ubuntu-24.04 .\backup\Ubuntu-24.04.tar.gz
```

This will import your previous data and if you run `ubuntu2404.exe` an Ubuntu WSL instance
should be restored with your previous configuration intact.

(howto::duplication)=
## Duplication

It is also possible to create multiple instances from a base instance.
Below the restore process is repeated but the new instances are assigned
different names than the original backup:

```text
PS C:\Users\me\howto> wsl --import ubuntu24.04-b .\backup\ .\backup\Ubuntu-24.04.tar.gz
PS C:\Users\me\howto> wsl --import ubuntu24.04-c .\backup\ .\backup\Ubuntu-24.04.tar.gz
```

This will create two additional instances of Ubuntu 24.04 that can be launched and configured independently.
For example, launching one of these instances is achieved with the command:

```text
wsl -d ubuntu2404-b
```
