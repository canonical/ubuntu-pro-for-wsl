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

Client-Side Code
  Client-side code is code that runs on the user's device (in their {term}`web browser`, {term}`desktop app`, or {term}`mobile app`) rather than on a remote server.


Client-side technologies
  Client-side technologies refer to web technologies that run on the user's device, primarily executed within the browser, such as HTML, CSS, and JavaScript, enabling dynamic user interfaces and interactive web applications. These technologies are essential for enhancing user experience by reducing server load and allowing asynchronous interactions without continuous full-page reloads. As the functionality and efficiency of web browsers continue to grow, understanding client-side technologies becomes crucial for modern web development, as they empower developers to create fast, responsive, and engaging user experiences.


CLI
Command-line interface
  CLI is a way to interact with a computer by typing text commands instead of clicking buttons and icons.

  See also:
    * [command-line interfaces](https://en.wikipedia.org/wiki/Command-line_interface)


:::

(terms_D)=
## D

:::{glossary}

Debian
  Debian is a free, open-source operating system. The "parent" of Ubuntu and many other distributions. Debian is known for extreme stability and reliability. It is Run entirely by volunteers (no company behind it).
    
    See also:
    * [Debian the Community](https://www.debian.org/)

Distro 
    Distro is short for distribution - specifically a Linux distribution. A Linux distro is a complete operating system built around the Linux kernel, bundled with different software, tools, and a package manager. Popular Linux distros include: 
          * {term}`Ubuntu`
          * {term}`Debian`
          * {term}`Fedora`

     See also:
     * [Linux kernel](https://en.wikipedia.org/wiki/Linux_kernel)

:::


(terms_E)=
## E

:::{glossary}

    *Work in Progress*

:::

(terms_F)=
## F

:::{glossary}

Fedora
  Fedora is a free, open-source Linux distribution that focuses on innovation and cutting-edge technology. It is sponsored by Red Hat (now owned by IBM) and is known for being at the forefront of new Linux features.


    See also:
    * [Fedora_Linux](https://en.wikipedia.org/wiki/Fedora_Linux)(https://www.youtube.com/watch?v=aYpnpjQDX64)

  

Front end 
    Front end refers to everything the user directly interacts with in an application. The front-end includes:
           * GUI (visual design and layout)
           * User interaction logic (UIL)
           * Client-side code (like {term}`HTML`, {term}`CSS`, {term}`JavaScript` for {term}`web apps`)
           * How data is displayed to users

    Related topic(s):
    * {term}`GUI`
    * {term}`GUI front end`
    * User interaction logic ({term}`UIL`)
    * {term}`Client-side code`
:::



(terms_G)=
## G

:::{glossary}

GUI 
Graphical User Interface (GUI)
    GUI is a visual way for people to interact with computers using graphics like windows, icons, buttons, and menus instead of typing text commands. Common elements of a GUI include windows, icons, buttons, menus, and dialog boxes. 

    Related topic(s):
    *command-line interface({term}`CLI`)     

GUI front end 
Grafical User Iinterface front end 
    GUI front end in Pro for WSL refers to a small GUI that helps users provide an Ubuntu Pro token and [configure Landscape](ref::landscape-config). When the GUI starts, it attempts to establish a connection to the [Pro for WSL Windows Agent](ref::up4w windows-agent). If this fails, the agent is restarted. Stop the process by clicking `ubuntu-pro-agent-launcher.exe` "end task" in Windows Task Manger. In a PowerShell terminal type: `Stop-Process -Name ubuntu-pro-agent.exe`
    
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

Landscape client
    Landscape client is a {term}`systemd` unit running inside every Ubuntu WSL instance. The Landscape client comes pre-installed in your {term}`distro` as part of the package `landscape-client`. It sends information about the system to the Landscape server. The server, in turn, sends instructions that the client executes. Check the status of the Landscape client in any particular Ubuntu WSL instance by starting a shell in that instance and running systemctl status landscape-client.service

Landscape configuration
    It has different meanings depending on the context. In IT "landscape" refers to the overall system architecture.

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

Ubuntu
  Ubuntu is a free, open-source operating system based on Linux. It's one of the most popular Linux distributions (distros) in the world, known for being user-friendly and beginner-friendly.

Ubuntu Pro client
    Ubuntu Pro client is a command-line utility that manages the different offerings of the Ubuntu Pro subscription. In Pro for WSL, this executable is used within each of the managed WSL distros to enable [Ubuntu Pro](https:/documentation.ubuntu.com/pro/) services within that distro and it comes pre-installed in {term}`Ubuntu WSL` as part of the {term} `ubuntu-pro-client` package since Ubuntu 24.04 LTS.

UIL
User interaction logic
  UIL is the code and rules that handle how users interact with software.
  
:::

(terms_V)=
## V

:::{glossary}

Verbose logging (-vvv)

:::

(terms_W)=
## W

:::{glossary}


Windows agent 
    Windows agent (Pro for WSL) is a Windows application running in the background. It starts automatically when the user logs in to Windows. If it stops for any reason, it can be started by launching the Pro for WSL {term}`GUI` or running the executable from the terminal, optionally with `-vvv` for verbose logging {term}`ubuntu-pro-agent.exe -vvv`. Windows agent is Pro for WSL's central hub that communicates with all the components to coordinate them.
    

WSL Pro service
Windows Subsystem for Linux (WSL) Pro Service 
    WSL Pro service is a bridge between the Windows agent and Ubuntu WSL instances that controls Pro and Landscape status GitHub. It is a component that's part of Ubuntu Pro for WSL. It is a `systemd` unit running inside every Ubuntu WSL instance. The [Windows agent](ref::up4w-windows-agent) running on the Windows host sends commands that the WSL Pro Service executes, such as pro-attaching or configuring the [Landscape client](ref::landscape-client). Check the current status of the WSL Pro Service in any particular {term}`distro` with: systemctl status wsl-pro.service 

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






