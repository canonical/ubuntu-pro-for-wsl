# Architectural Decision Records

Decisions that are hard to reverse, surprising without context, and the result of a real trade-off.
Numbered sequentially, grouped by section. 'Who' and 'when' are captured by Git.

---

## 1. Packaging and deployment

### 1.01 - Package Windows components as MSIX

* **Problem/Context**: Windows packaging options are mostly complex and third-party. They also lack Package
  Identity needed for in-app purchases.
* **Decision**: Use MSIX — first-party, modern, tool-supported, and more secure than MSI.
* **Consequences**:
  - Positive: Declarative, easier to maintain; clean Store upgrades/uninstall; easy OEM install; App
    Container confinement builds trust.
  - Negative: App Container FS/Registry virtualization overhead; no per-user services; some low-level
    Win32 APIs unavailable.

### 1.02 - Package wsl-pro-service as deb for image inclusion

* **Problem/Context**: Must touch system binaries, run as root, and be present at first boot.
* **Decision**: Package as deb, seeded in the image via Ubuntu archive main pocket — our scope doesn't
  fit a confined snap.
* **Consequences**:
  - Positive: Simple to reason about; shells out to pro client/landscape-register instead of needing
    IPC; leverages systemd.
  - Negative: Debian packaging is non-trivial and needs security review for main; weaker confinement
    than a snap.

### 1.03 - The agent runs as a per-user MSIX startup task, not a Windows service

* **Problem/Context**: The agent must run continuously, but MSIX apps can't ship a per-user Windows
  service — only a startup task, enabled by the OS only after first user interaction.
* **Decision**: Package as the non-elevated per-user MSIX startup task `UbuntuProAutoStart`, which
  launches the Launcher (never the agent binary directly); the GUI relaunches the Launcher if the
  agent is down; a documented registry remediation marks the task "Enabled by Policy" for fleet tools.
* **Consequences**:
  - Positive: Store-compliant, no admin rights, natural per-user HKCU config.
  - Negative: Agent runs only for the logged-in user; first-interaction requirement blocks zero-touch
    deployment (mitigated, not solved, by the racy registry remediation); update restarts need manual
    RegisterApplicationRestart wiring.

### 1.04 - The agent is hosted under an invisible pseudo-console by a separate launcher process

* **Problem/Context**: The agent must run invisibly, but WSL API spawns console processes and Windows
  has no API to suppress the console window a non-console parent triggers.
* **Decision**: Compile the agent as a console app hosted under an invisible ConPTY created by a
  separate windowless Win32 process, the Launcher (a skinny conhost stand-in), which drains console
  output, handles restart-on-update, and is the launch target for both the MSIX startup task and the
  GUI.
* **Consequences**:
  - Positive: No console windows ever appear; killing the Launcher cleanly shuts down the agent
    (including the address file).
  - Negative: Extra C++ component whose lifetime packaging/GUI must coordinate; agent's console I/O
    goes through a synthetic console.

### 1.05 - MSIX registry write virtualization is disabled for the Canonical keys

