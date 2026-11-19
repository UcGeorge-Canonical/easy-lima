# Lima configuration, VM trust and port exposure

**Status:** Normative internal corrections for ELD-03, ELD-04 and ELD-05, selected on 2026-11-19 under the author's delegation. These contracts supplement [the design](design_draft.md). Lima 2.2.1 remains a qualification target; the mechanisms below have not been runtime-tested or independently re-reviewed.

## Frozen namespace configuration (ELD-03)

Lima reads namespace defaults and overrides when loading an instance, so freezing only `lima.yaml` does not freeze its effective inputs. Its creation-time base is another input. [Tagged loader](https://github.com/lima-vm/lima/blob/v2.2.1/pkg/limayaml/load.go), [tagged global-configuration contract](https://github.com/lima-vm/lima/blob/v2.2.1/templates/default.yaml#L625).

V1 deliberately supports **no semantic namespace defaults, overrides or creation base in operations that consume those inputs**. `_config/default.yaml` and `_config/override.yaml`, plus `_config/base.yaml` for creation, may be absent, empty/comment-only, or a single empty mapping. A nonempty mapping, multiple documents, aliases, invalid YAML, unreadable content, symlink or non-regular entry is an unsupported namespace input. Report the exact path and offending key/type without echoing arbitrary values. Do not rewrite, flatten into, remove or temporarily rename a shared namespace file. Native instance YAML and its supported materialized base graph remain available within the existing custom-template boundaries.

This explicit v1 boundary avoids maintaining an approximate copy of Lima's global merge engine. An existing root with semantic start-time defaults/overrides is ineligible for create/adoption/start until the user resolves that shared configuration outside the operation; easy-lima does not recommend a destructive blanket deletion or switch roots. Read-only diagnostics remain available. Selecting a root containing unrelated instances does not authorize changing their configuration.

### Retained inputs and admission

Each environment revision contains an `effective_config_manifest` with:

- Contract version, selected Lima executable/build, host/driver, instance name and namespace ID.
- Exact instance YAML digest and approved owned-field snapshot; resolved local template/asset dependency digests and the concrete variables used for path/template expansion.
- A record for each of the three global paths: role (`start` or `creation`), `absent` or `present_empty`, parent/path identity, byte length and digest when present. Absence is an observation, never the digest of an empty file.
- The effective mount, port, transport, resource, provisioning and asset ledger checked against the preview. Explicitly pin governed values in shipped YAML; remaining built-in semantics are tied to the qualified Lima contract.

Read each input through verified namespace/path handling and compare identity/content before and after reading. Bounds are 1 MiB per namespace YAML file and 8 MiB per instance YAML file; larger files fail before parsing. Use a bounded node parser with duplicate-key rejection. No template execution, download or Lima invocation is needed to reject an unsupported global file.

Check start-time defaults/overrides before create, start/restart, native edits, adoption retirement, and any retry which will dispatch such an operation. Creation and creation retries additionally check `base.yaml` and every materialized base dependency. A completed instance retains its historical creation-base evidence; a later `base.yaml` change alone is reported as creation-input drift and does not invalidate an existing start, because it is not a start-time input. Never silently apply it to that instance.

Take a shared namespace-configuration claim during validation and dispatch. An explicit acceptance of changed *supported* inputs uses the environment-update preview and a new immutable revision; ordinary start/resume never updates the baseline. A new empty file, deletion, or changed comment bytes still changes recorded input evidence and needs that deliberate acceptance. Nonempty globals remain unsupported even with `--yes`. A namespace-wide file change can affect several registrations; each retains its own baseline, and accepting it for one cannot silently update the others.

All easy-lima creation/edit actions recheck these inputs immediately before dispatch and compare again after dispatch before publishing readiness. A changed input during a potentially mutating call yields unknown/drift and retains claims for reconciliation; it is not a retry signal. An OS lock cannot prevent an unrelated raw Lima process or editor from changing these files. The same cooperative-management limitation as the existing start/create race applies; do not claim preflight makes external edits atomic.

### Other namespace and inherited inputs

The qualified adapter maintains an explicit input-read inventory, including the installed distribution's guest-agent/assets, `_templates` references and `_config/networks.yaml` when used. V1 rejects a native configuration referring to an unmanaged or named host network whose additional inputs and exposure cannot be enumerated under the existing networking boundary. An unused `networks.yaml` need not block a default user-network VM. New adapter-consumed files discovered in qualification must join the manifest or be rejected before that build is advertised.

Lima transport private keys remain referenced credentials, not configuration bytes copied into the manifest; retain their expected public fingerprints and ownership evidence. Clear inherited `SSH`, personal agent/askpass variables and unapproved Lima/driver overrides; explicitly select the executable, namespace and forwarding backend. Disable proxy-environment propagation for shipped configurations and retain any expressly configured proxy value using the existing secret-handling rules. No change to the process's global HOME is permitted.

Adoption validates consumed start-time inputs and the existing flattened instance configuration before retiring old ownership; it records historical base provenance without reapplying a creation base. A global-file conflict leaves the prototype record and all namespace files unchanged. Stopped instances can be checked from their configuration without a boot. Read-only inspection reports recorded versus observed digests and unverified fields without refreshing the manifest or initializing missing files.

## VM SSH host-key trust (ELD-04)

Lima's generated SSH settings are connection hints, not an enrolled VM trust record: the tagged implementation disables strict checking and localhost host authentication. easy-lima must construct its own managed transport policy instead of including those settings wholesale. [Lima SSH defaults](https://github.com/lima-vm/lima/blob/v2.2.1/pkg/sshutil/sshutil.go#L427).

### Initial enrollment and trust assumption

Use explicit, recorded **trust on first use of the local VM endpoint** during an authorized create/adoption workflow, or its saved pending enrollment on the next explicit start. Initial trust assumes the local host, selected Lima installation and owned instance endpoint are trustworthy. It is not an independent cryptographic attestation of the VM. Mention this enrollment in the existing setup/adoption preview; do not introduce a second routine host-key prompt. Routine connections, the alias proxy and diagnostics never enroll or replace a key.

1. Verify namespace/volume, instance ownership, configuration and disk evidence. Resolve the running instance's concrete loopback address/port and transport public-key identity from allowlisted Lima observations; a non-loopback endpoint, changed instance, ambiguous mapping or unsupported transport refuses enrollment.
2. If adoption provides an unambiguous previously pinned public VM host key with supported provenance, preserve it and require a match. An entry for some other localhost service, `/dev/null`, disabled checking or an old port number alone is not such evidence. Otherwise record the initial local-first-use policy in the adopted plan. A stopped adopted VM keeps a pending enrollment tied to its stable registration and next explicit start; adoption does not boot it merely to obtain a host key.
3. Perform one bounded direct SSH handshake, with no profile agent, no forwarding, no command and no reused master. Capture the negotiated public server key and prove server possession during that handshake. Authenticate only with the recorded Lima VM-login key. Capture and use the same handshake key; do not run a separate scan followed by an unchecked connection. Only supported host-key algorithms are accepted, initially Ed25519 or RSA with SHA-2 signatures; legacy `ssh-rsa` SHA-1 is not enabled.
4. Recheck endpoint/instance evidence and commit the key and its source to authoritative state. Initial key acquisition through this local channel is TOFU; successful client authentication does not independently validate the server's identity. A candidate observed before a crash can be retained as pending evidence but cannot authorize forwarding. Resume against that same candidate; a changed candidate is a conflict requiring explicit recovery, not a new silent first use.
5. Publish the exact pinned public key to a private owned known-hosts export, verify its digest, then open a **new strictly verified** connection. Only after that succeeds may guest setup, a shell/editor, or a profile tunnel use the managed transport.

Lima may use its own bootstrap/maintenance SSH internally during `limactl start`; easy-lima does not claim to retrofit that internal client with this pin. Those subprocesses receive no Git-profile or ambient personal agent. The pin governs all subsequent easy-lima-managed guest SSH operations and aliases. A compromised local host is outside the development-workload trust model.

### Trust record, effective options and recovery

`vm_trust` contains registration/namespace IDs, instance binding, trust generation, supported algorithm/public key and fingerprint, enrollment operation ID, source (`imported_pin`, `local_first_use` or `explicit_repair`), evidence digest and timestamp. There is one active trust generation per live registration. `connection_bindings` references that generation as well as its profile/VM generations. Public trust exports live at `<state>/ssh/known-hosts/<registration-id>`; they are disposable publications of authoritative state, not alternate trust authorities. ELD-13 completes the relational constraints/publication schema.

Use a stable `HostKeyAlias` derived from the immutable registration ID, independent of source path, displayed SSH alias and TCP port. The known-hosts token must match that alias exactly. Port changes and project rebinding keep the pin. A different endpoint must present the same key, and any rebind must still pass its namespace/instance checks. Deleting a VM retires its trust record with its registration; recreating a project creates a new registration and enrollment.

For managed SSH use explicit `StrictHostKeyChecking=yes`, `NoHostAuthenticationForLocalhost=no`, the private `UserKnownHostsFile`, `GlobalKnownHostsFile=/dev/null`, `VerifyHostKeyDNS=no`, `UpdateHostKeys=no`, and the stable `HostKeyAlias`. Use the exact VM-login `IdentityFile`, `IdentityAgent=none`, `IdentitiesOnly=yes`, `AddKeysToAgent=no`, `ForwardAgent=no`, and no GSSAPI/host-based/password fallback. Restrict allowed server algorithms to the enrolled key type. User and Lima multiplexers cannot bypass the handshake; only the app's verified owned tunnel may persist. These option meanings and first-value precedence come from [OpenSSH configuration](https://man.openbsd.org/ssh_config).

Internal SSH calls use an owned configuration selected with `-F` and explicit arguments. The public alias contains the same trust constraints and the private proxy. When publishing it, inspect supported user configuration statically and detect conflicting earlier settings/includes; never execute `Match exec` while diagnosing. Dynamic user configuration whose effect cannot be determined is reported as unverified/conflicting rather than overwritten. Qualification can inspect `ssh -G` against controlled fixtures without executable matches; read-only doctor must not execute arbitrary host configuration commands to prove a check.

The proxy establishes/reuses only a generation-matching strictly verified identity tunnel, then relays the current registered loopback endpoint. The outer SSH client also verifies the pin. The public SSH/editor workflow verifies the owned alias/config before launch. Manually overriding verification on an arbitrary external SSH invocation is outside the managed-path guarantee.

Missing or changed trust/export prevents managed profile forwarding; absence is not automatic reenrollment. An unchanged authoritative pin with a missing export can be republished through an explicit connection repair. A changed host key requires a separate repair preview with a replacement public key/fingerprint obtained through an independent VM console or equivalent trusted path. Preserve the old pin and operation evidence, revoke owned old tunnels and publish a new generation only after that explicit repair is verified. `--yes`, restart and profile unlock cannot turn a mismatch into acceptance. Automatic `accept-new`, blind `ssh-keyscan` trust and deleting known-host entries on failure are not recovery policies.

Diagnostics read the existing pin and observe a bounded handshake with no enrollment, export write, master creation or guest side effect. Missing trust is its own unavailable check, not a failed Git key or a missing VM. Read-only checks against a stopped VM remain unavailable.

## Explicit application-port forwarding (ELD-05)

Lima has an automatic localhost-forwarding fallback. Its documented disabling rule includes a wildcard address, `guestIPMustBeZero: false` and both protocols. [Lima port-forwarding guidance](https://lima-vm.io/docs/config/port/#disabling-port-forwarding).

The effective native rule list is ordered as approved explicit mappings, then this owned final rule:

```yaml
portForwards:
  # Approved application mappings, if any, precede this rule.
  - guestIP: "0.0.0.0"
    guestIPMustBeZero: false
    proto: any
    guestPortRange: [1, 65535]
    ignore: true
```

An empty wrapper mapping list still emits this rule. All effective earlier rules must be classified and displayed: a broad allow, an unbounded range, non-loopback host binding, unexpected socket forward or overlay that defeats the final ignore is a conflict before creation/start/adoption. Exact duplicate/conflicting host endpoints are rejected. Native lists are not assumed to obey TOML's replacement semantics; the namespace boundary above eliminates global list contributions, and retained local bases must be checked after resolution.

Persistent explicit TCP and UDP mappings use the qualified GRPC forwarder; set `LIMA_SSH_PORT_FORWARDER=false` explicitly for managed starts. Host binds are literal loopback addresses only, IPv4 `127.0.0.1` or IPv6 `::1`; enumerate every actual host address/port/protocol in the preview. A rule's guest address and matching behavior must be retained, including wildcard/loopback equivalences verified against the tagged matcher. Do not call an IPv4-only host bind dual-stack. Qualification must exercise wildcard, loopback and concrete guest IPv4/IPv6 listeners for both protocols. An unsupported address/protocol request fails before effects, never falls back to automatic forwarding.

The CLI's temporary `forward` command remains an owned foreground TCP SSH tunnel with an explicit localhost port; UDP requires a persistent supported mapping rather than an invented SSH UDP tunnel. Exiting that foreground command closes only its tunnel. Persistent host endpoint/protocol tuples receive conflict reservations across managed VMs; the actual bind still needs verification because unrelated processes can claim a port. Failure to bind is a separate port-readiness failure, not permission to choose another host port silently.

The VM's loopback SSH port is a separate required transport, not an application mapping. Record and display its actual port; the catch-all does not disable it. Adopted profile socket forwarding is separately enumerated under its identity contract; TCP/UDP ignore rules do not remove Unix-socket forwards. Shipped configs disable Lima's ambient agent forwarding and use the owned T9 tunnel. Additional native network exposure outside automatic port forwarding must meet the existing custom-network boundary; this rule is not a guest firewall or a hostile-workload sandbox.

An adopted configuration already meeting this rule can be preserved byte-for-byte. An incompatible configuration is refused **before ownership retirement** with the exact offending rule; ordinary adoption does not silently rewrite it. Resolve it under the current owner's explicit configuration workflow and re-preview adoption. Environment updates retain and revalidate the suppression rule. Read-only inspection reports discrepancies but does not patch them.

## Planned files and qualification

Implement namespace manifests and effective rule validation in `internal/host/lima/config.go` and the version contract in `internal/host/lima/contract.go`; add `internal/host/lima/config_test.go`. Implement VM enrollment/export verification in `internal/connection/trust.go` with `trust_test.go`. Extend the planned lifecycle/adoption/recovery, connection and real-host contract tests. Preserve the prototype's useful catch-all test scenario; no production code is copied during design.

Required evidence before release:

- Introduce/change/delete only a namespace default/override while instance YAML stays unchanged. Create/start/resume/adopt refuse before dispatch and leave unrelated files/instances unchanged. Cover absence versus an empty file, unreadable/symlinked files and comment-only drift. Check creation-base drift independently from an existing start.
- Inspect all adapter file reads and effective SSH options for the exact supported Lima/OpenSSH build. No implicit global include, host-home key, personal agent, unsupported network input or raw-output persistence may bypass the declared contract.
- Enroll new and adopted VMs, including deferred enrollment for a stopped adoption. Change the port and rebind the source without changing trust. Substitute a wrong endpoint/key **after enrollment** and fail before profile forwarding. Initial enrollment tests explicitly retain the local-TOFU limitation.
- Interrupt between key observation, authoritative commit, export publication and first strict connection; resume against the same candidate/pin. Doctor never creates trust, writes an export or accepts a changed key. A user SSH override is reported without executing it.
- Start unlisted TCP/UDP listeners on IPv4/IPv6 loopback, wildcard and supported concrete guest addresses in two VMs, including after restart. Observe no unrequested host listeners. Verify each approved mapping, host-port collision handling, separate SSH transport, and both creation/adoption paths.

All checks are planned. The original review evidence and selected numerical compatibility target remain distinct from implementation qualification.

The complete relational additions and external publication/restore predicates are specified in [STATE_CONTRACT.md](STATE_CONTRACT.md). Exact public field and command spelling is specified in [INTERFACE_V1.md](INTERFACE_V1.md), approved on 2026-11-19; independent re-review pending.
