# How to auto-register WSL distros to Landscape with UP4W
You can use a private Landscape instance (different from [landscape.canonical.com](https://landscape.canonical.com)). It must be over HTTP, as using certificates is not yet supported. To do so, follow these steps:
1.  Find registry key `HKEY_CURRENT_USER\Software\Canonical\UbuntuPro`, field `LandscapeClientConfig`.
2.  Copy the contents of the Landscape configuration file into the registry key:
    ```ini
    [host]
    url= The URL of the Landscape hostagent API

    [client]
    # The configuration for the WSL client. See an example here
    # https://github.com/canonical/landscape-client/blob/master/example.conf
    ```
3. The changes will take effect next time you start Ubuntu Pro For Windows. All new distros will automatically become landscape-enabled. If you want them to be applied now, follow the steps on how to restart Ubuntu Pro For Windows. Otherwise, you're done.
