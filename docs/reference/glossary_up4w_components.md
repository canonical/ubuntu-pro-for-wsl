---
myst:
  html_meta:
    "description lang=en":
       "Comprehensive glossary for the Ubuntu Pro for WSL application."
---

(ref::glossary-up4w-components)=
# Glossary of Ubuntu Pro for WSL components

The architecture of Pro for WSL and how its components integrate together is covered in our detailed [explanation article](../explanation/ref-arch-explanation.md).
This glossary includes concise descriptions of the Ubuntu Pro for WSL components for reference.

We are currently compiling and defining terms for the Ubuntu Pro for WSL glossary. If you would
like to help, please visit our {ref}`contributions page <contribute>`
for details on how to get involved.

**Jump to:**

{ref}`A <terms_A>` -- {ref}`B <terms_B>` -- {ref}`C <terms_C>` --
{ref}`D <terms_D>` -- {ref}`E <terms_E>` -- {ref}`F <terms_F>` --
{ref}`G <terms_G>` -- {ref}`H <terms_H>` -- {ref}`I <terms_I>` --
{ref}`J <terms_J>` -- {ref}`K <terms_K>` -- {ref}`L <terms_L>` --
{ref}`M <terms_M>` -- {ref}`N <terms_N>` -- {ref}`O <terms_O>` --
{ref}`P <terms_P>` -- {ref}`Q <terms_Q>` -- {ref}`R <terms_R>` --
{ref}`S <terms_S>` -- {ref}`T <terms_T>` -- {ref}`U <terms_U>` --
{ref}`V <terms_V>` -- {ref}`W <terms_W>` -- {ref}`X <terms_X>` --
{ref}`Y <terms_Y>` -- {ref}`Z <terms_Z>`


(terms_A)=
## A

:::{glossary}
  *Work in Progress*

:::

(terms_B)=
## B

:::{glossary}

    *Work in Progress*

:::


(terms_C)=
## C

:::{glossary}

    *Work in Progress*


:::

(terms_D)=
## D

:::{glossary}

## Distro 
  Distro is short for distribution - specifically a Linux distribution.
  A Linux distro is a complete operating system built around the Linux kernel, bundled with different software, tools, and a package manager. Think     of it as a "version" of Linux.
  Popular Linux distros include: 
      * Ubuntu
      * Deblan
      * Fedora

    Related topic(s):
    * Linux distribution
    * Linux kernel

:::


(terms_E)=
## E

:::{glossary}

    *Work in Progress*

:::

(terms_F)=
## F

:::{glossary}

## Front end 
   Refers to everything the user directly interacts with in an application. 
   The front-end includes:
   * The GUI (visual design and layout)
   * User interaction logic {term}`(UIL)`
   * Client-side code (like {term}`HTML`, {term}`CSS`, {term}`JavaScript` for {term}`web apps`)
   * How data is displayed to users


    Related topic(s):
    * GUI
    * GUI front end
    * User interaction logic (UIL)
    * Client-side code
:::



(terms_G)=
## G

:::{glossary}

    *Work in Progress*

    
(ref::up4w-gui)=
## GUI 
Graphical User Interface (GUI)
    It is a visual way for people to interact with computers using graphics like windows, icons, buttons, and menus instead of typing text commands. Before GUIs became common in the 1980s, most people interacted with computers through command-line       interfaces {term}`(CLIs)`. GUIs made computers much more accessible because you could point and click with a mouse rather than memorize commands.
    Common elements of a GUI include:
 * Windows that display content and applications
 * Icons representing files, folders, and programs
 * Buttons you can click to perform actions
 * Menus with dropdown options
 * Dialog boxes for settings and confirmations

## GUI front end (Ubuntu Pro for WSL)
  Grafical User Iinterface front end 
  GUI front end in Pro for WSL refers to a small GUI that helps users provide an Ubuntu Pro token and
