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
* Ubuntu 24.04 or higher installed on WSL 2 (different versions of CUDA will work on older versions of Ubuntu, here we'll use the latest of both)
* Broadband internet connection able to download a few GB of data

```{tip}
For information on how to install Ubuntu on WSL, refer to our [installation guide](howto::install-ubuntu-wsl).

```

## Install the appropriate Windows vGPU driver for WSL


```{note}
Specific drivers are needed to enable use of a virtual GPU, which is how Ubuntu applications are able to access your GPU hardware, so you’ll need to follow this step even if your system drivers are up-to-date.

```

Please refer to the official [WSL documentation](https://learn.microsoft.com/en-us/windows/wsl/tutorials/gui-apps) for up-to-date [links for specific GPU vendors](https://learn.microsoft.com/en-us/windows/wsl/tutorials/gui-apps#prerequisites).


```{note}
This is the only device driver you’ll need to install. Do not install any display driver on Ubuntu.
```

Once downloaded, double-click on the executable file and click "Yes" to allow the program to make
changes to your computer. Follow the installation wizard and select "Express Installation" when
prompted, using the default options. Make sure you can accept the license agreement before
completing this step. You might be prompted to restart your computer.

## Install NVIDIA CUDA on Ubuntu


```{note}
Normally, the CUDA toolkit for Linux comes packaged with the device driver for the GPU. On WSL 2, the CUDA driver used is part of the Windows driver installed on the system, and, therefore, care must be taken **not** to install this Linux driver.
```

The following commands will install the WSL-specific CUDA toolkit version 13.2 on Ubuntu 24.04 AMD64 architecture. Be aware that older versions of CUDA (<=10) don’t support WSL 2. Also notice that attempting to install the CUDA toolkit packages straight from the Ubuntu repository (`cuda`, `cuda-13`, or `cuda-drivers`) will attempt to install the Linux NVIDIA graphics driver, which is not what you want on WSL 2.

Navigate to the [CUDA Downloads page](https://developer.nvidia.com/cuda-downloads) and select the appropriate options for your system. In this case, we will select:

* Operating System: Linux
* Architecture: x86_64
* Distribution: WSL-Ubuntu
* Version: 2.0
* Installer Type: deb (network)

The website renders a series of commands matching the options you selected. The last line installs
a specific version of the CUDA toolkit compatible with WSL 2 distro instances. Make sure to adjust
the version to suit your specific needs.

For our example, the output will be the following:

```{code-block} text
$ wget https://developer.download.nvidia.com/compute/cuda/repos/wsl-ubuntu/x86_64/cuda-keyring_1.1-1_all.deb

$ sudo dpkg -i cuda-keyring_1.1-1_all.deb

$ sudo apt-get update

$ sudo apt-get -y install cuda-toolkit-13-2
```

Copy the commands shown in the website and paste them on a terminal to execute the installation.
A successful installation will show a lot of text, ending in a pattern similar to the following
(the exact details may depend on your system and the CUDA version being installed):

```{code-block} text
done.
Setting up default-jre-headless (2:1.21-75+exp1) ...
Setting up openjdk-21-jre:amd64 (21.0.10+7-1~24.04) ...
Setting up default-jre (2:1.21-75+exp1) ...
Setting up cuda-nsight-13-2 (13.2.20-1) ...
Setting up cuda-visual-tools-13-2 (13.2.0-1) ...
Setting up cuda-tools-13-2 (13.2.0-1) ...
Setting up cuda-toolkit-13-2 (13.2.0-1) ...
g@mib01:~$
```

Congratulations! You should have a working installation of CUDA by now. Let’s test it in the next step.

## Compile a sample application

NVIDIA provides an open source repository on GitHub with samples for CUDA Developers to explore the features available in the CUDA Toolkit. Building one of these is a great way to test your CUDA installation. Let’s choose the simplest one just to validate that our installation works.

Let’s say you have a `~/Dev/` directory where you usually put your working projects. Navigate inside the directory and `git clone` the [cuda-samples repository](https://github.com/nvidia/cuda-samples):

```{code-block} text
$ cd ~/Dev

$ git clone https://github.com/nvidia/cuda-samples
```

To build the application, you'll need `cmake`. Install it and then go to the cloned repository directory and run it:

```{code-block} text
$ sudo apt install cmake

$ cd ~/Dev/cuda-samples/Samples/1_Utilities/deviceQuery

$ cmake -S . -B build

$ cmake --build build
```

A successful build will look like the following:

```{code-block} text
g@mib01:~/Dev/cuda-samples/Samples/1_Utilities/deviceQuery$ cmake -S . -B build
-- The C compiler identification is GNU 13.3.0
-- The CXX compiler identification is GNU 13.3.0
-- The CUDA compiler identification is NVIDIA 13.2.51 with host compiler GNU 13.3.0
-- Detecting C compiler ABI info
-- Detecting C compiler ABI info - done
-- Check for working C compiler: /usr/bin/cc - skipped
-- Detecting C compile features
-- Detecting C compile features - done
-- Detecting CXX compiler ABI info
-- Detecting CXX compiler ABI info - done
-- Check for working CXX compiler: /usr/bin/c++ - skipped
-- Detecting CXX compile features
-- Detecting CXX compile features - done
-- Detecting CUDA compiler ABI info
-- Detecting CUDA compiler ABI info - done
-- Check for working CUDA compiler: /usr/local/cuda-13/bin/nvcc - skipped
-- Detecting CUDA compile features
-- Detecting CUDA compile features - done
-- Found CUDAToolkit: /usr/local/cuda-13/targets/x86_64-linux/include (found version "13.2.51")
-- Performing Test CMAKE_HAVE_LIBC_PTHREAD
-- Performing Test CMAKE_HAVE_LIBC_PTHREAD - Success
-- Found Threads: TRUE
-- CUDA Samples installation configured:
--   Architecture: x86_64
--   OS: linux
--   Build Type: release
--   Install Prefix: /home/g/Dev/cuda-samples/Samples/1_Utilities/deviceQuery/build/bin
--   Install Directory: /home/g/Dev/cuda-samples/Samples/1_Utilities/deviceQuery/build/bin/x86_64/linux/release
-- Configuring done (43.2s)
-- Generating done (0.0s)
-- Build files have been written to: /home/g/Dev/cuda-samples/Samples/1_Utilities/deviceQuery/build
g@mib01:~/Dev/cuda-samples/Samples/1_Utilities/deviceQuery$ cmake --build build
[ 50%] Building CXX object CMakeFiles/deviceQuery.dir/deviceQuery.cpp.o
[100%] Linking CXX executable deviceQuery
[100%] Built target deviceQuery
g@mib01:~/Dev/cuda-samples/Samples/1_Utilities/deviceQuery$
```


Once complete, run the application with:

```{code-block} text
$ ./build/deviceQuery
```

You should see a similar output to the following detailing the functionality of your CUDA setup (the exact results depend on your hardware setup):

```text
./build/deviceQuery Starting...

 CUDA Device Query (Runtime API) version (CUDART static linking)

Detected 1 CUDA Capable device(s)

Device 0: "NVIDIA GeForce MX130"
  CUDA Driver Version / Runtime Version          13.0 / 13.2
  CUDA Capability Major/Minor version number:    5.0
  Total amount of global memory:                 2048 MBytes (2147352576 bytes)
  (003) Multiprocessors, (128) CUDA Cores/MP:    384 CUDA Cores
  GPU Max Clock rate:                            1189 MHz (1.19 GHz)
  Memory Clock rate:                             2505 Mhz
  Memory Bus Width:                              64-bit
  L2 Cache Size:                                 1048576 bytes
  Maximum Texture Dimension Size (x,y,z)         1D=(65536), 2D=(65536, 65536), 3D=(4096, 4096, 4096)
  Maximum Layered 1D Texture Size, (num) layers  1D=(16384), 2048 layers
  Maximum Layered 2D Texture Size, (num) layers  2D=(16384, 16384), 2048 layers
  Total amount of constant memory:               65536 bytes
  Total amount of shared memory per block:       49152 bytes
  Total shared memory per multiprocessor:        65536 bytes
  Total number of registers available per block: 65536
  Warp size:                                     32
  Maximum number of threads per multiprocessor:  2048
  Maximum number of threads per block:           1024
  Max dimension size of a thread block (x,y,z): (1024, 1024, 64)
  Max dimension size of a grid size    (x,y,z): (2147483647, 65535, 65535)
  Maximum memory pitch:                          2147483647 bytes
  Texture alignment:                             512 bytes
  Concurrent copy and kernel execution:          Yes with 4 copy engine(s)
  Run time limit on kernels:                     Yes
  Integrated GPU sharing Host Memory:            No
  Support host page-locked memory mapping:       Yes
  Alignment requirement for Surfaces:            Yes
  Device has ECC support:                        Disabled
  Device supports Unified Addressing (UVA):      Yes
  Device supports Managed Memory:                Yes
  Device supports Compute Preemption:            No
  Supports Cooperative Kernel Launch:            No
  Device PCI Domain ID / Bus ID / location ID:   0 / 1 / 0
  Compute Mode:
     < Default (multiple host threads can use ::cudaSetDevice() with device simultaneously) >

deviceQuery, CUDA Driver = CUDART, CUDA Driver Version = 13.0, CUDA Runtime Version = 13.2, NumDevs = 1
Result = PASS
``` 


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
