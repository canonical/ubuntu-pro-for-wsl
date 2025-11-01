---
myst:
  html_meta:
    "description lang=en":
      "Enable GPU acceleration with NVIDIA CUDA for Ubuntu on WSL, to support AI, ML and other computationally-intensive projects."
---

# Enable GPU acceleration for Ubuntu on WSL with the NVIDIA CUDA Platform

While WSL's default setup allows you to develop cross-platform applications without leaving Windows, enabling GPU acceleration inside WSL provides users with direct access to the hardware. This provides support for GPU-accelerated AI/ML training and the ability to develop and test applications built on top of technologies, such as OpenVINO, OpenGL, and CUDA that target Ubuntu while staying on Windows.

## What you will learn

* How to install a Windows graphical device driver compatible with WSL 2
* How to install the NVIDIA CUDA toolkit for WSL 2 on Ubuntu
* How to compile and run a sample CUDA application on Ubuntu on WSL 2

## What you will need

The following steps assume a specific hardware configuration. Although the concepts are essentially the same for other architectures, different hardware configurations will require the appropriate graphics drivers and CUDA toolkit.

Make sure the following prerequisites are met before moving forward:

* Windows 11 (recommended) or Windows 10 with minimum version 21H2 on a physical machine
* NVIDIA graphics card and administrative permission to install device drivers (see also [NVIDIA's system requirements for CUDA](https://docs.nvidia.com/cuda/cuda-installation-guide-microsoft-windows/))
* Ubuntu 20.04 or higher installed on WSL 2
* Broadband internet connection able to download a few GB of data

```{tip}
For information on how to install Ubuntu on WSL, refer to our [installation guide](howto::install-ubuntu-wsl).

```

## Install the appropriate Windows vGPU driver for WSL


```{note}
Specific drivers are needed to enable use of a virtual GPU, which is how Ubuntu applications are able to access your GPU hardware, so you’ll need to follow this step even if your system drivers are up-to-date.

```

Please refer to the official [WSL documentation](https://learn.microsoft.com/en-us/windows/wsl/tutorials/gui-apps) for up-to-date [links for specific GPU vendors](https://learn.microsoft.com/en-us/windows/wsl/tutorials/gui-apps#prerequisites).

![Install support for Linux GUI apps page on Microsoft WSL documentation.](assets/gpu-cuda/install-drivers.png)


```{note}
This is the only device driver you’ll need to install. Do not install any display driver on Ubuntu.
```

Once downloaded, double-click on the executable file and click `Yes` to allow the program to make changes to your computer.

![Windows file explorer showing the downloaded NVIDIA GPU driver for WSL.](assets/gpu-cuda/downloads-folder.png)

![Windows Package Installer confirmation page for NVIDIA Package Launcher.](assets/gpu-cuda/nvidia-allow-changes.png)

Confirm the default directory and allow the self-extraction process to proceed.

![Default directory confirmation page for NVIDIA Display Driver.](assets/gpu-cuda/default-dir.png)

![NVIDIA Display Driver installation progress screen.](assets/gpu-cuda/please-wait-install.png)

A splash screen appears with the driver version number and quickly turns into the main installer window. Read and accept the license terms to continue.

![NVIDIA Graphics Driver startup page.](assets/gpu-cuda/splash-screen.png)

![NVIDIA software license agreement.](assets/gpu-cuda/license.png)

Confirm the wizard defaults by clicking `Next` and wait until the end of the installation. You might be prompted to restart your computer.

![NVIDIA Graphics Driver installation options with "Express" checked.](assets/gpu-cuda/installation-options.png)

![NVIDIA Virtual Host controller installation progress.](assets/gpu-cuda/installing.png)

This step ends with a screen similar to the image below.

![NVIDIA Graphics Driver installation success page.](assets/gpu-cuda/install-finished.png)

## Install NVIDIA CUDA on Ubuntu


```{note}
Normally, the CUDA toolkit for Linux comes packaged with the device driver for the GPU. On WSL 2, the CUDA driver used is part of the Windows driver installed on the system, and, therefore, care must be taken **not** to install this Linux driver.
```

The following commands will install the WSL-specific CUDA toolkit version 11.6 on Ubuntu 22.04 AMD64 architecture. Be aware that older versions of CUDA (<=10) don’t support WSL 2. Also notice that attempting to install the CUDA toolkit packages straight from the Ubuntu repository (`cuda`, `cuda-11-0`, or `cuda-drivers`) will attempt to install the Linux NVIDIA graphics driver, which is not what you want on WSL 2. So, first remove the old GPG key:

```{code-block} text
$ sudo apt-key del 7fa2af80
```

Then setup the appropriate package for Ubuntu WSL with the following commands:

```{code-block} text
$ wget https://developer.download.nvidia.com/compute/cuda/repos/wsl-ubuntu/x86_64/cuda-wsl-ubuntu.pin

$ sudo mv cuda-wsl-ubuntu.pin /etc/apt/preferences.d/cuda-repository-pin-600

$ sudo apt-key adv --fetch-keys https://developer.download.nvidia.com/compute/cuda/repos/wsl-ubuntu/x86_64/3bf863cc.pub

$ sudo add-apt-repository 'deb https://developer.download.nvidia.com/compute/cuda/repos/wsl-ubuntu/x86_64/ /'

$ sudo apt-get update

$ sudo apt-get -y install cuda
```

Once complete, you should see a series of outputs that end in `done.`:

![Terminal output showing successful installation of NVIDIA CUDA toolkit on Ubuntu.](assets/gpu-cuda/done-done.png)

Congratulations! You should have a working installation of CUDA by now. Let’s test it in the next step.

## Compile a sample application

NVIDIA provides an open source repository on GitHub with samples for CUDA Developers to explore the features available in the CUDA Toolkit. Building one of these is a great way to test your CUDA installation. Let’s choose the simplest one just to validate that our installation works.

Let’s say you have a `~/Dev/` directory where you usually put your working projects. Navigate inside the directory and `git clone` the [cuda-samples repository](https://github.com/nvidia/cuda-samples):

```{code-block} text
$ cd ~/Dev
$ git clone https://github.com/nvidia/cuda-samples
```

To build the application, go to the cloned repository directory and run `make`:

```{code-block} text
$ cd ~/Dev/cuda-samples/Samples/1_Utilities/deviceQuery
$ make
```

A successful build will look like the screenshot below.

![Terminal output showing the successful compilation of a CUDA sample application.](assets/gpu-cuda/make.png)

Once complete, run the application with:

```{code-block} text
$ ./deviceQuery
```

You should see a similar output to the following detailing the functionality of your CUDA setup (the exact results depend on your hardware setup):

![Terminal output showing the results of running the device query sample application.](assets/gpu-cuda/device-query.png)


## AMD support for WSL

While this guide focuses on NVIDIA, WSL is also supported by some AMD GPUs.

If you need to use WSL with an AMD GPU, refer to the official AMD documentation.

The documentation includes a dedicated [guide for using AMD Radeon GPUs with Ubuntu on WSL](https://rocm.docs.amd.com/projects/radeon/en/latest/docs/install/wsl/howto_wsl.html).

## Enjoy Ubuntu on WSL!

That’s all folks! In this tutorial, we’ve shown you how to enable GPU acceleration on Ubuntu on WSL 2 and demonstrated its functionality with the NVIDIA CUDA toolkit, from installation through to compiling and running a sample application.

We hope you enjoy using Ubuntu inside WSL for your Data Science projects. Don’t forget to check out [our blog](https://ubuntu.com/blog) for the latest news on all things Ubuntu.

### Further Reading

* [Setting up WSL for Data Science](https://ubuntu.com/blog/wsl-for-data-scientist)
* [Ubuntu WSL for Data Scientists Whitepaper](https://ubuntu.com/engage/ubuntu-wsl-for-data-scientists)
* [NVIDIA's CUDA Post Installation Actions](https://docs.nvidia.com/cuda/cuda-installation-guide-linux/)
* [Install Ubuntu on WSL 2](../howto/install-ubuntu-wsl2.md)
* [Microsoft WSL Documentation](https://learn.microsoft.com/en-us/windows/wsl/)
* [CUDA on WSL User Guide](https://docs.nvidia.com/cuda/wsl-user-guide/index.html)
* [Ask Ubuntu](https://askubuntu.com/)