[configure Landscape](ref::landscape-config).

  When the GUI starts, it attempts to establish a connection to the [Pro for WSL Windows Agent](ref::up4w-windows-agent). If this fails, the agent is restarted. 
  For troubleshooting purposes, you can restart the agent by:
  * Stopping the Windows process `ubuntu-pro-agent-launcher.exe`in Windows Task Manger
  * Issuing the following command in a PowerShell terminal:

 See also:
 ```text
    * Stop-Process -Name ubuntu-pro-agent.exe
 ```
    Related topic(s):
    
:::


(terms_H)=
## H

:::{glossary}

    *Work in Progress*

:::


(terms_I)=
## I

:::{glossary}

    *Work in Progress*

:::

(terms_J)=
## J

:::{glossary}

    *Work in Progress*

:::

(terms_K)=
## K

:::{glossary}

    *Work in Progress*

:::


(terms_L)=
## L

:::{glossary}


(ref::landscape-client)=
## Landscape client

    The Landscape client is a {term}`systemd` unit running inside every Ubuntu WSL instance.   The Landscape client comes pre-installed in your {term}`distro` as part of the package `landscape-client`. 
    It sends information about the system to the Landscape server. The server, in turn, sends instructions that the client executes.
    Check the status of the Landscape client in any particular Ubuntu WSL instance by:
      * starting a shell in that instance and running:

```text
systemctl status landscape-client.service
```
:::

(terms_M)=
## M

:::{glossary}

    *Work in Progress*

:::

(terms_N)=
## N

:::{glossary}

    *Work in Progress*

:::

(terms_O)=
## O

:::{glossary}

    *Work in Progress*

:::

(terms_P)=
## P

:::{glossary}

    *Work in Progress*

:::

(terms_Q)=
## Q

:::{glossary}

    *Work in Progress*

:::

(terms_R)=
## R

:::{glossary}

    *Work in Progress*

:::


(terms_S)=
## S

:::{glossary}

    *Work in Progress*

:::

(terms_T)=
## T

:::{glossary}

    *Work in Progress*

:::

(terms_U)=
## U

:::{glossary}

(ref::ubuntu-pro-client)=
## Ubuntu Pro client
    Ubuntu Pro client is a command-line utility that manages the different offerings of the Ubuntu Pro subscription. In Pro for WSL, this executable
    is used within each of the managed WSL distros to enable [Ubuntu Pro](https://documentation.ubuntu.com/pro/) services within that distro. 
    This executable comes pre-installed in {term}`Ubuntu WSL` as part of the {term} `ubuntu-pro-client` package since Ubuntu 24.04 LTS.


:::

(terms_V)=
## V

:::{glossary}

   ## Verbose logging (-vvv)

:::

(terms_W)=
## W

:::{glossary}

   (ref::up4w-windows-agent)=
## Windows agent (Pro for WSL)
   The Windows agent is a Windows application running in the background. It starts automatically when the user logs in to Windows. 
   If it stops for any reason, it can be started by launching the Pro for WSL {term}`GUI` or running the executable from the terminal, optionally with `-vvv` for verbose logging:

      ```text
      ubuntu-pro-agent.exe -vvv
      ```
    The Windows agent is Pro for WSL's central hub that communicates with all the components to coordinate them.

    (ref::up4w-wsl-pro-service)=
## WSL Pro service
  Windows Subsystem for Linux (WSL) Pro Service is a bridge between the Windows agent and Ubuntu WSL instances that controls Pro and Landscape status   GitHub. It is a component that's part of Ubuntu Pro for WSL. It is a `systemd` unit running inside every Ubuntu WSL instance. The [Windows agent]     (ref::up4w-windows-agent) running on the Windows host sends commands that the WSL Pro Service executes, such as pro-attaching or configuring the      [Landscape client](ref::landscape-client).
  Check the current status of the WSL Pro Service in any particular {term}`distro` with:

  ```text
  systemctl status wsl-pro.service
  ```


:::

(terms_X)=
## X

:::{glossary}

    *Work in Progress*

:::

(terms_Y)=
## Y

:::{glossary}

    *Work in Progress*

:::

(terms_Z)=
## Z

:::{glossary}

    *Work in Progress*

:::






