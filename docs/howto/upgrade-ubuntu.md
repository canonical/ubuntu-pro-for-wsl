---
myst:
  html_meta:
    "description lang=en":
      "Upgrade instances of Ubuntu on WSL."
---

(howto::upgrade-instructions)=
# Upgrade Ubuntu on WSL

This page describes how to upgrade an installation of an {term}`LTS` release of
Ubuntu to a new version.

```{important}
Upgrades of Ubuntu instances require WSL 2 with {term}`systemd` enabled.

Read about the [differences between WSL 1 and WSL 2](explanation::wsl-version).
```

For a list of the available Ubuntu LTS releases, run `wsl --list --online`.

## Upgrading to an LTS release

To upgrade an instance of Ubuntu on WSL, run:

```{code-block} text
$ sudo apt update && sudo apt full-upgrade -y
$ sudo do-release-upgrade
```

When new LTS versions are released, the Ubuntu instance can be upgraded once
the first point release is available.

This upgrade behaviour is determined by the `Prompt` setting in
`release-upgrades`, which is set to `lts` by default:

```{code-block} text
:caption: /etc/update-manager/release-upgrades
:class: no-copy
Prompt=lts
```

:::{dropdown} Renaming your instance after an upgrade
:icon: light-bulb
If you install a numbered version of Ubuntu, such as `Ubuntu-22.04`, then
upgrade it to the next LTS (24.04), you may wish to rename your instance to
reflect that new version.

Our guide on instance management and configuration provides [instructions on
renaming instances](howto::renaming).
:::

## Upgrading to an interim release

If you want `do-release-upgrade` to upgrade an instance to **any new Ubuntu
release, including interim releases**, change the `Prompt` setting as follows:

```{code-block} diff
:caption: /etc/update-manager/release-upgrades
:class: no-copy
-Prompt=lts
+Prompt=normal
```

```{seealso}
A [detailed reference](reference::distros) on the Ubuntu releases available to install for WSL.
```
