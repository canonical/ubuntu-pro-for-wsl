# Ubuntu Pro app translation handoff

The app title is the exact brand `Ubuntu Pro` in every existing explicit title override. Do not translate the brand; locales without an override continue to use normal fallback.

The English source changes three explanatory messages. `tokenInputTitle` introduces configuration for Ubuntu instances. `proHeading` explains that the app automates Ubuntu Pro attachment on WSL and that security services protect Ubuntu instances, not Windows. Preserve the `aboutLink` placeholder and its String type. `ubuntuProEnabled` describes configuration for those instances, not a Windows subscription or protection status. Ubuntu Pro is the subscription, the Ubuntu Pro app is the Windows application, and the Ubuntu Pro client applies attachment inside Ubuntu. A Microsoft Store purchase is one possible subscription source; installing the app does not grant a subscription.

Existing translations of these explanatory keys have not been changed or reviewed. The following locale/key pairs need human translation updates and review before release. Missing keys retain the existing fallback behavior and are not represented as reviewed translations. English source is updated; no human translation review is claimed.

| Locale | Existing keys requiring update | Status |
| --- | --- | --- |
| `ca` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `cs` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `de` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `el` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `en_GB` | `ubuntuProEnabled` | Pending translation and review |
| `eo` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `es` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `et` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `eu` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `fi` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `fr` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `ga` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `gl` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `he` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `hu` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `id` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `ja` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `ka` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `kn` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `ko` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `lo` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `nl` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `oc` | `tokenInputTitle`, `ubuntuProEnabled` | Pending translation and review |
| `pl` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `pt` | `ubuntuProEnabled` | Pending translation and review |
| `pt_BR` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `ro` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `ru` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `sk` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `sv` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `ta` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `tr` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `ug` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `uk` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `zh` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |
| `zh_TW` | `tokenInputTitle`, `proHeading`, `ubuntuProEnabled` | Pending translation and review |

## CLI gettext handoff

The Windows agent runs on the Windows host and manages Ubuntu instances on WSL.
`wsl-pro-service` runs inside those instances and connects them to the agent.
These descriptions refer to components of the Ubuntu Pro app, not to the Ubuntu
Pro subscription or the Ubuntu Pro client.

The `ca_ES` catalog has untranslated changed descriptions for the Windows agent
short and long help and the WSL service long help. Translate and review those
messages before release; the generated template is the source for any other
locale translations. No human translation review is claimed.
