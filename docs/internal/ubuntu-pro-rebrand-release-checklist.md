# Ubuntu Pro app rebrand: release handoff

Version **1.1.2** is a recommendation, not a version constant. The release tag
continues to supply the version to all components. Nothing in this checklist
asserts that a release has been published.

## Proposed customer-facing copy

- **Title / winget PackageName:** Ubuntu Pro
- **Short description:** Automate Ubuntu Pro attachment and Landscape configuration for Ubuntu on WSL.
- **Full description:** The Ubuntu Pro app for Windows configures compatible Ubuntu
  instances on WSL for Ubuntu Pro and Landscape. It automates attachment with
  your Pro token and can register instances with your Landscape server. An Ubuntu
  Pro subscription is required for subscription services; the Ubuntu Pro client
  runs inside each instance. Installing the app alone does not grant a subscription.
- **Release note:** Ubuntu Pro for WSL is now called Ubuntu Pro. This is the same
  Windows app for managing Ubuntu instances on WSL; existing installations,
  configuration and entitlements are retained on update.

Use the unchanged winget package ID `Canonical.UbuntuProforWSL` in commands and
release notes. Review localizations and platform wording before publication.

## Repository completion gates

- [ ] Obtain and review all five renamed-build captures listed in
  `docs/internal/ubuntu-pro-rebrand-capture-checklist.md`. The four screenshots
  and README GIF are user input, currently missing; existing visuals remain in
  place. Repository visual work is not complete until replacements are checked
  for title, current copy, readability and privacy.
- [ ] Run applicable native checks and address relevant failures. See verification
  evidence below. Do not repair unrelated tests or add branding-string tests.
- [ ] Recheck the final integrated diff for technical identity and release artifacts.
  Keep package names, commands, protocol fields, gettext domain, registry paths,
  `UP4W` identifiers, links and historical downloads unchanged.
- [ ] Complete and review all affected explanatory translations before release;
  the user owns this gate. See `rebranding-translations.md` for locale/key and
  gettext context. Mechanical changes are not human review.

## User-owned publication steps

1. In Partner Center reserve the display name for the **existing** app
   `9PD1WZNBDXKZ`; do not create a replacement product. Review title, short and
   full description, release notes and screenshots for every listing language.
   Review each add-on title, description and screenshot. Preserve purchase ID
   `9PBDP6SFLM8G` and verify existing subscription entitlements remain usable.
2. Submit the updated app for Store certification and wait for approval. Retrieve
   the Store-signed `UbuntuPro.msixbundle` for the intended version. Verify its
   signature, package identity `CanonicalGroupLimited.UbuntuPro`, publisher and
   package family name `CanonicalGroupLimited.UbuntuPro_79rhkp1fndgsc`.
3. Prepare the GitHub release for the selected tag with the signed
   `UbuntuPro.msixbundle` and `UbuntuProForWSL.appinstaller`. Confirm the latter
   points to that tag's bundle and retains its stable update URI:
   `https://github.com/canonical/ubuntu-pro-for-wsl/releases/latest/download/UbuntuProForWSL.appinstaller`.
   Preserve old release assets and downloads. Publish only after certification,
   verification and release gates are satisfied.
4. Submit a new-version winget manifest for `Canonical.UbuntuProforWSL`, retaining
   moniker `ubuntu-pro` and PFN `CanonicalGroupLimited.UbuntuPro_79rhkp1fndgsc`.
   Update PackageName, descriptions, bundle URL and SHA256 from the signed bundle;
   review community automation metadata instead of assuming it updates correctly.
   Leave older version manifests and URLs untouched. Existing winget ARM64
   coverage is separate work, not part of this rebrand.
5. Check availability of Chocolatey ID `ubuntu-pro` and submit/update it only
   when available and appropriate. External moderation is nonblocking; do not
   claim the package is published or reserved without confirmation.
6. Coordinate documentation publication and external Canonical download and
   Landscape pages, including their button captions and screenshots. Those
   services have independent owners; do not describe changes as live before they
   are published.

## Controlled Windows validation and recovery

- [ ] On a test Windows host verify Store, installed-app and winget display metadata.
- [ ] Upgrade an existing installation in place and verify retained configuration,
  startup policy and an existing WSL instance's agent connection. Check both
  Store and AppInstaller update paths where applicable.
- [ ] With an authorized Store test account verify purchase ID and existing
  entitlements externally. The disabled purchase tests do not establish this.
- [ ] If publication fails, stop promotion, preserve historical downloads and the
  existing AppInstaller URI, restore a known-good release at that stable endpoint,
  and coordinate Store, GitHub and package-manager rollback with their owners.
  Never migrate the existing AppInstaller URI as a rebrand workaround.

## Repository verification evidence

- CLI documentation generators succeeded on Windows; both gettext generators
  succeeded under Ubuntu WSL with gettext. The Windows locale generators lacked
  `xgettext`. Protobuf generation succeeded with `agentapi/generate.sh` using
  protoc 3.21.12 and Go/Dart plugins; the schema change is comment-only.
- Focused Windows Go tests passed for the Windows agent. The Linux service suite
  failed on Windows (executable suffix assertion and a nil daemon panic), but
  passed on Ubuntu WSL. These platform-specific failures were not repaired.
- `golangci-lint` reported existing gosec/staticcheck findings and Windows gci
  formatting reports; this broad check did not pass. Recheck in the project CI
  environment before release.
- Windows lacks `make`. In WSL, `make -C docs html` built output but failed on
  six last-updated Git warnings: the worktree `.git` file contains a Windows
  path that WSL Git cannot resolve. Targeted `vale` and `spelling` with
  `CHECK_PATH=explanation/pro-explanation.md` passed.
- WSL `make -C docs lint-md` failed: source MD032 findings in
  `howto/custom-rootfs-multiple-targets.md:20`,
  `howto/data-science-and-engineering.md:107`, `howto/run-workflows-azure.md:59`,
  `reference/actions.md:17`, and `reference/landscape_config.md:39`, plus MD023
  in `internal/go-standards.md:80`. Comparison with `rebrand` confirms those
  lines predate this change. Generated `_build` files also produced MD032 and
  MD014 findings; they are not source changes. Do not repair unrelated baseline.
- WSL `make -C docs linkcheck` failed on one external link:
  `howto/data-science-and-engineering.md:111`, `https://askubuntu.com/` returned
  HTTP 403. Recheck network access before release.
- The current `winget show` reports 1.1.1, moniker `ubuntu-pro`, and a historical
  `UbuntuProForWSL.msixbundle` URL. Keep that old URL; only future bundle URLs use
  `UbuntuPro.msixbundle`.

Remaining old-name occurrences are expected in compatibility identifiers and
paths, historical version/download references, explicit former-name guidance and
external captions awaiting their owners. Review the final integrated diff for
unintended active prose or stale new-release bundle references; those are defects,
not compatibility exceptions. Repository completion is distinct from Store,
winget, Chocolatey, documentation and external website publication.
