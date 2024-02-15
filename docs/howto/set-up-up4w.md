# How to set up Ubuntu Pro For WSL

## Prerequisites
### Prepare a compatible Ubuntu WSL distro

<details><summary> Expand to see how to make a pre-existing WSL distro UP4W-compatible </summary>

> Note: You can make more than one distro compatible, and UP4W will manage all of them.
1.	Check the version of your distro with `cat /etc/os-release`.
	- If the `NAME` is not `Ubuntu`, the distro cannot be made compatible. You'll need to create a new one.
	- If the `VERSION_ID` is not `24.04`, the distro cannot be made compatible. You'll need to create a new one.
	<!-- Once Noble is released, there will also be the option to upgrade the distro -->

2.	Check if package `wsl-pro-service` is installed by running this command in your distro:
	```bash
	pkg -s wsl-pro-service | grep Status
	```

	- If the output says `Status: install ok installed`: Congratulations, your WSL instance is already compatible with UP4W.
	- Otherwise: Install it by running: `sudo apt update && sudo apt install -y wsl-pro-service`
</details>

<details><summary> Expand to see how to create a new UP4W-compatible distro </summary>

1.	Verify that you have WSL installed: In your Windows terminal, run `wsl --version` and see that there is no error. Otherwise, install it with `wsl --install`.
2.	Ensure that you don't have an _Ubuntu (Preview)_ distro registered by running `wsl --list --quiet` on your Windows terminal.
	- If the output contains _Ubuntu-Preview_, you already have an instance of Ubuntu (Preview).
		- You can make it compatible with UP4W
 		- You can remove it and continue installing a new instance.
			- To **irreversibly** remove the distro, run: `wsl --unregister Ubuntu-Preview`.
3.	Ensure that you have the latest _Ubuntu (Preview)_ app installed:
On your Windows host, go to the Microsoft Store, search for _Ubuntu (Preview)_, click on the result and look at the options:
	- If you see a button `Install`, click it.
	- If you see a button `Update`, click it.

	On the same Microsoft Store page, there should be an `Open` button. Click it. _Ubuntu (Preview)_ will start and guide you through the installation steps.
</details>


### Obtain an Ubuntu Pro token

<details><summary> Expand to see how </summary>

