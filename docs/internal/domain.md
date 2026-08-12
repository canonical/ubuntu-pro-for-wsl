# Domain Model

Ubiquitous language for Ubuntu Pro for WSL. This is a glossary — definitions only, no implementation
details, no specs, no scratch-pad content. Keep entries sorted alphabetically.

---

### Address File

The file the *Windows Agent* writes under the *Public Directory* at every startup, holding its
current gRPC listening address. Removed on clean shutdown, rewritten on every restart (which also
rotates the mTLS material, per ADR 3.03). *wsl-pro-service* and other clients read it to locate a
running agent, since the agent has no fixed or discoverable dial address.

### Agent

See *Windows Agent*.

### Command

An instruction the *Windows Agent* sends to a distro instance over its *control stream* (apply a *Pro
Token*, or apply a *Landscape* client configuration). Declarative: carries the desired end state, and
an empty payload resets to unconfigured — empty Pro token detaches, empty Landscape config disables
registration. Contrast with commands a *Landscape* server sends to the *Landscape Host Agent*, a
separate channel.

### Configuration Source

The provenance of a configuration parameter (*Pro Token* or *Landscape* client configuration): *User*
(manual GUI input), *Microsoft Store* (a *Microsoft Store subscription* purchase), or *Organization*
(a registry key deployed by remote management). Strictly ordered by precedence — Organization >
Microsoft Store > User — and a lower-precedence source cannot replace an active higher-precedence
value.

### Contracts Server

Canonical's hosted backend bridging Microsoft Store purchases to Ubuntu Pro. The *Windows Agent*
calls it to obtain an ephemeral Azure AD token and validate a *Microsoft Store subscription*
entitlement (presented as a *User JWT*), receiving the *Pro Token* in exchange.

### Control stream

The long-lived gRPC streams a distro instance's *wsl-pro-service* opens to the *Windows Agent* and
keeps open for the connection's life: one instance-state stream plus one command stream per command
type (Pro attachment, Landscape). The only channel between a distro instance and the agent. Every
stream opens with a handshake carrying the instance's WSL name, binding all its streams to one
identity.

### Deferred Task

A *Task* submitted without waking the distro instance: it waits until the instance next runs (e.g.
its next connection) before executing.

### Distro

Ambiguous shorthand: either the Linux distribution as a distributable package (what `wsl --install`
downloads, tar-based or msix) or a materialized *Distro instance*. Code uses `distro` pervasively in
the latter sense; in prose, prefer *Distro instance* for a materialized system.

### Distro identity

The (name, GUID) pair uniquely identifying a distro instance. Name alone isn't stable — WSL allows
unregistering and re-registering under the same name — so the *Windows Agent* validates the stored
GUID against the system and treats a mismatch as a different instance, invalidating the old entry.

### Distro Database

The *Windows Agent*'s persisted registry of managed distro instances, held in the *Private
Directory*. Each entry stores a *Distro identity* and configuration state, validated against the
system so a stale entry (name reused by a different GUID) is detected and invalidated. Reconciled at
runtime against distro instances WSL reports with no entry — *Unmanaged distro instances* — which
become managed on first `wsl-pro-service` connection.

### Distro instance

A named Ubuntu on WSL system registered with WSL — a materialized instance of a *Distro* package,
whether or not the *Windows Agent* manages it.

### GUI

