---
myst:
  html_meta:
    "description lang=en":
      "Upgrade instances of Ubuntu on WSL."
---

(howto::upgrade-instructions)=
## Upgrade Ubuntu on WSL

To upgrade an instance of Ubuntu on WSL, run:

```{code-block} text
$ sudo apt update && sudo apt full-upgrade -y
$ sudo do-release-upgrade
```

Note that the upgrade behaviour in WSL depends on whether you are upgrading an
instance of a default `Ubuntu` release or a numbered Ubuntu release, such as
`Ubuntu-22.04`.

### Default Ubuntu release

Running `wsl --install` or `wsl --install Ubuntu` installs an {term}`instance`
of the default Ubuntu release.

The default always ships the latest stable LTS release of Ubuntu.

The `Prompt` setting in `release-upgrades` for this default is set to `normal`:

```{code-block} text
:caption: /etc/update-manager/release-upgrades
:class: no-copy
Prompt=normal
```

Run `do-release-upgrade` to upgrade this instance to **any new Ubuntu release if
it is available**, including interim releases.

### Numbered Ubuntu release

Ubuntu instances can also be installed as an explicitly-numbered release, with
a command like `wsl --install Ubuntu-22.04`.

The `Prompt` setting in `release-upgrades` for numbered releases is set to `lts`:

```{code-block} text
:caption: /etc/update-manager/release-upgrades
:class: no-copy
Prompt=lts
```

Run `do-release-upgrade` to upgrade that instance to the **next available LTS if
it is available**.

### LTS versions of Ubuntu are recommended

We recommend LTS versions of Ubuntu on WSL, which receive standard support for
five years.

You are welcome to try interim releases of Ubuntu but they are not recommended
for production.

If you want to prevent distribution upgrades for an instance, set `Prompt` to
`never`:

```{code-block} diff
:caption: /etc/update-manager/release-upgrades
:class: no-copy
Prompt=never
```

```{seealso}
A [detailed reference](reference::distros) on the Ubuntu releases available to install for WSL.
```
