// Generate the command index from the same grammar the CLI parses.
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/UcGeorge-Canonical/easy-lima/internal/ui/cli"
)

func main() {
	var b strings.Builder
	b.WriteString("# Command reference\n\n[Documentation home](../README.md) · [Everyday examples](../usage.md) · [TOML reference](toml.md)\n\nRun `easy-lima` without a command to open the browser in the current directory. An existing directory as the first argument changes the browsing root. Shared browser inputs are `--config`, `--project-config`, and `--limactl`.\n\nRun any command with `--help` for its exact grammar. `PATH` defaults to the original CWD. Select by exact folder or `--vm FULL_VM_ID`, never both. Operation and VM IDs are full immutable 32-character hex identifiers.\n\n## Shared flags\n\n| Flag | Behavior |\n| --- | --- |\n| `--config FILE` | Existing user TOML input; keeps the same registry. |\n| `--project-config FILE` | Existing project TOML input. |\n| `--limactl FILE` | Explicit installed Lima CLI override, version checked. |\n| `--json` | One result object for metadata commands; noninteractive. Not accepted by raw terminal commands. |\n| `--dry-run` | Preview a mutation without applying it. |\n| `--yes` | Accept a concrete effect plan; does not bypass explicit profile lock, key authentication, or delete ID confirmation. |\n| `--capture-output` | Opt in to saving raw command output, with known secret patterns excluded. Default operational logs contain no raw output. |\n\nFlags can precede or follow the command, before `--`. Scalar flags cannot repeat. Booleans accept `true` or `false`; a bare boolean means true. `--vm` repeats only for profile apply/assign/unassign; `--repository` repeats for doctor.\n\n## Commands\n\n| Syntax | Specific flags |\n| --- | --- |\n")
	names := []string{}
	for name := range cli.Specs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		s := cli.Specs[name]
		flags := strings.Fields(s.Flags)
		if map[string]bool{"up": true, "start": true, "restart": true, "code": true, "ssh": true, "exec": true, "forward": true, "environment.update": true, "rebind": true, "repair": true, "adopt": true}[name] {
			flags = append(flags, "passphrase-fd")
		}
		for i := range flags {
			flags[i] = "`--" + flags[i] + "`"
		}
		fmt.Fprintf(&b, "| `easy-lima %s` | %s |\n", strings.ReplaceAll(s.Usage, "|", "\\|"), strings.Join(flags, ", "))
	}
	b.WriteString("\n`--presets` is a comma-separated list. `--mounts-from` and `--ports-from` read schema-versioned TOML arrays. CPU is a positive integer; memory and disk use `MiB`, `GiB` or `TiB`. `--repository` takes an absolute **guest** directory. `--passphrase-fd` is an inherited descriptor of at least 3; never pass a secret as a command argument or environment variable.\n\n`environment update` and `repair` preview by default; add `--apply` to request execution. `profile edit` changes desired TOML; `profile apply` changes the effective profile. `rebind` uses `--vm` and `--to` and preserves disk/alias. Adoption accepts only the supported prototype under the same storage root. See [recovery](../recovery.md).\n\n`ssh`, `exec`, `forward`, and `code` do not support `--json`. `exec` requires a literal `--` followed by the guest command; other commands use `--` to end option parsing for paths such as `./-project`. `logs --follow` streams until cancelled and is incompatible with JSON. Lists accept `--limit` from 1 to 1000 and return `next_cursor` when another page exists; cursors expire when registry generation changes.\n\n## Exit statuses\n\n| Code | Meaning |\n| --- | --- |\n| 0 | Completed, or a successful preview. |\n| 2 | Invalid input or configuration. |\n| 3 | Missing or unsupported dependency. |\n| 4 | Decision, ownership, settings or binding conflict. |\n| 5 | Unlock or credential interaction required. |\n| 6 | Partial, unknown, unavailable state, or unverified provider result. |\n| 7 | A check or operation failed. |\n| 8 | Requested target not found. |\n| 9 | Storage, resource or port availability prevents the request. |\n| 130 | Cancelled. Already dispatched effects can still need reconciliation. |\n| 255 | `exec` could not establish the guest exit status. |\n\nWhen `exec` obtains a guest exit status, it returns that exact status. Read JSON `errors`, `checks`, `operation_id` and each `next_action` rather than inferring a cause from an exit number alone.\n")
	if err := os.WriteFile("docs/reference/commands.md", []byte(b.String()), 0644); err != nil {
		panic(err)
	}
}
