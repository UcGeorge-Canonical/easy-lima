// Package model contains the versioned public results and shared value types.
package model

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"
)

var Version = "dev"

const SchemaVersion = 1

var idPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func ValidID(s string) bool { return idPattern.MatchString(s) }
func Now() string           { return time.Now().UTC().Format(time.RFC3339Nano) }
func Ptr[T any](v T) *T     { return &v }

type Scope struct {
	Kind string  `json:"kind"`
	ID   *string `json:"id"`
	Path *string `json:"path"`
}

func Global() Scope                 { return Scope{Kind: "global"} }
func VMScope(id, path string) Scope { return Scope{Kind: "vm", ID: Ptr(id), Path: Ptr(path)} }

type Issue struct {
	Code             string   `json:"code"`
	Scope            Scope    `json:"scope"`
	Message          string   `json:"message"`
	NextAction       string   `json:"next_action"`
	SuggestedCommand []string `json:"suggested_command"`
}
type Error struct {
	Issue
	Exit int
}

func (e *Error) Error() string { return e.Message }
func Fail(code, message string) error {
	return &Error{Issue: Issue{Code: code, Scope: Global(), Message: message, NextAction: NextAction(code)}, Exit: ExitFor(code)}
}
func AsError(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	// Raw adapter errors may contain arguments or output; never render them.
	return &Error{Issue: Issue{Code: "internal.error", Scope: Global(), Message: "The operation could not be completed. Inspect its recorded evidence.", NextAction: "inspect"}, Exit: 7}
}
func ExitFor(code string) int {
	switch code {
	case "input.invalid", "config.invalid", "config.unsupported_schema", "config.unknown_field", "secret.input_invalid":
		return 2
	case "dependency.missing", "dependency.unsupported", "lima.output_unsupported", "editor.unavailable":
		return 3
	case "connection.repair_required", "decision.required", "operation.busy", "ownership.conflict", "binding.conflict", "config.drift", "settings.edited", "profile.scope_conflict", "profile.external_managed", "profile.binding_required", "trust.mismatch", "state.backup_stale":
		return 4
	case "profile.unlock_required", "profile.explicitly_locked", "credential.interaction_required":
		return 5
	case "operation.unknown", "operation.partial", "state.unavailable", "state.publication_pending", "inventory.stale", "record.invalid", "check.incomplete", "provider.unverified", "output.truncated":
		return 6
	case "target.not_found":
		return 8
	case "storage.unavailable", "storage.insufficient", "resource.unsupported", "port.unavailable":
		return 9
	case "operation.cancelled":
		return 130
	default:
		return 7
	}
}
func NextAction(code string) string {
	switch code {
	case "profile.unlock_required", "profile.explicitly_locked", "credential.interaction_required":
		return "unlock_profile"
	case "dependency.missing", "dependency.unsupported":
		return "install_dependency"
	case "storage.unavailable":
		return "connect_storage"
	case "operation.unknown", "operation.partial", "operation.busy":
		return "resume_operation"
	case "state.unavailable", "state.publication_pending", "state.backup_stale":
		return "state_recover"
	case "config.invalid", "config.drift", "config.unknown_field", "config.unsupported_schema":
		return "resolve_config"
	case "connection.repair_required", "settings.edited", "trust.mismatch":
		return "repair"
	case "profile.external_managed":
		return "migrate_agent"
	default:
		return "inspect"
	}
}

type Check struct {
	Code             string   `json:"code"`
	Status           string   `json:"status"`
	Scope            Scope    `json:"scope"`
	CheckedAt        string   `json:"checked_at"`
	EvidenceTime     *string  `json:"evidence_time"`
	Source           string   `json:"source"`
	Message          string   `json:"message"`
	NextAction       string   `json:"next_action"`
	SuggestedCommand []string `json:"suggested_command"`
}

func NewCheck(code, status, message string, scope Scope) Check {
	return Check{Code: code, Status: status, Message: message, Scope: scope, CheckedAt: Now(), Source: "observed", NextAction: "none"}
}

