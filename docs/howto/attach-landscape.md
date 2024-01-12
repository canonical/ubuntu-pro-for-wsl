# How to register your machine to Landscape via the Windows registry

> **ⓘ Note:** This can also be performed by automated tools, so long as they can write into the Windows Registry.

This how-to guide will teach you how to connect your WSL instances to a Landscape server. See [the reference](../reference/landscape) to learn more about Landscape and Ubuntu Pro For Windows.

Note that for this to work, you need an Ubuntu Pro subscription. Read more about [how to enable Ubuntu Pro for Windows](../dev/howto/04-enable-pro.md).

1. Open the Registry Editor on your Windows machine
   - To open the Registry Editor, press the Windows key + R and type `regedit`.
2. Go to `HKEY_CURRENT_USER\Software`
3. If it does not exist, create a new key and name it `Canonical`.
   - You can do this with Edit or Right Click > New > Key.
5. If it does not exist, create a new key inside the `Canonical` key and name it `UbuntuPro`.
6. Add a new multi-string value within the `UbuntuPro` key.
   - You can do this with Right Click > New > Multi-string value
8. Name this value `LandscapeConfig`.
9. Open the `LandscapeConfig` value and write the contents (not the path!) of your configuration file. A basic configuration file would look like:
   ```ini
   [host]
   url = {HOST_URL}

   [client]
   account_name = {ACCOUNT_NAME}
   registration_key = {REGISTRATION_KEY}
   url = {CLIENT_URL}
   ping_url = {PING_URL}
   ```
   Replace the text in between {braces} with your configuration.
   Read more about the contents of this configuration file in [the reference](landscape-config).
10. Your agent and WSL distros will register to Landscape within the next few minutes. You should see a notification pop up in your Landscape dashboard.

## Read more
- [How to enable Ubuntu Pro for Windows](../dev/howto/04-enable-pro)
- [Ubuntu Pro for Windows Landscape reference](../reference/landscape)

## External links
- [Landscape documentation](https://ubuntu.com/landscape/docs)
- [Get Ubuntu Pro](https://ubuntu.com/pro)