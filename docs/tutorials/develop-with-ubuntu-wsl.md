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

Open a PowerShell prompt as an Administrator and run:

```{code-block} text
> wsl --install
```

This command will enable the features necessary to run WSL and installs the
latest Ubuntu distribution available for WSL.

As this step creates an Ubuntu instance, you will be prompted to create a username
and password. An Ubuntu terminal will then open automatically.

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

To exit the Ubuntu terminal, type the `exit` command and execute it by pressing
<kbd>enter</kbd>, which will return you to the PowerShell prompt.

```{tip}
It is recommended to reboot your machine after this initial installation to
complete the setup.
```

### Install a specific version of Ubuntu on WSL

There are multiple ways of installing Ubuntu on WSL, here we focus on using the
terminal. For more detail on installation methods for Ubuntu on WSL, refer to
our [dedicated installation guide](../howto/install-ubuntu-wsl2.md).

To install Ubuntu 24.04 LTS, run the following command in a PowerShell terminal:

```{code-block} text
> wsl --install Ubuntu-24.04
```

You'll see an indicator of the installation progress in the terminal:

```{code-block} text
:class: no-copy
Installing: Ubuntu 24.04 LTS
[==========================72,0%==========                 ]
```

```{note}
WSL supports a variety of Ubuntu releases. Read our [reference on distributions
of Ubuntu on WSL](../reference/distributions.md) for more information.
```

### Run a specific Ubuntu version

Use `wsl -l -v` to list all your installed distros and the version of WSL that they are using:

```{code-block} text
:class: no-copy
  NAME            STATE           VERSION
  Ubuntu          Stopped         2
* Ubuntu-24.04    Stopped         2
```

Two instances of Ubuntu are installed:

1. The default Ubuntu version that was installed automatically when you installed WSL
2. The numbered Ubuntu version that you installed manually

You can open a specific instance from PowerShell using its NAME:

```{code-block} text
> wsl ~ -d Ubuntu-24.04
```

The `~` is passed to the `wsl command` to start the instance in the Ubuntu home directory,
the `-d` flag is added before specifying a distro.

## Install Visual Studio Code on Windows

One of the advantages of WSL is its integration with native Windows applications, such as Visual Studio Code.

Search for "Visual Studio Code" in the Microsoft Store and install it.

![Installation page for Visual Studio Code on the Microsoft Store.](assets/vscode/msstore.png)

Alternatively, you can install Visual Studio Code from the [web link](https://code.visualstudio.com/Download).

![Visual Studio Code download page showing download options for Windows, Linux, and Mac.](assets/vscode/download-vs-code.png)

During installation, under the 'Additional Tasks' step, ensure the `Add to PATH` option is checked.

![Visual Studio Code's "Additional Tasks" setup dialog with the "Add to Path" and "Register Code as an editor for supported file types" options checked.](assets/vscode/aditional-tasks.png)

Once the installation is complete, open Visual Studio Code.

## Install the Remote Development Extension

Navigate to the `Extensions` menu in the sidebar and search for `Remote Development`.

This is an extension pack that allows you to open any folder in a container, remote machine, or in WSL. Alternatively, you can just install `Remote - WSL`.

![Installation page for the Remote Development Visual Studio Code extension.](assets/vscode/remote-extension.png)

Once installed you can test the development environment by creating an example local web server with Node.js

## Install Node.js and create a new project

Open an Ubuntu terminal using the `wsl ~ -d Ubuntu24.04` command.

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

The first time you do run `code` from Ubuntu, it will trigger a download of the necessary dependencies:

![Bash snippet showing the installation of Visual Studio Code Server's required dependencies.](assets/vscode/downloading-vscode-server.png)

Once complete, your native version of Visual Studio Code will open the folder.

## Creating a basic web server

In Visual Studio Code, create a new `package.json` file and add the following text:

```{code-block} json
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
<h1>Hello World</h1>
```

Now return to your Ubuntu terminal (or use the Visual Studio Code terminal window) and type the following to install a server defined by the above specifications detailed in `package.json`:

```{code-block} text
$ npm install
```

Finally, type the following to launch the web server:

```{code-block} text
$ npm start
```

You can now navigate to `localhost:10001` in your native Windows web browser by using <kbd>CTRL</kbd>+<kbd>LeftClick</kbd> on the terminal links.

![Windows desktop showing a web server being run from a terminal with "npm start", A Visual Studio Code project with a "hello world" html file, and a web browser showing the "hello world" page being served on local host.](assets/vscode/hello-world.png)

That’s it!

By using Ubuntu on WSL you’re able to take advantage of the latest Node.js packages available on Linux as well as the more streamlined command line tools.

## Enjoy Ubuntu on WSL!

In this tutorial, we’ve shown you how to connect the Windows version of Visual Studio Code to your Ubuntu on WSL filesystem and launch a basic Node.js webserver.

We hope you enjoy using Ubuntu inside WSL. Don’t forget to check out our other tutorials for tips on how to optimise your WSL setup for Data Science.

### Further Reading

* [Install Ubuntu on WSL2](../howto/install-ubuntu-wsl2.md)
* [Microsoft WSL Documentation](https://learn.microsoft.com/en-us/windows/wsl/)
* [Setting up WSL for Data Science](https://ubuntu.com/blog/wsl-for-data-scientist)
* [Ask Ubuntu](https://askubuntu.com/)
