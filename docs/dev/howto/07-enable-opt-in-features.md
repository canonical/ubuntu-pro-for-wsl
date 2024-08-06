# How to enable opt-in features

Some features in UP4W are opt-in. While the code is arranged such that CI
always tests with those features enabled, when running UP4W on your machine,
you need to enable them explicitly via the Windows registry. This guide shows
you how to do that.

## Enable Landscape configuration in the GUI

1. Open the Windows registry editor: press `Win + R`, type `regedit` and press `Enter`.
1. Navigate to the following key: `HKEY_CURRENT_USER\Software\Canonical\UbuntuPro\`.
1. Create a new DWORD value named `ShowLandscapeConfig` and set it to `1`.

The next time you open the GUI, you'll find that the Landscape configuration
page can be shown via the set up wizard or by clicking on the 'Configure
Landscape' button.

## Enable subscribing via the Microsoft Store

1. Open the Windows registry editor: press `Win + R`, type `regedit` and press `Enter`.
1. Navigate to the following key: `HKEY_CURRENT_USER\Software\Canonical\UbuntuPro\`.
1. Create a new DWORD value named `AllowStorePurchase` and set it to `1`.

The next time you open the GUI you'll find the button to subscribe to Ubuntu
Pro via the Microsoft Store.

```{warning}
Beware that can incur in real charges if you proceed with the purchase.
```