Get the Ubuntu Pro token associated with your subscription (it's free for up to 5 machines).
> See more: [Ubuntu Pro dashboard](https://ubuntu.com/pro)

</details>

### Set up a Landscape server

<details><summary> Expand to see how </summary>

1. Set up an Ubuntu WSL to act as the server:
	> Note: you can skip step 1 if you already have a Landscape Beta server running.

	> Note: The usual setup calls for the Landscape server to run on another machine (a server). For demonstration purposes, we explain how to set up a Landscape server in one of your WSL distros.

   1. Install a new Ubuntu WSL distro
	```shell
	wsl --install Ubuntu-22.04
   	```
   2. Find out the Windows host IP: In the WSL distro named _Ubuntu-22.04_, run:
      ```bash
	  wslinfo --networking-mode
	  ```
        - If it says `mirrored`, the relevant IP is `127.0.0.1`. Take note of this address.
        - Otherwise, open file `/etc/resolv.conf` in the WSL instance named _Ubuntu-22.04_. Find the line starting with `nameserver` followed by an IP address.
           - If the IP address does not start with `127`, take note of this address.
           - Otherwise, run the command `ip route | grep ^default` and take note of the IP address that is printed.
   3. Set up a Landscape Beta server. 
      1. Start a shell in your _Ubuntu-22.04_ distro.
      2. Install the Landscape (beta) following the steps in the Landscape Quickstart deployment with the following considerations:
         - Make sure you install the beta version.
         - Your FQDN is the address you took note of in the previous step.
   		> See more: [Landscape | Quickstart deployment](https://ubuntu.com/landscape/docs/quickstart-deployment)
   4. Take note of the following addresses:
      	- Hostagent API endpoint: `${WINDOWS_HOST_IP}:6554`
      	- Message API endpoint: `${WINDOWS_HOST_IP}/message-system`
      	- Ping API endpoint: `${WINDOWS_HOST_IP}/ping`
   5. Open a `Ubuntu-22.04` terminal and keep it open during the rest of the guide.
      	- This ensures this distro keeps running in the background. See more: [Microsoft's FAQ](https://learn.microsoft.com/en-us/windows/wsl/faq#can-i-use-wsl-for-production-scenarios--).
2. Store the following file somewhere in your Windows system. Name it `landscape-client.conf`. Replace the variables in the file with the relevant values for your server.
	```ini
	[host]
	url = ${HOSTAGENT_API_ENDPOINT}

	[client]
	url = ${MESSAGE_API_ENDPOINT}
	ping_url = ${PING_API_ENDPOINT}
	account_name = standalone
	```
	> See more: [UP4W Landscape config reference](ref::landscape-config).

</details>

## 1. Install Ubuntu Pro for WSL
On your Windows host, go to the Microsoft Store, search for _Ubuntu Pro for WSL_ and click on the result. Find the _Install_ button. Click on it.

## 2. Configure Ubuntu Pro for WSL
You have two ways of setting up UP4W. You can use the graphical interface (GUI), which is recommended for users managing a single Windows machine. Alternatively, you can use the registry which we recommend for users who want to deploy at scale.

### Using the GUI
> See more: [Ubuntu Pro for WSL GUI](ref::up4w-gui)
1. Open the Windows menu, search and click on Ubuntu Pro for WSL.
2. Input your Ubuntu Pro Token:
	1. Click on **Already have a token?**.
	2. Write your Ubuntu Pro token and click **Confirm**.
3. Input your Landscape configuration:
	1. Click on [TO BE ADDED] <!--TODO: Landscape data input GUI is not implemented yet-->
	2. Write the path to file `landscape-client.conf` specified during the Landscape server setup.

### Using the registry
> See more: [Windows registry](windows-registry)
1. Press Win+R, type `regedit.exe`, and click OK.
2. Navigate the tree to `HKEY_CURRENT_USER\Software`.
3. Under this key, search for key `Canonical`. If it does not exist, create it:
	- Right-click `Software` > New > Key > Write `Canonical`.
4. Under this key, search for key `UbuntuPro`. If it does not exist, create it:
	- Right-click `Canonical` > New > Key > Write `UbuntuPro`.
5. Click on the `UbuntuPro` key. Its full path should be `HKEY_CURRENT_USER\Software\Canonical\UbuntuPro`.
6. Input your Ubuntu Pro token:
	1. Create a new string value with the title `UbuntuProToken`:
		- Right-click the `UbuntuPro` key > New > String value > Write `UbuntuProToken`.
	2. Set its value to your Ubuntu Pro token:
		- Right-click `UbuntuProToken` > Modify > Write the Ubuntu Pro token.
7. Input your Landscape configuration:
	1. Create a new multi-string value with the title `LandscapeConfig`:
		- Right-click the `UbuntuPro` key > New > Multi-string value > Write `LandscapeConfig`.
	2. Set its value to the contents of file `landscape-client.conf` specified during the Landscape server setup:
		- Right-click `LandscapeConfig` > Modify > Write the contents of the specified file.

## 3. Verify that UP4W is working
> If either verification step fails, wait for a few seconds and try again. This should not take longer than a minute.
1. Start the UP4W GUI and check that your subscription is active.
   - To open the GUI, search _Ubuntu Pro for WSL_ in the Windows menu and click on it.
   - The GUI will explicitly say that you are subscribed.
2. Open any of the distros you want to manage and check that it is pro-attached with `pro status`.
	> See more: [Ubuntu Pro client](ref::ubuntu-pro-client)
1. Open Landscape and check that the host and distro were registered.
	> See more: [Landscape | View WSL host machines and child computers](https://ubuntu.com/landscape/docs/perform-common-tasks-with-wsl-in-landscape/#heading--view-wsl-host-machines-and-child-computers)