type Result struct {
	SchemaVersion int     `json:"schema_version"`
	Command       string  `json:"command"`
	RequestID     string  `json:"request_id"`
	OperationID   *string `json:"operation_id"`
	Outcome       string  `json:"outcome"`
	ExitCode      int     `json:"exit_code"`
	Data          any     `json:"data"`
	Checks        []Check `json:"checks"`
	Warnings      []Issue `json:"warnings"`
	Errors        []Issue `json:"errors"`
}

func NewResult(command string) Result {
	return Result{SchemaVersion: 1, Command: command, RequestID: NewID(), Outcome: "completed", Checks: []Check{}, Warnings: []Issue{}, Errors: []Issue{}}
}
func (r *Result) SetError(err error) {
	e := AsError(err)
	r.Errors = append(r.Errors, e.Issue)
	r.ExitCode = e.Exit
	r.Outcome = "failed"
	if e.Exit == 6 {
		r.Outcome = "partial"
	}
	if e.Exit == 4 || e.Exit == 5 || e.Exit == 9 || e.Exit == 3 || e.Exit == 8 {
		r.Outcome = "blocked"
	}
	if e.Exit == 130 {
		r.Outcome = "cancelled"
	}
}
func (r *Result) Diagnostic(scope Scope) {
	counts := map[string]int{"passed": 0, "failed": 0, "unavailable": 0, "skipped": 0, "unknown": 0, "unverified": 0}
	for _, c := range r.Checks {
		counts[c.Status]++
	}
	r.Data = map[string]any{"target": scope, "summary": counts}
	if counts["failed"] > 0 {
		r.ExitCode = 7
		r.Outcome = "failed"
	} else if counts["unavailable"]+counts["unknown"]+counts["unverified"] > 0 {
		r.ExitCode = 6
		r.Outcome = "partial"
	}
}

