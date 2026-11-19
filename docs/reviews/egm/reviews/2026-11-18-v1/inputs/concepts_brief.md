# easy-lima

- **Author:** George Uche-Umeh
- **Updated:** 2026-11-18
- **Status:** V1 scope approved by the author on 2026-11-18. EGM Goldfish review round 1 is in progress; implementation design has not started.

## Overview

easy-lima is a project-oriented wrapper around Lima. A developer selects a folder, starts its VM, and connects through SSH or VS Code. The wrapper supplies project browsing, explicit storage choices, selected Git identities, and a consistent CLI around Lima's mechanisms.

The author prefers a straightforward Lima-derived name and has proposed `easy-lima`. The earlier independent-product naming direction is superseded. Lima is the runtime; a generic layer over competing VM providers is outside the chosen direction.

## Analysis

The existing helper establishes the core single-Mac workflow: nested folder selection, persistent Ubuntu VMs, development tooling, SSH/editor access, external-volume safeguards, and one work identity with signing. Public reuse requires replacing installation-specific assumptions with explicit user configuration and tested behavior.

The important additions for v1 are a usable installer, reliable instance ownership and project rebinding, named identity profiles, configuration boundaries, diagnosis/recovery, and adoption of existing environments. Storage safety and identity separation are essential to the original use case; they should remain in the first release.

The principal scope risk is growing a wrapper into a second environment platform. Reuse Lima YAML, templates, provisioning, and lifecycle operations. More integrations and persistent-data operations should follow after the basic contract is reliable.

## Proposed Feature Set

The detailed source of truth is [FEATURES.md](/Volumes/Nebula/dev/Canonical/local/FEATURES.md). It defines F1–F9 with release criteria, first-release limits, a platform proposal, later feature groups, and end-to-end acceptance checks.

The proposed v1 covers installation/setup; nested project selection and CLI parity; persistent VM lifecycle and stable project associations; native Lima configuration and limited Ubuntu presets; storage/resource visibility; SSH, VS Code, shared files, and explicit ports; named Git identities; diagnosis/recovery; and a public release with prototype adoption.

macOS and Linux are the approved first-release targets, with an explicit tested compatibility matrix still to be defined. Windows/WSL is deferred.

## Goals vs. Non-Goals

**Goals:** Other developers can install the tool without machine-specific edits, use a complete project VM workflow, choose storage deliberately, keep identities separate, and understand failures. Existing instances and credentials survive adoption.

**Later proposals:** Automatic storage relocation, snapshot/clone controls, multiple environments per project, branch/worktree workflows, more editors, Workshop integration, and Windows support.

**Outside the chosen direction:** An independent VM engine, competing-provider abstraction, SDK marketplace, and hosted cloud platform.

## Alternatives Weighed

The author narrowed the earlier broad environment-manager idea to a Lima wrapper and rejected clever independent naming. A direct name such as easy-lima reflects that boundary.

Workshop remains a possible guest integration rather than the foundation of v1. Lima's existing formats and operations should be evaluated before adding wrapper-owned mechanisms. No implementation language, license, or specific packaging route has been selected.

## Key Decisions and Next Steps

Confirmed direction: Lima wrapper, simple Lima-derived working name, and a deliberate separation between v1 and later features.

Approved v1 scope: F1–F9, macOS plus Linux as initial hosts, Ubuntu as the tested guest, one VM per project, and delaying automatic storage moves and Workshop integration. Exact versions and implementation mechanisms remain open for design.

The author has explicitly requested fresh-context EGM Goldfish review before implementation design. Record findings and proposed amendments without silently changing the approved scope. This brainstorming work has not changed the helper or its running environments.
