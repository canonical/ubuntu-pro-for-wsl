# Ubuntu Pro app translation handoff

The app title is the exact brand `Ubuntu Pro` in every existing explicit title override. Do not translate the brand; locales without an override continue to use normal fallback.

## CLI gettext handoff

The Windows agent runs on the Windows host and manages Ubuntu instances on WSL.
`wsl-pro-service` runs inside those instances and connects them to the agent.
These descriptions refer to components of the Ubuntu Pro app, not to the Ubuntu
Pro subscription or the Ubuntu Pro client.

The `ca_ES` catalog has untranslated changed descriptions for the Windows agent
short and long help and the WSL service long help. Translate and review those
messages before release; the generated template is the source for any other
locale translations. No human translation review is claimed.

## Release gate

Before publication, the user must complete and review the changed CLI gettext
descriptions in each affected locale. The generated catalogs are not evidence of
human review. This gate is separate from missing visual captures and environmental
verification blockers.
