# How to auto-register distros to Landscape
You can use a private Landscape instance (different from [landscape.canonical.com](https://landscape.canonical.com)). It must be over HTTP, as using certificates is not yet supported. To do so, follow these steps:
1.  Find registry key `HKEY_CURRENT_USER\Software\Canonical\UbuntuPro`, field `LandscapeConfig`.
2.  Copy the contents of the Landscape configuration file into the registry key:
    ```ini
    [host]
    # The main URL for the landscape server to connect this client to. If you
    # purchased a Landscape Dedicated Server (LDS), change this to point to your
    # server instead. This needs to point to the message-system URL.
    #
    # Please pay special attention to the protocol used here, since it is a common
    # source of error.
    url = https://landscape.canonical.com/TODO-HOSTAGENT-ENDPOINT

    [client]
    # The configuration for the WSL client. See an example here
    # https://github.com/canonical/landscape-client/blob/master/example.conf
    ```
3. The changes will take effect next time you start Ubuntu Pro For Windows. All new distros will automatically become landscape-enabled. If you want them to be applied now, follow the steps on how to restart Ubuntu Pro For Windows. Otherwise, you're done.
