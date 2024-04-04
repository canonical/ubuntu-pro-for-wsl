# How to install and configure UP4W

## 1. Check that you meet the prerequisites

To install and configure UP4W you will need:

- A Windows host
- An Ubuntu Pro token
- Verify that the [firewall rules are correctly set up](../reference/firewall_requirements.md)


<details><summary> How do I get an Ubuntu Pro token? </summary> 

1. Visit the Ubuntu Pro page to get a subscription.
    
> See more: [Ubuntu | Ubuntu Pro > Subscribe](https://ubuntu.com/pro/subscribe). If you choose the personal subscription option (`Myself`), the subscription is free for up to 5 machines. 

2. Visit your Ubuntu Pro Dashboard to retrieve your subscription token.
    
> See more: [Ubuntu | Ubuntu Pro > Dashboard](https://ubuntu.com/pro/dashboard)

</details>



- (*Only if you want to use UP4W with Landscape:*) <br>A UP4W-compatible Landscape server
    - i.e., Landscape beta



<details><summary> How do I set up a  Landscape beta server?</summary> 

> See: [Landscape | Quickstart deployment](https://ubuntu.com/landscape/docs/quickstart-deployment)

</details>

<details><summary>Can you help me set up a Landscape beta server locally, just for testing?</summary>

Sure: 

1. Set up an Ubuntu WSL to act as the server:

   1. Install a new Ubuntu WSL distro:
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
   3. Set up a Landscape Beta server: 
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
      	- This ensures this distro keeps running in the background. See also: [Microsoft's FAQ](https://learn.microsoft.com/en-us/windows/wsl/faq#can-i-use-wsl-for-production-scenarios--).
2. Store the following file somewhere in your Windows system. Name it `landscape-client.conf`. Replace the variables in the file with the relevant values for your server.
	```ini
	[host]
	url = ${HOSTAGENT_API_ENDPOINT}

	[client]
	url = ${MESSAGE_API_ENDPOINT}
	ping_url = ${PING_API_ENDPOINT}
	account_name = standalone
	```
</details>


- (*Only for the verify step:*) <br> One or more UP4W-compatible Ubuntu WSL instances
    - i.e., from an `Ubuntu`, `Ubuntu-Preview`, or `Ubuntu-22.04`+ distro and with `wsl-pro-service` installed 
	    - note: with a freshly installed or updated `Ubuntu-Preview` or `Ubuntu-22.04`+, `wsl-pro-service` comes automatically pre-installed

<details><summary> I already have an Ubuntu WSL instance. Can I make it UP4W-compatible? </summary>

It depends:

1.	Open a shell into your instance and check its distro version:
    ```bash
	cat /etc/os-release
	```
	If the distro is *not* `Ubuntu`, `Ubuntu-Preview`, or `Ubuntu-22.04`+: Your instance cannot be made UP4W-compatible. Please create a new one that is compatible. Otherwise:

2.	Open a shell into your distro and  check if package `wsl-pro-service` is installed:
	```bash
	pkg -s wsl-pro-service | grep Status
	```

	If the status is *not* `Status: install ok installed`: Install it by running: `sudo apt update && sudo apt install -y wsl-pro-service`.

</details>


<details><summary> How do I create a new Ubuntu WSL instance that is UP4W-compatible? </summary>

```{note}
Here we assume you already have WSL installed. Run `wsl --version` to verify; if there you get an error because it is not there, install it: `wsl --install`.
```

```{warning}
Here we assume you do not have any existing `Ubuntu-Preview` or `Ubuntu-22.04`+ instances or you have them but do not mind overwriting them. 
- To view your current registered instances, run `wsl --list --quiet`. 
- To export, delete, and re-import an instance, see `wsl --export`, `wsl --unregister`, and `wsl --import`. 
> See more: [Microsoft | WSL Documentation](https://learn.microsoft.com/en-us/windows/wsl/basic-commands).
```


On your Windows host, in the Microsoft Store, search for the `Ubuntu-Preview` or `Ubuntu-22.04`+ distro app, click **Install** / **Update**, then **Open**. Follow the instructions to set up the instance. Note: The instance will come with `wsl-pro-service` pre-installed.

</details>



<!-- ## Prerequisites -->
<!-- ### Prepare a compatible Ubuntu WSL distro -->

<!-- <details><summary> Expand to see how to make a pre-existing WSL distro UP4W-compatible </summary> -->

<!-- > Note: You can make more than one distro compatible, and UP4W will manage all of them. -->
<!-- 1.	Check the version of your distro with `cat /etc/os-release`. -->
<!-- 	- If the `NAME` is not `Ubuntu`, the distro cannot be made compatible. You'll need to create a new one. -->
<!-- 	- If the `VERSION_ID` is not `24.04`, the distro cannot be made compatible. You'll need to create a new one. -->
<!-- 	<\!-- Once Noble is released, there will also be the option to upgrade the distro -\-> -->

<!-- 2.	Check if package `wsl-pro-service` is installed by running this command in your distro: -->
<!-- 	```bash -->
<!-- 	pkg -s wsl-pro-service | grep Status -->
<!-- 	``` -->

<!-- 	- If the output says `Status: install ok installed`: Congratulations, your WSL instance is already compatible with UP4W. -->
<!-- 	- Otherwise: Install it by running: `sudo apt update && sudo apt install -y wsl-pro-service` -->
<!-- </details> -->

<!-- <details><summary> Expand to see how to create a new UP4W-compatible distro </summary> -->

<!-- 1.	Verify that you have WSL installed: In your Windows terminal, run `wsl --version` and see that there is no error. Otherwise, install it with `wsl --install`. -->
<!-- 2.	Ensure that you don't have an _Ubuntu (Preview)_ distro registered by running `wsl --list --quiet` on your Windows terminal. -->
<!-- 	- If the output contains _Ubuntu-Preview_, you already have an instance of Ubuntu (Preview). -->
<!-- 		- You can make it compatible with UP4W -->
<!--  		- You can remove it and continue installing a new instance. -->
<!-- 			- To **irreversibly** remove the distro, run: `wsl --unregister Ubuntu-Preview`. -->
<!-- 3.	Ensure that you have the latest _Ubuntu (Preview)_ app installed: -->
<!-- On your Windows host, go to the Microsoft Store, search for _Ubuntu (Preview)_, click on the result and look at the options: -->
<!-- 	- If you see a button `Install`, click it. -->
<!-- 	- If you see a button `Update`, click it. -->

<!-- 	On the same Microsoft Store page, there should be an `Open` button. Click it. _Ubuntu (Preview)_ will start and guide you through the installation steps. -->
<!-- </details> -->


<!-- ### Obtain an Ubuntu Pro token -->

<!-- <details><summary> Expand to see how </summary> -->

<!-- Get the Ubuntu Pro token associated with your subscription (it's free for up to 5 machines). -->
<!-- > See more: [Ubuntu Pro dashboard](https://ubuntu.com/pro) -->

<!-- </details> -->

<!-- ### Set up a Landscape server -->

<!-- <details><summary> Expand to see how </summary> -->

<!-- 1. Set up an Ubuntu WSL to act as the server: -->
<!-- 	> Note: you can skip step 1 if you already have a Landscape Beta server running. -->

<!-- 	> Note: The usual setup calls for the Landscape server to run on another machine (a server). For demonstration purposes, we explain how to set up a Landscape server in one of your WSL distros. -->

<!--    1. Install a new Ubuntu WSL distro -->
<!-- 	```shell -->
<!-- 	wsl --install Ubuntu-22.04 -->
<!--    	``` -->
<!--    2. Find out the Windows host IP: In the WSL distro named _Ubuntu-22.04_, run: -->
<!--       ```bash -->
<!-- 	  wslinfo --networking-mode -->
<!-- 	  ``` -->
<!--         - If it says `mirrored`, the relevant IP is `127.0.0.1`. Take note of this address. -->
<!--         - Otherwise, open file `/etc/resolv.conf` in the WSL instance named _Ubuntu-22.04_. Find the line starting with `nameserver` followed by an IP address. -->
<!--            - If the IP address does not start with `127`, take note of this address. -->
<!--            - Otherwise, run the command `ip route | grep ^default` and take note of the IP address that is printed. -->
<!--    3. Set up a Landscape Beta server.  -->
<!--       1. Start a shell in your _Ubuntu-22.04_ distro. -->
<!--       2. Install the Landscape (beta) following the steps in the Landscape Quickstart deployment with the following considerations: -->
<!--          - Make sure you install the beta version. -->
<!--          - Your FQDN is the address you took note of in the previous step. -->
<!--    		> See more: [Landscape | Quickstart deployment](https://ubuntu.com/landscape/docs/quickstart-deployment) -->
<!--    4. Take note of the following addresses: -->
<!--       	- Hostagent API endpoint: `${WINDOWS_HOST_IP}:6554` -->
<!--       	- Message API endpoint: `${WINDOWS_HOST_IP}/message-system` -->
<!--       	- Ping API endpoint: `${WINDOWS_HOST_IP}/ping` -->
<!--    5. Open a `Ubuntu-22.04` terminal and keep it open during the rest of the guide. -->
<!--       	- This ensures this distro keeps running in the background. See also: [Microsoft's FAQ](https://learn.microsoft.com/en-us/windows/wsl/faq#can-i-use-wsl-for-production-scenarios--). -->
<!-- 2. Store the following file somewhere in your Windows system. Name it `landscape-client.conf`. Replace the variables in the file with the relevant values for your server. -->
<!-- 	```ini -->
<!-- 	[host] -->
<!-- 	url = ${HOSTAGENT_API_ENDPOINT} -->

<!-- 	[client] -->
<!-- 	url = ${MESSAGE_API_ENDPOINT} -->
<!-- 	ping_url = ${PING_API_ENDPOINT} -->
<!-- 	account_name = standalone -->
<!-- 	``` -->
<!-- </details> -->

## 2. Install UP4W
On your Windows host, go to the Microsoft Store, search for _Ubuntu Pro for WSL_ and click on the result. Find the _Install_ button. Click. Done.

(howto::configure-up4w)=

## 3. Configure UP4W for Ubuntu Pro and Landscape

> See also: [Ubuntu Pro](ref::ubuntu-pro), [Landscape](ref::landscape)

There are two ways in which you can configure UP4W for Ubuntu Pro and Landscape -- using the UP4W GUI or using the Windows Registry.

### Using the GUI

```{note}
With this method you can only configure UP4W on a single Windows host at a time.
```

> See also: [UP4W GUI](ref::up4w-gui)
1. Open the Windows menu, search for "Ubuntu Pro for WSL", click.
2. Input your Ubuntu Pro token:
	1. Click on **Already have a token?**.
	2. Write your Ubuntu Pro token and click **Confirm**.
3. Input your Landscape configuration:
	1. Click on **Quick setup**.
	2. Write the **FQDN** of your server.
	3. Leave the **registration key** field empty.
	4. Click the **Continue** button.

(howto::configure::registry)=
### Using the registry

```{note}
This method can be adapted to configure UP4W on multiple Windows hosts at a time.
```


> See also: [Windows registry](windows-registry)
1. Press Win+R, type `regedit.exe`, and click OK.
2. Navigate the tree to `HKEY_CURRENT_USER\Software\Canonical\UbuntuPro`.
	```{note}
	This key will not exist until you've run UP4W at least once. Otherwise, you'll have to create the key and values yourself. See more: [Microsoft Learn | Windows registry information for advanced users](https://learn.microsoft.com/en-us/troubleshoot/windows-server/performance/windows-registry-advanced-users)
	```
6. Input your Ubuntu Pro token:
	- Right-click `UbuntuProToken` > Modify > Write the Ubuntu Pro token.
7. Input your Landscape configuration:
	- Right-click `LandscapeConfig` > Modify > Write the Landscape config.
	  > See more: [UP4W Landscape config reference](ref::landscape-config).

## 4. Verify that UP4W is working
> If either verification step fails, wait for a few seconds and try again. This should not take longer than a minute.
1. Start the UP4W GUI and check that your subscription is active.
   - To open the GUI, search _Ubuntu Pro for WSL_ in the Windows menu and click on it.
   - The GUI will explicitly say that you are subscribed.
2. Open any of the distros you want to manage and check that it is pro-attached with `pro status`.
	> See also: [Ubuntu Pro client](ref::ubuntu-pro-client)
1. Open Landscape and check that the host and distro were registered.
	> See more: [Landscape | View WSL host machines and child computers](https://ubuntu.com/landscape/docs/perform-common-tasks-with-wsl-in-landscape/#heading--view-wsl-host-machines-and-child-computers)
