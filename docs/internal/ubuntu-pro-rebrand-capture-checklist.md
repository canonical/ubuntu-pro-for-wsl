# Ubuntu Pro app rebrand capture checklist

These captures must come from the renamed build on Windows. The existing assets
are placeholders until supplied and reviewed; do not paint over their titles.
Use a test environment and English UI. Capture the full app window, including its
`Ubuntu Pro` title, at a readable scale. The current screenshot dimensions are
1111 × 742 pixels; match them where practical, and save as PNG.

1. `docs/assets/token-input-placeholder.png`: start with no configured Pro token.
   Open the app and capture the token entry screen with the field empty and the
   new explanatory copy visible.
2. `docs/assets/status-complete.png`: configure a test subscription token in the
   app, confirm, and capture the completed status screen with the Landscape and
   detach controls visible. Do not expose the token.
3. `docs/assets/landscape-config-ui.png`: from the completed screen select
   **Configure Landscape**. Capture the manual configuration screen with empty
   fields and its advanced configuration option visible.
4. `docs/howto/assets/status-complete-registry.png`: in an isolated test user,
   configure `HKCU:\Software\Canonical\UbuntuPro` `UbuntuProToken` with a
   test token, restart the Windows agent and open the app. Capture the resulting
   organization-managed status screen, without the registry editor or token.
   Remove the test registry value afterwards.
5. `img/readme-pro.gif`: record the renamed app demonstrating token entry and
   completed configuration, preserving the existing README animation's purpose.
   Export an animated GIF at approximately 1865 × 1002 pixels with readable
   window text. Do not show typing or displaying a real token.

Before sharing, inspect every frame for tokens, usernames, hostnames, account
information, notifications and other personal data. Use empty or fictional
values; crop unrelated desktop content without obscuring the app title or state.
Supply the five files at the paths above for visual review. Confirm the title,
current explanatory copy and intended state in each image; verify animation
frames, readability and privacy before replacing existing assets.