* **Problem/Context**: MSIX virtualizes the registry by default, but the agent needs
  RegNotifyChangeKeyValue (which doesn't work under virtualization) and sysadmins need to target real
  registry keys.
* **Decision**: Disable write virtualization for HKCU\Software\Canonical\UbuntuPro (and ...\Ubuntu),
  via the restricted capabilities runFullTrust and `unvirtualizedResources`; the agent treats the
  registry as a read-only Configuration Source.
* **Consequences**:
  - Positive: Live change notifications work; remote-deployed config visible as written; one canonical
    location for org config.
  - Negative: Registry state isn't isolated and survives uninstall, requiring manual cleanup docs; the
    restricted capabilities add Store-certification scrutiny.

### 1.06 - Pre-releases are tagged M.9999[ab]p; one git-derived version feeds all components

* **Problem/Context**: MSIX needs a four-part numeric version (MSBuild rejects the fourth element for
  alpha/beta encoding), yet a plain numeric pre-release tag would make Read the Docs publish it as
  latest stable.
* **Decision**: Stable releases: M.m.p. Pre-releases: M.9999[ab]p (minor 9999 = "not for stable
  channel"; a bare M.9999.p tag must fail CI). The full `git describe` string is the single version
  identity, injected at build time into every component (Go `ldflags`, Flutter `--dart-define`, MSIX
  manifest, .appinstaller).
* **Consequences**:
  - Positive: One coherent version across Go/C++/Flutter/MSIX; correct Read the Docs stable/latest
    publishing; every build traces to a commit.
  - Negative: Magic-number convention contributors must learn; would collide with a hypothetical
    stable 1.9999.x series.

### 1.07 - Non-Store installs self-update via an .appinstaller pinned to a stable releases URI

* **Problem/Context**: Corporate environments often block the Store, so sideloaded MSIX needs its own
  update channel; AppInstaller checks the URI baked in at install time, which existing installs can't
  change.
* **Decision**: The published .appinstaller points at the stable "latest release" GitHub download URI,
  which must always resolve to the latest version; update checks run on launch and periodically in the
  background; the build script patches version into manifest and .appinstaller each release.
* **Consequences**:
  - Positive: Automatic updates without Store dependence; one channel for all sideloaded installs.
  - Negative: The asset URI can never change without orphaning existing installs; sideload auto-update
    is limited on Windows 10.

## 2. Security

### 2.01 - Secure the Public Directory via WSL 9P Extended Attributes

* **Problem/Context**: The agent writes runtime state (gRPC address, TLS material, cloud-init data) to
  the Public Directory, projected into every instance via 9P/DrvFs; by default 9P maps files to the
  unprivileged WSL user, exposing private keys and letting any process tamper with them.
* **Decision**: Stamp every node created under the Public Directory with NT Extended Attributes
  ($LXUID=0, $LXGID=0, $LXMOD — directories 040700, files 0100600) at creation, so 9P projects it as
  root-owned. Centralized in the `securefiles` custodian component, with a plain `os` fallback (no EA
  stamping) on non-Windows for cross-platform build/test.
* **Consequences**:
  - Positive: Confidentiality/integrity/availability hold inside every instance; attributes are
    stamped before content is written; `common/certs` stays a pure in-memory generator.
  - Negative: Depends on WSL 9P EA behavior via github.com/Microsoft/go-winio; the parent directory
    remains tamperable by the WSL user (accepted limitation).

## 3. Integration

### 3.01 - Distro instances dial the agent; commands flow as reverse unary calls

* **Problem/Context**: The agent must push Commands to running instances and read their state, but
  under WSL networking instances have no stable, host-routable address.
* **Decision**: Only wsl-pro-service initiates connections: it discovers the agent via the address
  file in the Public Directory, resolves the host address per WSL networking mode, dials with mTLS,
  and serves Reverse unary calls over the client-initiated streams of its Control stream — replacing
  the earlier double-server design.
* **Consequences**:
  - Positive: Works under any WSL networking mode or instance count with zero host-side discovery;
    no listeners/firewall changes inside instances.
  - Negative: Inverts gRPC roles, requiring a custom streams package; the agent can't reach a
    disconnected instance.

### 3.02 - Microsoft Store purchases are resolved into Pro tokens via a Contracts Server round-trip

* **Problem/Context**: A Microsoft Store subscription entitlement exists only in Microsoft's systems;
  the agent can't verify it locally or issue a Pro Token, and must not embed Canonical credentials.
* **Decision**: Follow Microsoft's recommended validation pattern: the agent gets an ephemeral Azure
  AD token, has the Store runtime mint a User JWT bound to it and to an anonymous user hash, and
  submits it to the Contracts Server, which validates against the Store Graph API and returns the Pro
  Token. REST payloads are a cross-repository public contract kept in sync with Canonical's cloud-contracts
  codebase by convention.
* **Consequences**:
  - Positive: Authoritative server-side validation; no Canonical/Entra credentials in the client;
    native Store purchase UX.
  - Negative: Pro attachment needs Contracts Server reachability; schemas must be manually synced
    across repositories; token freshness depends on polling billing-period expiration.

### 3.03 - mTLS with a per-startup self-signed CA and one shared client key pair

* **Problem/Context**: The agent-instance channel crosses the Public Directory trust boundary — any
  local process could otherwise connect — and a public CA doesn't fit (no DNS name, dynamic addresses).
* **Decision**: At every startup the agent generates a fresh self-signed root CA, an agent cert, and
  one client key pair shared by the GUI and all instances, written to the Public Directory under Secure
  Projection. Both sides require/verify peer certs (TLS 1.3 min); certs expire after 30 days, rotating
  naturally on restart; clients re-read material on every (re)connection and pin the fixed server name
  "UP4W".
* **Consequences**:
  - Positive: No external PKI, zero user setup; compromise window bounded by process lifetime + cert
    expiry; reuses ADR-2.01 secure projection.
  - Negative: Every restart rotates the PKI, invalidating existing connections until instances re-read
    it; all clients share one TLS identity, so the agent can't cryptographically distinguish
    instances (WSL name is self-asserted; accepted as one trust domain per Windows user); the fixed
    server-name override looks like a bypass and invites accidental "fixes".

### 3.04 - One C++ core for Microsoft Store APIs, with thin per-language bridges

* **Problem/Context**: Both the agent (Go) and GUI (Dart/Flutter) need Store subscription/purchase
  functionality, but WinRT Store APIs are only ergonomic from C++/WinRT; per-language implementation
  would duplicate logic.
* **Decision**: `storeapi/base` implements the logic once against a swappable context; `storeapi/dll`
  exposes a minimal C ABI (negative-integer error codes, explicit memory ownership) loaded lazily by
  the agent's Go wrapper; the GUI imports the C++ from source via its Flutter plugin — bridges stay as
  thin as possible.
* **Consequences**:
  - Positive: One source of truth for subscription logic, testable independent of the WinRT runtime.
  - Negative: Error codes/enums duplicated and manually synced across C++/Go/Dart; memory ownership
    crosses the ABI; DLL located at runtime via fallback search paths.

## 4. Configuration

### 4.01 - Configuration sources are strictly ordered, and higher sources veto lower ones

* **Problem/Context**: A Pro Token (and Landscape config) can come from several Configuration Sources
  (registry/Organization, Microsoft Store, GUI/User); without ordering, writes would conflict and
  organizations couldn't guarantee fleet-wide policy.
* **Decision**: Model each parameter as per-source slots resolved by fixed precedence (Organization >
  Microsoft Store > User). Lower-precedence writes are rejected while a higher one is active; when the
  higher value disappears, the next lower source activates.
* **Consequences**:
  - Positive: Predictable resolution and authoritative fleet policy; GUI can show the active source
    and go read-only when org-managed.
  - Negative: Users see "higher priority subscription active" errors needing explanation; veto logic
    with checksum-based change detection adds complexity; the planned OEM source requires touching the
    ordering.

### 4.02 - Organization-provided configuration is never persisted to disk

* **Problem/Context**: Registry-supplied Pro Tokens/Landscape config are org-managed policy; copying
  them into the agent's Private Directory store would create a stale-able second copy of
  organizational secrets and blur the source of truth.
* **Decision**: The store persists only User/Store-sourced values. Organization values live in memory
  only (excluded from serialization), re-read from the registry at every start, and tracked by a
  persisted SHA-512 checksum for change detection without storing the data.
* **Consequences**:
  - Positive: Single source of truth for org data; no org token at rest; registry edits need no
    migration/cache invalidation.
  - Negative: Agent can't resolve org config without a registry read at startup; change detection
    depends on checksums; load/serialization code must carefully preserve in-memory org fields.

### 4.03 - Configuration is delivered via cloud-init at first boot and via wsl-pro-service afterwards

* **Problem/Context**: Pro/Landscape config must apply to future instances and already-running ones;
  cloud-init applies only at first boot, while wsl-pro-service only exists once an instance is
  running — neither channel covers the other's case.
* **Decision**: Write config for future/uninitialized instances as cloud-init user data into the
  Public Directory; deliver changes after first boot as Tasks over the instance's Control stream via
  wsl-pro-service.
* **Consequences**:
  - Positive: Zero-touch, race-free provisioning via the ecosystem-standard path; Landscape can deploy
    pre-configured instances declaratively.
  - Negative: Two mechanisms must stay consistent; cloud-init data can't reconfigure an initialized
    instance; reset semantics differ per channel (empty Command detaches/disables vs. cloud-init data
    simply removed).

## 5. Runtime behavior

### 5.01 - Distro instance start-ups are serialized process-wide

* **Problem/Context**: Multiple instances starting simultaneously can freeze WSL and the whole
  machine — a WSL platform limitation hit during bulk operations (attachment, Landscape installs,
  keep-awake cycles).
* **Decision**: A single process-wide mutex serializes all instance wake-ups; every start-up path must
  hold it, so concurrent starts queue.
* **Consequences**:
  - Positive: Bulk operations can't freeze the host.
  - Negative: Bulk wake-ups are slower; the constraint is invisible in code, so removing the mutex
    looks like a safe optimization but would reintroduce the freeze.

### 5.02 - wsl-pro-service gives up connecting and lets systemd restart it on a long delay

* **Problem/Context**: wsl-pro-service is preinstalled in Ubuntu on WSL images and routinely runs
  where the agent doesn't exist; endless retries would spam the journal and delay boot — yet the
  instance must still converge when an agent does appear, without a reboot.
* **Decision**: Retry connecting with bounded in-process exponential backoff; on exhaustion, exit
  successfully and let systemd (Restart=always) restart it after a long delay.
* **Consequences**:
  - Positive: Silent, cheap idling on agent-less machines (no journal spam/boot delay); automatic
    recovery within a bounded time once the agent appears.
  - Negative: A merely transient failure also waits out the long delay — systemd can't vary restart
    delay by exit condition — baking a slow worst-case reaction time into the product.

### 5.03 - The distro name is discovered by reading WSL2_DISTRO_NAME from PID 2's environment

* **Problem/Context**: wsl-pro-service needs its instance's name (Control stream handshake, Landscape
  metadata, logging), but WSL exposes it only in the WSL init process's environment, which systemd
  services don't inherit, and there's no supported API to learn it.
* **Decision**: The systemd unit extracts WSL2_DISTRO_NAME from PID 2's environment (the forked WSL
  /init, which retains host-provided variables PID 1 cleared when executing systemd) into a runtime
  environment file; Go code then falls back through WSL_DISTRO_NAME, WSL2_DISTRO_NAME, and parsing
  `wslpath` output.
