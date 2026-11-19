# Security reporting

Report vulnerabilities privately through the repository's **Security → Report a vulnerability** interface when available. If private reporting has not yet been enabled, ask the maintainer for a private contact route without posting exploit details, private keys, passphrases, or user logs in a public issue.

Include the easy-lima and Lima versions, host platform, affected boundary, and a minimal reproduction using disposable state. Remove credentials and identifying project data from evidence. The initial development release does not yet have a maintained stable-release support window.

## Relevant boundaries

- A selected project is writable in the guest; additional mounts are explicit.
- A guest with an active profile connection can request signatures from that profile's agent. Separate profiles are needed for separate trust domains.
- Private keys remain on the host. Remembered passphrases belong in the host credential store.
- Imported external agents and credential items cannot be administered or deleted implicitly.
- VM host-key changes, edited owned settings, ambiguous ownership, and unknown dispatch outcomes must fail closed.
- Configuration can contain intentional native provisioning scripts; only use templates you trust. Uncertain arbitrary scripts are never automatically replayed.
- Default operational logs omit known secret material. Opt-in raw output can still contain secrets printed by user software and should not be posted publicly.

Maintainers should enable private vulnerability reporting before the first public release.
