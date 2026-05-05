(howto::upgrade-instructions)=
## Upgrade Ubuntu on WSL

The steps to upgrade an instance of Ubuntu on WSL depends on whether you have
installed the default Ubuntu release or a numbered release.

### Default Ubuntu release

Running `wsl --install` or `wsl --install Ubuntu` installs an {term}`instance`
of the default Ubuntu release.

The default always ships the latest stable LTS release.

You can upgrade this instance once the first point release of a new LTS is available:

```{code-block} text
$ sudo apt update && sudo apt full-upgrade 
$ sudo do-release-upgrade
```

### Numbered Ubuntu release

Ubuntu instances can also be installed as an explicitly-numbered release, with
a command like `wsl --install Ubuntu-24.04`.

Running `do-release-upgrade` will not upgrade that instance, unless the
following configuration change is made:

```{code-block} diff
:caption: /etc/update-manager/release-upgrades
:class: no-copy
- Prompt=never
+ Prompt=normal
```

### LTS versions of Ubuntu are recommended

We recommend LTS versions of Ubuntu on WSL, which receive standard support for
five years.

You are welcome to try interim releases of Ubuntu but they are not recommended
for production.

```{seealso}
A [detailed reference](reference::distros) on the Ubuntu releases available to install for WSL.
```
