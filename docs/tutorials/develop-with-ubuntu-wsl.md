---
myst:
  html_meta:
    "description lang=en":
      "Set up an Ubuntu development environment on Windows using WSL, with Visual Studio Code for remote development and local testing in the browser."
---

# Develop with Ubuntu on WSL

Ubuntu on WSL can be used as a powerful development environment on Windows and
offers excellent integration with developer tools like Visual Studio Code.

## What you will learn

* Installing WSL and Ubuntu on WSL from the terminal
* Setting up Visual Studio Code for remote development with Ubuntu on WSL
* Creating a basic Node.js webserver on Ubuntu using Visual Studio Code
* Previewing HTML served from an Ubuntu WSL instance in a native browser on Windows

## What you will need

* A machine running Windows 10 or 11

## Install Ubuntu on WSL

### Install WSL

You need to install and enable WSL before you can start using Ubuntu on WSL.

Open a PowerShell prompt and run:

```{code-block} text
> wsl --install
```

You may be prompted to grant permission to continue the installation.

This command will install and enable the features necessary to run WSL.

**After running this command, you need to reboot your machine.**

```{admonition} What if WSL is already installed and enabled?
:class: important
If WSL is already set up on your machine, running `wsl --install` will install
the default WSL distro, which is the latest version of Ubuntu.

If an instance named Ubuntu already exists, an installation of Ubuntu will be
initiated but it will fail.
```

### Install Ubuntu on WSL

For a list of distributions that you can install on WSL, run:

```{code-block} text
> wsl --list --online
```

To install Ubuntu 24.04 LTS, run the following command in a PowerShell terminal:

```{code-block} text
> wsl --install Ubuntu-24.04
```

After the distribution is installed, you are prompted to create a
username and password. An Ubuntu session will then start automatically.

Changing from PowerShell to Ubuntu is indicated by a change in the terminal prompt.

**PowerShell prompt**:

```{code-block} text
:class: no-copy
PS C:\Users\username>
```

**Ubuntu prompt**:

```{code-block} text
:class: no-copy
username@pc:~$
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

```{admonition} What is version 2?
:class: note
WSL implements two different architectures for running 
Linux distributions: WSL1 and WSL2.
This means that you are using WSL2, rather than WSL1.
WSL2 is the default WSL on recent versions of Windows.
```

You can open a specific instance from PowerShell using its NAME:

```{code-block} text
> wsl ~ -d Ubuntu-22.04
```

The `~` is passed to the `wsl command` to start the instance in the Ubuntu home directory and
the `-d` flag is added before specifying a distro.

```{admonition} Windows terminal integration
:class: tip
Each time you install a version of Ubuntu, it appears in the dropdown list of
terminal profiles in Windows terminal.

If you have one version of Ubuntu running in a tab, you can open another in a
separate tab by selecting it from the menu.
```

We only need an Ubuntu-24.04 instance for this tutorial.

To remove the Ubuntu-22.04 instance, run the following command in PowerShell:

```{code-block} text
> wsl --unregister Ubuntu-22.04
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

Then open the directory in Visual Studio Code:

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

Return to your Ubuntu terminal (or use Visual Studio Code's integrated terminal) and type the following to install a server defined by the specification detailed in `package.json`:

```{code-block} text
$ npm install
```

Finally, start the web server:

```{code-block} text
$ npm start
```

You can now navigate to `localhost:10001` in your native Windows web browser by using <kbd>CTRL</kbd>+<kbd>LeftClick</kbd> on the link in the terminal.

![Windows desktop showing a web server being run from a terminal with "npm start", A Visual Studio Code project with a "hello world" html file, and a web browser showing the "hello world" page being served on local host.](assets/vscode/hello-world.png)

## Enjoy Ubuntu on WSL!

In this tutorial, we’ve shown you how to set up a development environment with Ubuntu on WSL and Visual Studio Code to create a basic Node.js webserver.

### Further Reading

* [Install Ubuntu on WSL2](../howto/install-ubuntu-wsl2.md)
* [Microsoft WSL Documentation](https://learn.microsoft.com/en-us/windows/wsl/)
* [Setting up WSL for Data Science](https://ubuntu.com/blog/wsl-for-data-scientist)
* [Ask Ubuntu](https://askubuntu.com/)
