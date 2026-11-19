# Git profiles

A profile combines an explicit Git author identity, an SSH key, signing preferences, and an optional expected GitHub account. It is independent of VM startup and can be shared by any number of VMs. A VM has one profile or none.

The application never takes your author name, email, key, or account from the host's personal Git configuration. Parent/child project bindings with different profiles are rejected, including a profile versus no profile. This prevents one mounted project tree from silently mixing identities.

## Create or import once

```sh
easy-lima profile create work \
  --git-name "Example Developer" \
  --git-email "developer@example.com" \
  --github-user ExampleWork \
  --remember
```

Generated keys are encrypted Ed25519 keys. Interactive creation asks for a passphrase without echoing it. Remembering is optional and off unless requested. The private key is kept on the host; guest VMs receive public identity files and a connection to the profile's isolated agent.

To use an existing host key:

```sh
easy-lima profile import work \
  --key /private/path/work-key \
  --git-name "Example Developer" \
  --git-email "developer@example.com" \
  --github-user ExampleWork
```

Supported software keys are Ed25519, RSA of at least 2048 bits using SHA-2 signatures, and ECDSA P-256/P-384/P-521. Hardware keys and SSH certificates are outside v1. Imported unencrypted keys remain unencrypted; the application never strips encryption to make reuse easier.

## Finish GitHub registration

1. Run `easy-lima profile show work` and copy the printed **public** key.
2. Sign into the intended GitHub account and open [SSH and GPG keys](https://github.com/settings/keys).
3. Add that key once as an **Authentication Key**, then again as a **Signing Key**. GitHub requires two registrations when one key serves both purposes; see its [key registration guide](https://docs.github.com/en/authentication/connecting-to-github-with-ssh/adding-a-new-ssh-key-to-your-github-account).
4. Run `easy-lima profile check work --online`. It checks the selected agent, signing, provider authentication, and the expected account separately.

Account checks require network access. An unreachable provider remains unverified; it is not reported as a wrong account. Account matching is case-insensitive. Register a verified email with your provider as needed for its commit verification display. The application does not change account settings or upload keys for you.

## Everyday use

```sh
easy-lima up ./service --profile work
easy-lima up ./another-service --profile work
easy-lima code ./service
```

A loaded profile key is reused immediately. If it is not loaded and remembering is enabled, easy-lima retrieves the credential from macOS Keychain or Linux Secret Service. Necessary OS consent is handled interactively; noninteractive commands never open hidden dialogs.

Git's author settings and SSH signing are applied before identity setup is considered ready. Commits and tags are signed by default. Existing GitHub HTTPS remotes use a guest Git `insteadOf` mapping to SSH; repository remotes and host Git settings are not rewritten. Managed settings use dedicated included files. Conflicting user edits are reported instead of overwritten.

The selected guest can request signatures from its forwarded profile agent while connected. Use separate profiles for trust boundaries as well as account separation. Project files mounted into the VM are accessible to it.

## Lock, forget, edit, and reassign

```sh
easy-lima profile lock work
easy-lima profile unlock work
easy-lima profile forget work
easy-lima profile edit work --git-email new-address@example.com
easy-lima profile apply work
easy-lima profile assign work --vm VM_ID
easy-lima profile unassign --vm VM_ID
```

Lock revokes the profile's managed forwarding, removes the loaded key, and inhibits automatic reload across its VMs. Unlock explicitly releases that inhibition. Forget disables remembering and removes only an easy-lima-owned saved passphrase; it leaves the key file and currently loaded key alone.

Edit changes desired profile configuration. Apply previews and commits the change. Git-only changes may target selected members using repeatable `--vm ID`. Key or agent changes must include every member VM. Stopped VMs stay stopped and receive an authorized pending identity update; the next explicit start/open finishes it before Git access. No guest tools are upgraded as part of identity changes.

`profile remove` requires the profile to be unused and free of an active transition. It retires managed agent state while preserving imported private keys and history. An adopted external agent remains externally managed until `profile migrate-agent LABEL` explicitly migrates all members to an owned agent with the same key. Other users of the external agent are preserved.

## Automation and secret input

Metadata commands support `--json`. Use `--yes` to accept a resolved mutation plan in automation. It does not unlock an explicitly locked profile or supply missing identity fields.

Passphrases use `--passphrase-fd N`, an inherited readable descriptor numbered 3 or above. Arrange the descriptor through your secret manager or process supervisor. Do not put a secret value in arguments, environment variables, TOML, or shell history. The command reads at most 64 KiB and strips one trailing newline. Generated keys require a nonempty passphrase. `--capture-output` excludes credential interactions.

The application does not create a new key for each VM, repeatedly ask whether to remember an existing profile, or silently replace a missing external agent.