* **Consequences**:
  - Positive: Reliable name resolution from unit start, across WSL versions.
  - Negative: Depends on an undocumented WSL process-layout invariant (PID 2 = forked /init); failures
    are silent (empty name means an arbitrary connection identity); must be revisited if Microsoft
    ships a supported mechanism.

### 5.04 - Distro instances are kept awake by a reference-counted keep-awake poll, not shut down explicitly

* **Problem/Context**: An instance must stay running for its wsl-pro-service control stream to process
  Tasks, but WSL shuts it down once nothing external holds it open, and the agent has no OS primitive
  to pin it up short of interacting with it.
* **Decision**: Each instance carries a reference-counted awake lock (`LockAwake`/`ReleaseAwake`);
  while count > 0, a background loop periodically pokes the instance to defeat WSL's idle shutdown,
  restarting the poke loop across instance restarts. Releasing the last lock doesn't shut the instance
  down — it just stops artificially keeping it alive, leaving WSL's idle timeout in control.
* **Consequences**:
  - Positive: Reliable task delivery regardless of open terminals; the agent never has to explicitly
    stop an instance to end keep-awake.
  - Negative: The agent silently keeps instances running/consuming resources whenever work is pending,
    which can surprise users; polling interval trades responsiveness vs. overhead; removing the poke
    loop looks like safe cleanup but would silently reintroduce undelivered tasks.

