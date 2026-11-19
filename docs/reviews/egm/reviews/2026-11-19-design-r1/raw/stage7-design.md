Stage 7 Design Critic — REVIEW

Result: **0 Blockers, 4 Major findings, 0 Nits.** The draft satisfies the standard’s broad coverage of motivation, architecture, alternatives, and constraints. The following integration and ownership gaps need concrete design corrections. These are static findings; no qualification tests were executed.

All scopes below refer to the frozen [design_draft.md](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md).

**S7-DESIGN-01 — Major — Freeze Lima’s namespace-wide configuration as an input to every managed start**

Affected scope: [lines 267–269](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:267), 373–375, 957–965, and 987–1005.

The design freezes each environment’s template and provisioning inputs, controls the subprocess environment, and checks the recorded instance configuration before starting. It does not define how `$LIMA_HOME/_config/default.yaml`, `override.yaml`, and creation-time `base.yaml` participate in that frozen input boundary.

This is a concrete integration issue. Lima’s tagged template documents that `default.yaml` and `override.yaml` are reapplied on every start; override values can supersede instance configuration, and lists such as mounts and provisioning are combined. The tagged loader reads these files independently of the instance YAML. Controlling environment variables or materializing an instance template therefore does not neutralize them. [Lima v2.2.1 global configuration semantics](https://github.com/lima-vm/lima/blob/v2.2.1/templates/default.yaml#L625), [configuration loader](https://github.com/lima-vm/lima/blob/v2.2.1/pkg/limayaml/load.go#L31).

Failure scenario: after successful creation or adoption, a user changes a shared namespace override between operations. The next ordinary start finds unchanged owned instance YAML but boots with an additional mount or provisioning script. That can violate the no-unsolicited-upgrade, approved-mount, and frozen-retry contracts before a post-start comparison notices the difference. This scenario does not require a concurrent external mutation.

Recommendation: define an explicit policy for namespace configuration. Either reject unsupported global configuration without modifying it, or include the effective global files, their absence, relevant dependencies, and merge semantics in the retained input revision. Check them before create/start/update and refuse unreviewed drift. Explain how this works when the root also contains unrelated or remaining prototype instances. Add qualification cases that change only global configuration while leaving the instance YAML intact. The general instruction to prevent ambient settings from changing the target, and the later configuration-merging test gate, do not yet specify this enforcement boundary.

**S7-DESIGN-02 — Major — Specify VM host-key enrollment and effective SSH verification**

Affected scope: [lines 155–157](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:155), 904–914, and 951–953.

T9 requires the dedicated tunnel to use a verified VM host key and says the ordinary SSH/VS Code client performs host-key verification. The design supplies an enrollment mechanism for GitHub host keys, but none for the VM’s host key.

Lima 2.2.1’s common SSH options contain `StrictHostKeyChecking=no`, `UserKnownHostsFile=/dev/null`, and `NoHostAuthenticationForLocalhost=yes`. Its host agent uses those options when generating the instance’s SSH configuration. Retaining that configuration does not supply the verification asserted by T9. [Lima SSH options](https://github.com/lima-vm/lima/blob/v2.2.1/pkg/sshutil/sshutil.go#L375), [generated configuration path](https://github.com/lima-vm/lima/blob/v2.2.1/pkg/hostagent/hostagent.go#L198).

Failure scenario: an implementation preserves Lima’s generated settings and establishes the profile-agent reverse tunnel to a reachable loopback endpoint without authenticating that endpoint as the recorded VM. Alternatively, merely enabling strict checking leaves a fresh VM without an enrolled key and breaks the noninteractive connection path. OpenSSH requires an established trust entry for that strict behavior. [OpenSSH host-key checking](https://man.openbsd.org/ssh_config#StrictHostKeyChecking).

Recommendation: specify the initial trust source, when enrollment occurs, the per-registration host-key record/known-hosts location, and behavior for adoption, port changes, rebind, and unexpected key replacement. Explicitly override the three Lima defaults on every managed transport and alias where verification is promised. If initial trust relies on trust-on-first-use, state that policy rather than calling it independently verified. Add effective-configuration checks and a changed-key/wrong-endpoint test that fails before exposing the profile socket. Provider known-hosts handling does not resolve VM transport trust.

**S7-DESIGN-03 — Major — Define lock behavior for externally managed adopted agents**

Affected scope: [lines 812–818](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:812), 857–872, 906–918, and 1037. Related approved experience: `UX_PROPOSAL.md`, lines 44–46.

The explicit lock contract unloads the profile key and inhibits automatic reload across all its VMs. Adoption introduces another resource class: an agent shared with non-adopted projects that easy-lima may use but must not stop, reconfigure, or take exclusive control of. The design does not reconcile these rules.

Failure scenario: adopt one VM while another prototype VM still uses the same legacy agent. The user locks the imported profile. Removing the key from that shared agent disrupts the remaining prototype VM, contrary to the preservation contract. Recording inhibition without removing the key leaves an existing forwarding connection able to use it. Blocking future alias preparation alone does not revoke an already established forwarding path.

This follows from the draft’s own shared-agent and persistent-tunnel architecture; it is not a claim about the current live environment. The prototype source confirms that its managed aliases share the configured agent socket in `local.py`, lines 317–328.

Recommendation: add a capability and mutation policy for externally managed profiles covering lock, unlock/activation, apply, forget, and removal. Choose a concrete, truthful outcome for lock: for example, refuse it before effects until an explicit ownership migration, or define and implement a narrower revocation mechanism covering every relevant forwarding path. Do not report a full profile lock while access remains available, and do not unload a key used by resources outside the approved scope. Any user-visible limitation or altered lock meaning must be surfaced against the already approved UX. Validate with one adopted and one non-adopted VM sharing the legacy identity.

**S7-DESIGN-04 — Major — Disable Lima’s implicit port-forwarding fallback**

Affected scope: [lines 45–46](/Volumes/Nebula/dev/Canonical/local/github/ucgeorge-canonical/easy-lima/.egm/reviews/2026-11-19-design-r1/inputs/design_draft.md:45), 513–521, 904, and 1125. Related approved experience: `UX_PROPOSAL.md`, lines 36 and 64.

The design promises explicit localhost exposure, describes wrapper port-list precedence, and retains native Lima port forwarding. It does not establish the native rule that prevents unrequested ports from being forwarded.

Lima automatically forwards guest localhost ports. Its documented disabling mechanism is a catch-all ignore rule; since Lima 2.0, the wildcard guest-address rule also needs `guestIPMustBeZero=false` to cover loopback listeners. An empty wrapper port list alone does not disable the native fallback. [Lima port-forwarding documentation](https://lima-vm.io/docs/config/port/#disabling-port-forwarding). The prototype already handles this explicitly in `local.py`, lines 200–202.

Failure scenario: two newly created VMs have no requested mappings and each later starts a development server on guest port 8080. Native automatic forwarding can expose one at host port 8080, while the other encounters a collision. The preview advertised no such mapping, and the host endpoint can serve the wrong project from the user’s perspective.

Recommendation: make suppression of implicit forwarding an owned native-configuration invariant. Define ordering for approved mappings followed by a catch-all ignore rule, including IPv4/IPv6, TCP/UDP, and the unavoidable VM SSH transport exception. Apply the check to effective custom/adopted configuration, rejecting or explicitly resolving incompatible behavior. Add tests that start previously unlisted guest listeners after creation and verify that no new host application port appears. This is a known default requiring a design rule, rather than merely an unexecuted compatibility claim.

**Read acknowledgment and limitations**

I read completely:

- The supplied `agents/design_critic.md` persona and `feature-spec-standard.md`.
- Frozen `design_draft.md`, active lines **1–1129**, plus **1130–1239** for implementation details, mitigations, and traceability.
- Frozen `FEATURES.md` (208 lines), `concepts_brief.md` (83), `UX_PROPOSAL.md` (103), and `goldfish_review_report.md` (305). The last was treated only as closed historical concept-review context.

Actual local code evidence consulted: `/Volumes/Nebula/dev/Canonical/local/.local-launcher/local.py`, lines **1–170 and 200–550**, covering configuration, ownership, locking, SSH publication, lifecycle, and forwarding. No test files were consulted or executed.

Primary upstream evidence consulted: Lima v2.2.1 `templates/default.yaml`, `pkg/limayaml/load.go`, relevant portions of `pkg/sshutil/sshutil.go` and `pkg/hostagent/hostagent.go`; Lima port-forwarding documentation; and the OpenSSH `ssh_config` manual. Source behavior is distinguished above from inferred application failure scenarios.

No files were written. No builds, tests, VM commands, installations, live VM/state inspection, credentials, keys, credential stores, host SSH configuration, conversation/history/memory services, unrelated repositories, current-stage reviewer reports, or subagents were accessed. The selected versions and host matrix remain qualification targets; this review does not certify or disprove runtime compatibility.
