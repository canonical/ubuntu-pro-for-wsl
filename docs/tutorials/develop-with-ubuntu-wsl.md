---
myst:
  html_meta:
    "description lang=en":
      "Set up an Ubuntu development environment on Windows using WSL, with Visual Studio Code for remote development and local testing in the browser."
---

# Develop with Ubuntu on WSL

Ubuntu on WSL can be used as a powerful development environment on Windows and
offers excellent integration with developer tools like Visual Studio Code.

```{include} ../includes/prompt_symbols_notice.txt
    :start-after: <!-- Include start prompt symbols -->
    :end-before: <!-- Include end prompt symbols -->
```

## What you will learn

* Installing WSL and Ubuntu on WSL from the terminal
* Setting up Visual Studio Code for remote development with Ubuntu on WSL
* Creating a basic Node.js webserver on Ubuntu using Visual Studio Code
* Previewing HTML served from an Ubuntu WSL instance in a native browser on Windows

## What you will need

* Windows 11 (recommended) or Windows 10 with minimum version 21H2 on a physical machine

```{include} ../includes/virtualisation_requirements.txt
    :start-after: <!-- Include start virtualisation requirements -->
    :end-before: <!-- Include end virtualisation requirements -->
```

## Install Ubuntu on WSL

You can check if WSL is already installed by trying to display its version information.

Open PowerShell and run the following command:

```{code-block} text
> wsl --version
```

If WSL **is not** installed, press <kbd>Ctrl</kbd>+<kbd>C</kbd> to cancel, and then
follow our steps to [install WSL](tut::develop::install-wsl).

When WSL **is** already installed, follow our steps to [install Ubuntu](tut::develop::install-ubuntu).

(tut::develop::install-wsl)=
### Install WSL

In PowerShell, run the following command to install and enable WSL:

```{code-block} text
> wsl --install
```

You may need to reboot your machine for the changes to take effect.

You may also be prompted to grant permissions during the installation process.

(tut::develop::install-ubuntu)=
### Install Ubuntu

To check if you have any Ubuntu distributions installed on WSL, run:

```{code-block} text
> wsl --list
```

Ubuntu is the default distribution for WSL. It may be listed in the output
of the command if it was included with your WSL installation:

```{code-block} text
:class: no-copy
Ubuntu (Default)
```

For a list of distributions that you can install on WSL, run:

```{code-block} text
> wsl --list --online
```

For this tutorial, install Ubuntu 24.04 LTS by running the following command in
PowerShell:

```{code-block} text
> wsl --install Ubuntu-24.04
```

:::{dropdown} (Optional) Installing multiple instances of the same Ubuntu release
:color: success
:icon: light-bulb

If you already have an `Ubuntu-24.04` instance and you don't want to change or
remove it, you can install a second instance by giving it a unique name:

```{code-block} text
> wsl --install Ubuntu-24.04 --name Ubuntu-tutorial
```

You can then run that instance with:

```{code-block} text
> wsl -d Ubuntu-tutorial
```

If using a distribution with a custom name when following this tutorial, don't
forget to substitute your custom name for `Ubuntu-24.04` in the commands.
:::

After an Ubuntu distribution is installed, you are prompted to create a
username and password. An Ubuntu session will then start automatically.

Changing from PowerShell to Ubuntu is indicated by a change in the terminal
prompt, for example:

**PowerShell prompt**

```{code-block} text
:class: no-copy
PS C:\Users\windows-username>
```

**Ubuntu prompt**:

```{code-block} text
:class: no-copy
ubuntu-username@hostname:~$
```

To exit the Ubuntu terminal at any time, type the `exit` command and execute it
by pressing <kbd>enter</kbd>, which will return you to the PowerShell prompt.


```{admonition} Different methods to install Ubuntu on WSL
:class: tip
There are multiple ways of installing Ubuntu on WSL, here we focus on using the
terminal. For more detail on installation methods for Ubuntu on WSL, refer to
our [dedicated installation guide](../howto/install-ubuntu-wsl2.md).
```

### Running multiple versions of Ubuntu

You can install multiple versions of Ubuntu on WSL. Each Ubuntu instance can
then be used as a separate, self-contained development environment.

```{code-block} text
> wsl --install Ubuntu-22.04
```

Use `wsl -l -v` to list all of your installed distros.

```{code-block} text
:class: no-copy
  NAME            STATE           VERSION
  Ubuntu-22.04    Stopped         2
* Ubuntu-24.04    Stopped         2
```

This shows that both distros are stopped, that each uses WSL 2, and that
Ubuntu-24.04 is the default distro.