The `ubuntupro.exe` graphical app on the Windows host: the end-user configuration surface. Starts the
*Windows Agent* via the *Launcher* if not running and submits *Pro Token*/*Landscape* configuration
to the agent over gRPC — never writes configuration to the registry or filesystem itself.

### Instance

Synonym for *Distro instance* in WSL context. Prefer *Distro instance* in internal prose; bare
*instance* may appear in user-facing text and external vocabularies (e.g. the Landscape web portal
calls the Windows host itself an "instance").

### Landscape

A Canonical system-management product. Each distro instance's *Landscape client* connects to a
configured Landscape server so sysadmins can monitor and enforce compliance fleet-wide.

### Landscape client

The `landscape-client` daemon pre-installed in every distro instance. Connects to the *Landscape*
server, reports status, and executes server instructions locally. *wsl-pro-service*
enables/configures/disables it per the *Windows Agent*'s configuration.

### Landscape Host Agent

The role the *Windows Agent* plays toward the *Landscape* server: connects to the Landscape
HostAgent API, reports the Windows host and its distro instances (managed and unmanaged) as
inventory, and executes host-level commands (install/start/stop/uninstall instances, shut down
host). Addressed by its *Landscape Host Agent UID*, assigned on first contact.

### Landscape Host Agent UID

The identifier the *Landscape* server assigns the *Windows Agent* on first contact; persisted and
injected into every distro instance's `landscape-client` config as `hostagent_uid`, grouping all
instances under their host in Landscape. Any user-supplied `hostagent_uid` is overridden, but may
cause server rejection.

### Launcher

The `ubuntu-pro-agent-launcher.exe` process on the Windows host: a windowless Win32 app hosting the
*Windows Agent* under an invisible pseudo-console so console processes spawned on the agent's behalf
never surface windows. A startup entrypoint — the MSIX startup task and the *GUI* both launch it,
never the agent directly.

### Microsoft Store subscription

A Store subscription add-on for Ubuntu Pro for WSL, purchased e.g. via the *GUI*. A *Configuration
Source* of a *Pro Token*: the purchase lives in Microsoft's systems and only yields a Pro Token once
the *Contracts Server* validates the entitlement. Not itself an Ubuntu Pro subscription — the
purchase that, once validated, unlocks one.

### Private Directory

The filesystem directory `%LocalAppData%\Ubuntu Pro` on the Windows host (virtualized when deployed
via msix), accessed only by the *Windows Agent*. Holds its private state: config store, distro
database, per-distro task queues, single-instance lock file. Contrast with the *Public Directory*,
located for easy access from within distro instances.

### Pro Token

An opaque credential from Canonical's Contracts API authorising a machine to access Ubuntu Pro
services (ESM, FIPS, etc.). The *Windows Agent* holds the active token and instructs each distro
instance's *Ubuntu Pro client* to attach with it.

### Public Directory

The filesystem directory `%UserProfile%\.ubuntupro` on the Windows host: the trust boundary across
which the Windows Agent (sole legitimate author) writes files consumed by the root-privileged
`wsl-pro-service` inside each distro instance. Every node at or below it must carry a *Secure
Projection* so unprivileged Linux processes can't tamper with or read its contents. The guarantee
covers the directory and all descendants; tamperability of its parent (`%UserProfile%`) via the WSL
user is an accepted WSL-design limitation.

### Reverse unary call

An RPC whose direction is inverted relative to the connection: the party that accepted the connection
(the *Windows Agent*, gRPC server) issues the call, and the dialling party (a distro instance's
*wsl-pro-service*, gRPC client) executes it and returns the result on the same bidirectional stream.
How commands reach a distro instance over its *control stream*.

### Secure Projection

The property of a node under the *Public Directory* whereby the WSL 9P server translates its NT
Extended Attributes (`$LXUID`, `$LXGID`, `$LXMOD`) into Linux-visible ownership/mode, appearing
root-owned (`uid=0`, `gid=0`) with mode `0600` (files) or `0700` (directories). Denies all access to
unprivileged Linux processes, preserving confidentiality, integrity, and availability within any
distro instance.

### `securefiles`

The internal Windows Agent component (`windows-agent/internal/securefiles`), sole custodian of the
*Public Directory*. Constructed once with the base path; exposes creation primitives (`MkdirAll`,
`WriteFile`, `Create`) that stamp every node with the correct NT Extended Attributes (`$LXUID=0`,
`$LXGID=0`, `$LXMOD`) before content is written. No other agent component creates nodes there
directly.

### Task

A unit of configuration work (Pro attachment, Landscape config) queued for a distro instance and
executed over its `wsl-pro-service` gRPC connection. Persisted to disk, so pending work survives
*Windows Agent* restarts; retryable failed tasks are re-queued.

### Ubuntu Pro client

The `pro` command-line tool (package `ubuntu-pro-client`, formerly `ubuntu-advantage-tools`)
pre-installed in every *Distro*. In-distro executor of Pro attachment: `wsl-pro-service` invokes it
to attach with the active *Pro Token*, detach, and query attachment status.

### Ubuntu Pro for WSL

The product this repository builds: the *Windows Agent*, *Launcher*, and *GUI* packaged for Windows,
plus the *wsl-pro-service* deb shipped in Ubuntu, automating Pro attachment and Landscape enrollment
of distro instances. Abbreviated **UP4W** in code (the `UP4W_*` env-var prefix, gRPC TLS server name)
and build machinery (CMake project name). An internal convenience acronym — never in user-facing
text.

### Unmanaged distro instance

A distro instance the *Windows Agent* doesn't and can't manage: no database entry, no control
channel, so no Pro attachment or Landscape config can be enforced. Discovered on demand and reported
to *Landscape* as host inventory; the server may still request its uninstallation. Becomes managed
when its *wsl-pro-service* first connects to the agent.

### User JWT

The Microsoft Store "user ID key": a JWT minted on demand by the Store runtime, binding an Azure AD
token (vended by the *Contracts Server*) to an anonymous hash of the local Windows user. The *Windows
Agent* sends it to the Contracts Server, which queries Microsoft about the user's purchases on the
app's behalf. Neither a *Pro Token* nor a user credential.

### Windows Agent

The `ubuntu-pro-agent.exe` process on the Windows host, running with user privileges: the system's
orchestrator. Reads configuration from the registry and its *Private Directory*, manages the
lifecycle of all managed distro instances, and writes authoritative runtime state (address,
certificates, cloud-init data) to the *Public Directory* for `wsl-pro-service`.

### Worker

The per-distro-instance component owning and executing that instance's *Task* queue. Persists queued
tasks to disk (surviving *Windows Agent* restarts), processes them one at a time against the
instance's gRPC connection, and distinguishes an unreachable instance (invalidating its *Distro
Database* entry) from a task requesting retry at the instance's next connection.

### `wsl-pro-service`

A systemd unit running inside each distro instance with root privileges. Reads runtime state from the
*Public Directory* (projected via 9P/DrvFs), connects to the *Windows Agent* over mTLS gRPC, and
applies Pro attachment and Landscape configuration locally.
