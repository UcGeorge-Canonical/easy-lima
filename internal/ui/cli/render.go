package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"unicode"

	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"github.com/UcGeorge-Canonical/easy-lima/internal/storage"
)

func Safe(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return '�'
		}
		return r
	}, s)
}
func Render(out io.Writer, result model.Result, jsonMode bool) error {
	if jsonMode {
		encoder := json.NewEncoder(out)
		encoder.SetEscapeHTML(false)
		return encoder.Encode(result)
	}
	switch data := result.Data.(type) {
	case model.MutationData:
		if result.Outcome == "planned" || data.Operation == nil && len(result.Errors) > 0 {
			RenderPlan(out, data.Plan)
		}
		if data.VM != nil {
			fmt.Fprintf(out, "VM: %s\nProject: %s\nSSH: ssh %s\n", data.VM.ID, Safe(data.VM.SourcePath), Safe(data.VM.SSHAlias))
		}
		if data.Profile != nil {
			profileText(out, *data.Profile)
		}
		if data.Operation != nil {
			fmt.Fprintf(out, "Operation: %s (%s)\n", data.Operation.ID, data.Operation.Status)
		}
		for _, m := range data.MemberResults {
			fmt.Fprintf(out, "%s  %s\n", m.VMID, m.Status)
			if m.OperationID != nil && m.Status == "pending" {
				fmt.Fprintf(out, "  Continue: easy-lima operations resume %s\n", *m.OperationID)
			}
		}
		if data.RecoveryRequired && data.Operation != nil {
			fmt.Fprintf(out, "Resume: easy-lima operations resume %s\n", data.Operation.ID)
		}
		if len(data.AvailableRepairs) > 0 {
			fmt.Fprintf(out, "Repairs: %s\n", strings.Join(data.AvailableRepairs, ", "))
		}
	case model.ListData[model.VM]:
		for _, invalid := range data.InvalidRecords {
			fmt.Fprintf(out, "Unavailable record: %s — %s\n", Safe(invalid.Source), Safe(invalid.Message))
		}
		w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tSTATE (recorded)\tPROJECT\tSSH ALIAS")
		for _, v := range data.Items {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", v.ID, v.State.Recorded, Safe(v.SourcePath), Safe(v.SSHAlias))
		}
		w.Flush()
		if data.NextCursor != nil {
			fmt.Fprintf(out, "Next page: --cursor %s\n", *data.NextCursor)
		}
	case model.ListData[model.Profile]:
		for _, invalid := range data.InvalidRecords {
			fmt.Fprintf(out, "Unavailable record: %s — %s\n", Safe(invalid.Source), Safe(invalid.Message))
		}
		for _, p := range data.Items {
			fmt.Fprintf(out, "%s  %s  %s  %d VM(s)\n", Safe(p.Label), p.ID, p.LockState, len(p.MemberVMIDs))
		}
		if data.NextCursor != nil {
			fmt.Fprintf(out, "Next page: --cursor %s\n", *data.NextCursor)
		}
	case model.ListData[model.Operation]:
		for _, invalid := range data.InvalidRecords {
			fmt.Fprintf(out, "Unavailable record: %s — %s\n", Safe(invalid.Source), Safe(invalid.Message))
		}
		for _, op := range data.Items {
			fmt.Fprintf(out, "%s  %-22s %s\n", op.ID, op.Kind, op.Status)
		}
		if data.NextCursor != nil {
			fmt.Fprintf(out, "Next page: --cursor %s\n", *data.NextCursor)
		}
	case map[string]any:
		if profile, ok := data["profile"].(model.Profile); ok {
			profileText(out, profile)
		} else {
			if plan, ok := data["plan"].(model.Plan); ok && (result.Outcome == "planned" || len(result.Errors) > 0) {
				RenderPlan(out, plan)
			}
			filtered := map[string]any{}
			for key, value := range data {
				if key != "plan" {
					filtered[key] = value
				}
			}
			b, _ := json.Marshal(filtered)
			var value any
			_ = json.Unmarshal(b, &value)
			renderValue(out, value, "")
		}
	}
	for _, c := range result.Checks {
		fmt.Fprintf(out, "%-11s %-27s %s\n", c.Status, c.Code, Safe(c.Message))
		if len(c.SuggestedCommand) > 0 {
			fmt.Fprintf(out, "  %s\n", Safe(strings.Join(c.SuggestedCommand, " ")))
		} else if c.NextAction != "none" && c.NextAction != "" {
			fmt.Fprintf(out, "  Next: %s\n", strings.ReplaceAll(c.NextAction, "_", " "))
		}
	}
	for _, w := range result.Warnings {
		fmt.Fprintf(out, "Warning [%s]: %s\n", w.Code, Safe(w.Message))
	}
	for _, e := range result.Errors {
		fmt.Fprintf(out, "%s: %s\n", e.Code, Safe(e.Message))
		if len(e.SuggestedCommand) > 0 {
			fmt.Fprintln(out, strings.Join(e.SuggestedCommand, " "))
		}
	}
	if len(result.Errors) == 0 && len(result.Checks) == 0 {
		fmt.Fprintf(out, "%s\n", result.Outcome)
	}
	return nil
}
func RenderPlan(out io.Writer, p model.Plan) {
	if len(p.Effects) == 0 {
		return
	}
	fmt.Fprintf(out, "%s\n", strings.ReplaceAll(p.Action, ".", " "))
	if p.Target.Path != nil {
		fmt.Fprintf(out, "  Project: %s\n", Safe(*p.Target.Path))
	}
	if p.Target.ID != nil {
		fmt.Fprintf(out, "  %s ID: %s\n", p.Target.Kind, *p.Target.ID)
	}
	if len(p.AffectedVMIDs) > 0 {
		fmt.Fprintf(out, "  Affected VMs: %s\n", strings.Join(p.AffectedVMIDs, ", "))
	}
	for _, f := range p.Fields {
		if f.Path == "storage.creation_ledger" {
			var ledger storage.Ledger
			encoded, err := json.Marshal(f.Value)
			if err == nil && json.Unmarshal(encoded, &ledger) == nil {
				fmt.Fprintln(out, "  Storage destinations:")
				for _, d := range ledger.Destinations {
					fmt.Fprintf(out, "    %s: %s\n", Safe(d.Purpose), Safe(d.Path))
				}
				for _, g := range ledger.Groups {
					fmt.Fprintf(out, "    Shared capacity: %.1f GiB required, %.1f GiB available, %.1f GiB pending\n", float64(g.RequiredBytes)/(1<<30), float64(g.AvailableBytes)/(1<<30), float64(g.PendingBytes)/(1<<30))
				}
				continue
			}
		}
		fmt.Fprintf(out, "  %s: %v\n", f.Path, Safe(fmt.Sprint(f.Value)))
	}
	for i, e := range p.Effects {
		fmt.Fprintf(out, "  %d. %s\n", i+1, Safe(e.Description))
	}
	if p.Target.Kind == "vm" || p.Target.Kind == "storage" {
		fmt.Fprintln(out, "Future VM disk growth is not reserved.")
	}
}
func profileText(out io.Writer, p model.Profile) {
	fmt.Fprintf(out, "Profile: %s (%s)\nGit: %s <%s>\nAgent: %s · %s\n", Safe(p.Label), p.ID, Safe(p.Git.Name), Safe(p.Git.Email), p.AgentManagement, p.LockState)
	if p.GitHubUsername != nil {
		fmt.Fprintf(out, "Expected GitHub account: %s\n", Safe(*p.GitHubUsername))
	}
	fmt.Fprintf(out, "Capabilities: %s\nRemember passphrase: %t\n", strings.Join(p.Capabilities, ", "), p.RememberEnabled)
	if p.Fingerprint != nil {
		fmt.Fprintf(out, "Fingerprint: %s\n", *p.Fingerprint)
	}
	if p.PublicKey != nil {
		fmt.Fprintf(out, "Public key:\n%s\n\nAdd this public key to your intended GitHub account twice: as an Authentication Key and as a Signing Key.\nhttps://github.com/settings/keys\nThen verify with: easy-lima profile check %s --online\n", Safe(*p.PublicKey), Safe(p.Label))
	}
}

func renderValue(out io.Writer, value any, indent string) {
	switch v := value.(type) {
	case map[string]any:
		keys := []string{}
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			label := strings.ReplaceAll(k, "_", " ")
			switch child := v[k].(type) {
			case map[string]any, []any:
				fmt.Fprintf(out, "%s%s:\n", indent, label)
				renderValue(out, child, indent+"  ")
			case nil:
				fmt.Fprintf(out, "%s%s: not observed\n", indent, label)
			default:
				fmt.Fprintf(out, "%s%s: %s\n", indent, label, Safe(fmt.Sprint(child)))
			}
		}
	case []any:
		if len(v) == 0 {
			fmt.Fprintf(out, "%s(none)\n", indent)
		}
		for _, item := range v {
			switch item.(type) {
			case map[string]any, []any:
				renderValue(out, item, indent+"  ")
			default:
				fmt.Fprintf(out, "%s• %s\n", indent, Safe(fmt.Sprint(item)))
			}
		}
	default:
		fmt.Fprintf(out, "%s%s\n", indent, Safe(fmt.Sprint(v)))
	}
}