type Resources struct {
	CPUs         *int   `json:"cpus"`
	MemoryBytes  *int64 `json:"memory_bytes"`
	DiskMaxBytes *int64 `json:"disk_max_bytes"`
}
type Storage struct {
	Root                 string  `json:"root"`
	InstanceDirectory    string  `json:"instance_directory"`
	AllocatedBytes       *int64  `json:"allocated_bytes"`
	MeasurementTime      *string `json:"measurement_time"`
	FutureGrowthReserved bool    `json:"future_growth_reserved"`
}
type VMState struct {
	Recorded   string  `json:"recorded"`
	Observed   *string `json:"observed"`
	ObservedAt *string `json:"observed_at"`
}
type VM struct {
	ID                           string    `json:"id"`
	SourcePath                   string    `json:"source_path"`
	ProposedSourcePath           *string   `json:"proposed_source_path"`
	InstanceName                 string    `json:"instance_name"`
	SSHAlias                     string    `json:"ssh_alias"`
	NamespaceID                  string    `json:"namespace_id"`
	ProfileID                    *string   `json:"profile_id"`
	AppliedEnvironmentRevisionID *string   `json:"applied_environment_revision_id"`
	AppliedIdentityRevisionID    *string   `json:"applied_identity_revision_id"`
	PendingIdentityRevisionID    *string   `json:"pending_identity_revision_id"`
	Ownership                    string    `json:"ownership"`
	State                        VMState   `json:"vm_state"`
	Resources                    Resources `json:"resources"`
	Storage                      Storage   `json:"storage"`
	OperationIDs                 []string  `json:"operation_ids"`
}
type GitIdentity struct {
	Name        string `json:"name" toml:"name"`
	Email       string `json:"email" toml:"email"`
	SignCommits bool   `json:"sign_commits" toml:"sign_commits"`
	SignTags    bool   `json:"sign_tags" toml:"sign_tags"`
}
type Profile struct {
	ID                  string      `json:"id"`
	Label               string      `json:"label"`
	Git                 GitIdentity `json:"git"`
	GitHubUsername      *string     `json:"github_username"`
	PublicKey           *string     `json:"public_key"`
	Fingerprint         *string     `json:"fingerprint"`
	AgentManagement     string      `json:"agent_management"`
	LockState           string      `json:"lock_state"`
	RememberEnabled     bool        `json:"remember_enabled"`
	Capabilities        []string    `json:"capabilities"`
	MemberVMIDs         []string    `json:"member_vm_ids"`
	PendingOperationIDs []string    `json:"pending_operation_ids"`
}
type Operation struct {
	ID                 string  `json:"id"`
	Kind               string  `json:"kind"`
	Status             string  `json:"status"`
	Scope              Scope   `json:"scope"`
	ParentID           *string `json:"parent_operation_id"`
	RetryOfID          *string `json:"retry_of_operation_id"`
	LastConfirmedStage *string `json:"last_confirmed_stage"`
	RecoveryRequired   bool    `json:"recovery_required"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
	LogPath            *string `json:"log_path"`
}

func Terminal(status string) bool {
	return status == "completed" || status == "failed_known" || status == "cancelled_before_dispatch"
}

type Effect struct {
	Kind            string `json:"kind"`
	Scope           Scope  `json:"scope"`
	Description     string `json:"description"`
	RequiresRestart bool   `json:"requires_restart"`
	Destructive     bool   `json:"destructive"`
}
type Field struct {
	Path       string  `json:"path"`
	Value      any     `json:"value"`
	Origin     string  `json:"origin"`
	SourcePath *string `json:"source_path"`
	Changed    bool    `json:"changed"`
}
type Plan struct {
	Action                string   `json:"action"`
	Target                Scope    `json:"target"`
	Complete              bool     `json:"complete"`
	Effects               []Effect `json:"effects"`
	Fields                []Field  `json:"fields"`
	RequiredConfirmations []string `json:"required_confirmations"`
	AffectedVMIDs         []string `json:"affected_vm_ids"`
	UnresolvedCodes       []string `json:"unresolved_codes"`
	FutureGrowthReserved  bool     `json:"future_growth_reserved"`
}

func NewPlan(action string, target Scope) Plan {
	return Plan{Action: action, Target: target, Complete: true, Effects: []Effect{}, Fields: []Field{}, RequiredConfirmations: []string{}, AffectedVMIDs: []string{}, UnresolvedCodes: []string{}}
}
func (p *Plan) Add(kind, description string) {
	p.Effects = append(p.Effects, Effect{Kind: kind, Scope: p.Target, Description: description})
}

type MemberResult struct {
	VMID        string  `json:"vm_id"`
	Status      string  `json:"status"`
	OperationID *string `json:"operation_id"`
}
type MutationData struct {
	Checks           []Check        `json:"-"`
	Plan             Plan           `json:"plan"`
	Operation        *Operation     `json:"operation"`
	VM               *VM            `json:"vm"`
	Profile          *Profile       `json:"profile"`
	MemberResults    []MemberResult `json:"member_results"`
	ConfirmedEffects []string       `json:"confirmed_effects"`
	RecoveryRequired bool           `json:"recovery_required"`
	AvailableRepairs []string       `json:"available_repairs"`
}

func Mutation(plan Plan) MutationData {
	return MutationData{Plan: plan, MemberResults: []MemberResult{}, ConfirmedEffects: []string{}, AvailableRepairs: []string{}}
}

type RecordError struct {
	ID      *string `json:"id"`
	Source  string  `json:"source"`
	Code    string  `json:"code"`
	Message string  `json:"message"`
}
type ListData[T any] struct {
	Items          []T           `json:"items"`
	InvalidRecords []RecordError `json:"invalid_records"`
	Complete       bool          `json:"complete"`
	NextCursor     *string       `json:"next_cursor"`
}

func NewList[T any]() ListData[T] {
	return ListData[T]{Items: []T{}, InvalidRecords: []RecordError{}, Complete: true}
}
func Invalid(field, reason string) error {
	return Fail("config.invalid", fmt.Sprintf("%s: %s", field, reason))
}
