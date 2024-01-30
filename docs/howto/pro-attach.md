# How to attach Ubuntu Pro for WSL to your subscription

Ubuntu Pro for WSL requires a subscription to [Ubuntu Pro](../reference/ubuntu_pro). Get your subscription visiting the [Ubuntu Pro website](https://www.ubuntu.com/pro). Once you have a subscription, you have to attach Ubuntu Pro for WSL to it.

There are two different methods:

- Attaching via the [GUI](../reference/ubuntu_pro_for_wsl_gui.md) is the more user-friendly option.
- Attaching via the Windows registry is recommended for system administrators as it is easier to automate.

If you try to do both at once, the subscription used in the registry will overrule the one set from the GUI.

## Pro-attach via the Ubuntu Pro for WSL GUI

1. Go to your [Ubuntu Pro dashboard](https://ubuntu.com/pro) to get your [Ubuntu Pro token](../reference/ubuntu_pro_token.md).
2. Go to the Windows menu, and search and click *Ubuntu Pro for WSL*.
3. Click on "I already have a token".
4. Introduce the token you got from your Pro dashboard, and click "Apply".
5. That's it. All new and existing distros with the WSL-Pro-Service installed will be pro-attached next time you start them. You can verify it by starting any WSL distro with WSL-Pro-Service installed, and running:

    ```bash
    pro status
    ```

## Pro-attach via the Windows registry

1. Go to your [Ubuntu Pro dashboard](https://ubuntu.com/pro) to get your [Ubuntu Pro token](../reference/ubuntu_pro_token.md).
2. Open the Registry Editor on your Windows machine
   - To open the Registry Editor, press the Windows key + R and type `regedit`.
3. Go to `HKEY_CURRENT_USER\Software`
4. If it does not exist, create a new key and name it `Canonical`.
   - You can do this with Edit or Right Click > New > Key.
5. If it does not exist, create a new key inside the `Canonical` key and name it `UbuntuPro`.
6. Add a new string value within the `UbuntuPro` key.
   - You can do this with Right Click > New > String value
8. Name this value `UbuntuProToken`.
9. Open the `LandscapeConfig` value and paste your token.
10. That is it. All new and existing distros with the WSL-Pro-Service installed will be pro-attached next time you start them. You can verify it by starting any WSL distro with WSL-Pro-Service installed, and running:

    ```bash
    pro status
    ```

## Read more

- [Ubuntu Pro](../reference/ubuntu_pro.md)
- [How to register your machine to Landscape](./attach-landscape)

## External links

- [Ubuntu Pro](https://www.ubuntu.com/pro)
