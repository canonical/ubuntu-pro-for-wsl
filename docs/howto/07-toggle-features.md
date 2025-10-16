---
myst:
  html_meta:
    "description lang=en":
      "For developers who are testing, debugging or developing the application."
---

# Enable opt-in features of Ubuntu Pro for WSL during development

```{include} ../includes/dev_docs_notice.txt
    :start-after: <!-- Include start dev -->
    :end-before: <!-- Include end dev -->
```

Some features in UP4W are opt-in or can be toggled on and off via the Windows Registry.
While the code is arranged such that CI always tests with those features enabled,
when running UP4W on your machine, you may need to toggle them on and off
explicitly via the Windows registry. This guide shows you how to do that.

## Enable subscribing via the Microsoft Store

1. Open the Windows registry editor: press `Win + R`, type `regedit` and press `Enter`.
1. Navigate to the following key: `HKEY_CURRENT_USER\Software\Canonical\UbuntuPro\`.
1. Create a new DWORD value named `AllowStorePurchase` and set it to `1`.

The next time you open the GUI you'll find the button to subscribe to Ubuntu
Pro via the Microsoft Store.

```{warning}
This can incur real charges if you proceed with the purchase.
```

## Disable Landscape configuration in the GUI

Landscape configuration page and related buttons are enabled by default, but can be disabled via registry.

1. Open the Windows registry editor: press `Win + R`, type `regedit` and press `Enter`.
1. Navigate to the following key: `HKEY_CURRENT_USER\Software\Canonical\UbuntuPro\`.
1. Create a new DWORD value named `LandscapeConfigVisibility` and set it to `0`.

The next time you open the GUI, you'll find that the Landscape configuration
page can be shown via the set up wizard or by clicking on the 'Configure
Landscape' button.
If that value is not present or set to anything other than `0`, Landscape configuration page
and related buttons will be visible.