```{dropdown} What is WSL 2?
:open:
:color: warning
:icon: alert

WSL 2 is the default WSL architecture on recent versions of Windows and it is
recommended for this tutorial.

Read more about the [differences between WSL versions](explanation::wsl-version).
```

You can open a specific instance from PowerShell using its NAME:

```{code-block} text
> wsl ~ -d Ubuntu-22.04
```

The `~` is passed to the `wsl command` to start the instance in the Ubuntu home
directory, which is commonly symbolised by ~. The `-d` flag is added to specify the
distro.

We only need an Ubuntu-24.04 instance for this tutorial.

To remove the Ubuntu-22.04 instance, run the following command in PowerShell:

```{code-block} text
> wsl --unregister Ubuntu-22.04
```

```{admonition} Windows terminal integration
:class: tip
Each time you install a version of Ubuntu, it appears in the dropdown list of
terminal profiles in Windows terminal.

If you have one version of Ubuntu running in a tab, you can open another in a
separate tab by selecting it from the menu.
```

## Install Visual Studio Code on Windows

One of the advantages of WSL is its integration with native Windows applications, such as Visual Studio Code.

Open Microsoft Store on your Windows machine, search for "Visual Studio Code" and install the application.

When selecting additional tasks during setup, ensure the {guilabel}`Add to PATH` option is checked.

![Visual Studio Code's "Additional Tasks" setup dialog with the "Add to Path" and "Register Code as an editor for supported file types" options checked.](assets/vscode/aditional-tasks.png)

Once the installation is complete, open Visual Studio Code.

## Install the Remote Development Extension

Navigate to the {guilabel}`Extensions` menu in the sidebar and search for "Remote Development".

**Remote Development** is an extension pack that allows you to open any folder in a container, remote machine, or WSL.

If you only want the features that support WSL, install the **Remote - WSL** extension instead.

![Installation page for the Remote Development Visual Studio Code extension.](assets/vscode/remote-extension.png)

Once installed, you can test the development environment by creating an example local web server with Node.js

## Install Node.js and create a new project

Open an Ubuntu terminal using the `wsl ~ -d Ubuntu-24.04` command.

Ensure the packages in Ubuntu are up-to-date with the following command:

```{code-block} text
$ sudo apt update && sudo apt upgrade -y
```

Next, install Node.js and npm:

```{code-block} text
$ sudo apt-get install nodejs
$ sudo apt install npm
```

Create a directory for your server.

```{code-block} text
$ mkdir serverexample/
```

Change into that directory:

```{code-block} text
$ cd serverexample/
```

Then open the current directory (`.`) in Visual Studio Code:

```{code-block} text
$ code .
```

The first time you run `code` from Ubuntu, it will trigger a download of the necessary dependencies:

```{code-block} text
:class: no-copy
Installing VS Code Server for x64...
Downloading:
```

Once complete, your native version of Visual Studio Code will open the folder.

## Creating a basic web server

In Visual Studio Code, create a new `package.json` file and add the following text:

```{code-block} json
:caption: serverexample/package.json
{
    "name": "Demo",
    "version": "1.0.0",
    "description": "demo project.",
    "scripts": {
        "lite": "lite-server --port 10001",
        "start": "npm run lite"
    }, 
    "author": "",
    "license": "ISC",
    "devDependencies": {
        "lite-server": "^1.3.1"
    }
}
```

Save the file and then --- in the same folder --- create a new one called `index.html`

Add the following text, then save and close:

```{code-block} html
:caption: serverexample/index.html
<h1>Hello World</h1>
```

Return to your Ubuntu terminal (or use Visual Studio Code's integrated terminal) and run the following command from within the project directory to install a server defined by the specification detailed in `package.json`:

```{code-block} text
:caption: ~/serverexample
$ npm install
```

Still inside the project directory, start the web server:

```{code-block} text
:caption: ~/serverexample
$ npm start
```

You can now navigate to `localhost:10001` in your native Windows web browser by using <kbd>CTRL</kbd>+<kbd>LeftClick</kbd> on the link in the terminal.

![Windows desktop showing a web server being run from a terminal with "npm start", A Visual Studio Code project with a "hello world" html file, and a web browser showing the "hello world" page being served on local host.](assets/vscode/hello-world.png)

## Enjoy Ubuntu on WSL!

In this tutorial, we’ve shown you how to set up a development environment with Ubuntu on WSL and Visual Studio Code to create a basic Node.js webserver.

### Further Reading

* [Install Ubuntu on WSL 2](../howto/install-ubuntu-wsl2.md)
* [Microsoft WSL Documentation](https://learn.microsoft.com/en-us/windows/wsl/)
* [Setting up WSL for Data Science](https://ubuntu.com/blog/wsl-for-data-scientist)
* [Ask Ubuntu](https://askubuntu.com/)