## 6. Testing

### 6.01 - End-to-end tests install the production MSIX and drive the real system

* **Problem/Context**: Unit/integration tests can't cover packaging, registry watching, startup tasks,
  or real WSL registration — exactly where the components integrate. The Microsoft Store runtime is
  especially hard to mock: real WinRT APIs need a deployed, Store-associated app and a signed-in user,
  making the production purchase path untestable on a dev machine or CI.
* **Decision**: The end-to-end suite runs on a Windows+WSL host, installs the real MSIX and
  wsl-pro-service deb, registers real distro instances, and starts the agent through the GUI (the
  production entrypoint), while external services are redirected to mocks via compile-time switches
  shared with production binaries. For the Store, WinRT is abstracted behind a context interface whose
  mock talks HTTP to a configurable REST mock, using magic trigger inputs for specific error paths,
  shared by C++ and Go tests.
* **Consequences**:
  - Positive: Full-system fidelity including packaging regressions; the only tests exercising the real
    deployment path end to end; deterministic purchase/subscription flows without a Store account.
  - Negative: Requires a dedicated self-hosted Windows runner and mutates the host; slow and fragile,
    so some tests ship disabled; test/production binaries differ at compile time; the Store mock can
    drift from real WinRT, and its magic-string protocol implicitly couples C++ tests and the Go mock.
