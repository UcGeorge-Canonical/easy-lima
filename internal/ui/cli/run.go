package cli

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/UcGeorge-Canonical/easy-lima/internal/app"
	"github.com/UcGeorge-Canonical/easy-lima/internal/config"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/credential"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
	"github.com/UcGeorge-Canonical/easy-lima/internal/project"
	"github.com/UcGeorge-Canonical/easy-lima/internal/ui/tui"
	"golang.org/x/term"
)

func Run(ctx context.Context, args []string, in io.Reader, out, stderr io.Writer) int {
	s, err := app.New()
	if err != nil {
		fmt.Fprintln(stderr, model.AsError(err).Message)
		return model.AsError(err).Exit
	}
	s.In = in
	s.Out = out
	s.Err = stderr
	s.Secret = func(label string) ([]byte, error) { return credential.PromptContext(ctx, label) }
	if len(args) == 2 && args[0] == "__ssh-proxy" {
		if err = s.Proxy(ctx, args[1]); err != nil {
			fmt.Fprintln(stderr, model.AsError(err).Message)
			return 255
		}
		return 0
	}
	parsed, err := Parse(args, s.CWD)
	if err != nil {
		result := model.NewResult(parsed.Request.Command)
		result.SetError(err)
		if parsed.Request.Command == "exec" {
			result.ExitCode = 255
		}
		jsonMode := false
		for _, arg := range args {
			if arg == "--" {
				break
			}
			if arg == "--json" || arg == "--json=true" {
				jsonMode = true
			}
		}
		writer := stderr
		if jsonMode {
			writer = out
		}
		_ = Render(writer, result, jsonMode)
		return result.ExitCode
	}
	if parsed.Help {
		fmt.Fprint(out, Help(parsed.Request.Command))
		return 0
	}
	interactive := false
	if input, ok := in.(*os.File); ok {
		if output, ok := out.(*os.File); ok {
			interactive = term.IsTerminal(int(input.Fd())) && term.IsTerminal(int(output.Fd()))
		}
	}
	reader := bufio.NewReader(in)
	prompt := func(label, initial string) (string, error) {
		fmt.Fprint(out, Safe(label))
		if initial != "" {
			fmt.Fprintf(out, " [%s]", Safe(initial))
		}
		fmt.Fprint(out, ": ")
		type readResult struct {
			line string
			err  error
		}
		read := make(chan readResult, 1)
		go func() { line, e := reader.ReadString('\n'); read <- readResult{line, e} }()
		var line string
		var e error
		select {
		case <-ctx.Done():
			return "", model.Fail("operation.cancelled", "Input cancelled.")
		case result := <-read:
			line, e = result.line, result.err
		}
		if e != nil {
			return "", model.Fail("operation.cancelled", "Input ended before the choice was complete.")
		}
		line = strings.TrimSpace(line)
		if line == "" {
			line = initial
		}
		return line, nil
	}
	s.Confirm = func(plan model.Plan) (bool, error) {
		RenderPlan(out, plan)
		answer, e := prompt("Apply this plan? (yes/no)", "no")
		return answer == "yes", e
	}
	s.Progress = func(stage string) { fmt.Fprintln(stderr, "  "+Safe(stage)) }
	if parsed.Browse {
		if !interactive {
			fmt.Fprintln(stderr, "A terminal is required for the browser. Use easy-lima up PATH, list, or --help.")
			return 2
		}
		root := s.CWD
		if parsed.Request.Path != "" {
			root, err = project.Resolve(parsed.Request.Path, s.CWD, true)
			if err != nil {
				fmt.Fprintln(stderr, model.AsError(err).Message)
				return 2
			}
		}
		return browse(ctx, s, root, parsed.Request, prompt)
	}
	r := parsed.Request
	r.Interactive = interactive && !r.JSON
	if r.Interactive {
		if err = completeInteractive(ctx, s, &r, prompt); err != nil {
			result := model.NewResult(r.Command)
			result.SetError(err)
			_ = Render(stderr, result, false)
			return result.ExitCode
		}
	}
	result := s.Execute(ctx, r)
	if spec := Specs[r.Command]; spec.Raw {
		if len(result.Errors) > 0 {
			_ = Render(stderr, result, false)
		} else if r.Command == "code" || r.DryRun {
			_ = Render(out, result, false)
		}
		return result.ExitCode
	}
	if r.Follow && len(result.Errors) == 0 {
		return result.ExitCode
	}
	_ = Render(out, result, r.JSON)
	return result.ExitCode
}

type prompter func(string, string) (string, error)

func completeInteractive(ctx context.Context, s app.Service, r *app.Request, prompt prompter) error {
	var err error
	if r.Command == "init" && r.StorageRoot == "" {
		home, _ := os.UserHomeDir()
		r.StorageRoot, err = tui.PickStorage(ctx, s.CWD, home, s.In, s.Out)
		if err == nil && r.StorageRoot == "" {
			err = model.Fail("operation.cancelled", "Storage selection cancelled; no files were created.")
		}
		if err != nil {
			return err
		}
	}
	if r.Command == "init" && r.OpenAction == "" {
		choice, e := tui.Choose(ctx, "How should Enter open your projects?", []string{"VS Code over SSH", "SSH terminal"}, s.In, s.Out)
		if e != nil {
			return e
		}
		if choice < 0 {
			return model.Fail("operation.cancelled", "Setup cancelled.")
		}
		r.OpenAction = []string{"code", "ssh"}[choice]
	}
	if r.Command == "profile.create" || r.Command == "profile.import" {
		if r.Label == "" {
			r.Label, err = prompt("Profile label", "")
			if err != nil {
				return err
			}
		}
		if r.GitName == "" {
			r.GitName, err = prompt("Git author name", "")
			if err != nil {
				return err
			}
		}
		if r.GitEmail == "" {
			r.GitEmail, err = prompt("Git author email", "")
			if err != nil {
				return err
			}
		}
		if r.GitHubUser == "" {
			r.GitHubUser, err = prompt("Expected GitHub username (optional)", "")
			if err != nil {
				return err
			}
		}
		if r.Command == "profile.import" && r.Key == "" {
			r.Key, err = prompt("Existing private key path", "")
			if err != nil {
				return err
			}
		}
		if r.Remember == nil {
			value, e := prompt("Remember the passphrase on this device? (yes/no)", "no")
			if e != nil {
				return e
			}
			r.Remember = model.Ptr(value == "yes")
		}
	}
	if r.Command == "profile.edit" && r.GitName == "" && r.GitEmail == "" && r.GitHubUser == "" && !r.GitHubUserSet && r.Key == "" && r.PublicKey == "" && r.NewLabel == "" && r.SignCommits == nil && r.SignTags == nil && r.Remember == nil {
		result := s.Execute(ctx, app.Request{Command: "profile.show", Label: r.Label, PassphraseFD: -1})
		if len(result.Errors) > 0 {
			return &model.Error{Issue: result.Errors[0], Exit: result.ExitCode}
		}
		data, ok := result.Data.(map[string]any)
		if !ok {
			return model.Fail("record.invalid", "Profile response is invalid.")
		}
		profile, ok := data["profile"].(model.Profile)
		if !ok {
			return model.Fail("record.invalid", "Profile response is invalid.")
		}
		f, e := config.LoadProfile(filepath.Join(s.Paths.Config, "profiles", profile.ID+".toml"))
		if e != nil {
			return e
		}
		r.NewLabel, e = prompt("Profile label", f.Value.Label)
		if e != nil {
			return e
		}
		r.GitName, e = prompt("Git author name", f.Value.Git.Name)
		if e != nil {
			return e
		}
		r.GitEmail, e = prompt("Git author email", f.Value.Git.Email)
		if e != nil {
			return e
		}
		r.GitHubUser, e = prompt("Expected GitHub username (optional)", f.Value.GitHub.Username)
		if e != nil {
			return e
		}
		for _, flag := range []struct {
			label   string
			current bool
			target  **bool
		}{{"Sign commits", f.Value.Git.SignCommits, &r.SignCommits}, {"Sign tags", f.Value.Git.SignTags, &r.SignTags}, {"Remember passphrase on this device", f.Value.Unlock.Remember, &r.Remember}} {
			initial := "no"
			if flag.current {
				initial = "yes"
			}
			value, e := prompt(flag.label+" (yes/no)", initial)
			if e != nil {
				return e
			}
			if value != "yes" && value != "no" {
				return model.Invalid(flag.label, "answer yes or no")
			}
			*flag.target = model.Ptr(value == "yes")
		}
	}
	if r.Command == "delete" && r.ConfirmVM == "" && !r.DryRun {
		inspect := s.Execute(ctx, app.Request{Command: "inspect", Path: r.Path, VMID: r.VMID, Limit: 100, PassphraseFD: -1})
		if len(inspect.Errors) > 0 {
			return &model.Error{Issue: inspect.Errors[0], Exit: inspect.ExitCode}
		}
		if data, ok := inspect.Data.(map[string]any); ok {
			if v, ok := data["vm"].(model.VM); ok {
				fmt.Fprintf(s.Out, "Delete VM %s and its disk. The host checkout remains at %s.\n", v.ID, Safe(v.SourcePath))
				r.ConfirmVM, err = prompt("Type the full VM ID to confirm", "")
				return err
			}
		}
	}
	return nil
}
func browse(ctx context.Context, s app.Service, root string, shared app.Request, prompt prompter) int {
	var err error
	execute := s.Execute
	sExecute := func(ctx context.Context, r app.Request) model.Result {
		r.UserConfig = shared.UserConfig
		r.ProjectConfig = shared.ProjectConfig
		r.Limactl = shared.Limactl
		if r.Interactive && !Specs[r.Command].Raw && !Specs[r.Command].ReadOnly && r.Command != "init" {
			return executeTask(ctx, s, r)
		}
		return execute(ctx, r)
	}
	configPath := filepath.Join(s.Paths.Config, "config.toml")
	if shared.UserConfig != "" {
		var e error
		configPath, e = config.ResolvePath(shared.UserConfig, s.CWD)
		if e != nil {
			fmt.Fprintln(s.Err, e)
			return 2
		}
	}
	_, configErr := os.Stat(configPath)
	_, stateErr := os.Stat(filepath.Join(s.Paths.State, "state.sqlite3"))
	if os.IsNotExist(configErr) || os.IsNotExist(stateErr) {
		r := app.Request{Command: "init", Interactive: true, PassphraseFD: -1}
		if user, e := config.LoadUser(configPath); e == nil {
			r.StorageRoot = user.Value.Storage.Root
			r.OpenAction = user.Value.UI.OpenAction
		}
		for {
			if err = completeInteractive(ctx, s, &r, prompt); err != nil {
				return 130
			}
			result := sExecute(ctx, r)
			_ = Render(s.Out, result, false)
			if result.ExitCode == 0 {
				break
			}
			if len(result.Errors) != 1 || (result.Errors[0].Code != "config.invalid" && result.Errors[0].Code != "storage.unavailable" && result.Errors[0].Code != "operation.failed") {
				return result.ExitCode
			}
			choice, e := tui.Choose(ctx, "Setup could not use that location", []string{"Choose a storage location again", "Cancel setup"}, s.In, s.Out)
			if e != nil || choice != 0 {
				return result.ExitCode
			}
			r.StorageRoot = ""
		}
		choice, e := tui.Choose(ctx, "Reusable Git identity (optional)", []string{"Continue without setting up a profile", "Create or import a Git profile…"}, s.In, s.Out)
		if e != nil {
			return 130
		}
		if choice == 1 {
			profileMenu(ctx, s, nil, prompt, sExecute)
		}
	}
	var session *tui.Model
	pages := 1
	for ctx.Err() == nil {
		vms := []model.VM{}
		profiles := []model.Profile{}
		notice := ""
		unavailable := false
		for _, command := range []string{"list", "profile.list"} {
			cursor := ""
			for page := 0; page < pages; page++ {
				result := sExecute(ctx, app.Request{Command: command, Limit: 1000, Cursor: cursor, PassphraseFD: -1})
				if result.ExitCode != 0 {
					unavailable = true
					notice += command + " has unavailable records. "
				}
				var next *string
				switch data := result.Data.(type) {
				case model.ListData[model.VM]:
					vms = append(vms, data.Items...)
					next = data.NextCursor
				case model.ListData[model.Profile]:
					profiles = append(profiles, data.Items...)
					next = data.NextCursor
				}
				if next == nil {
					break
				}
				cursor = *next
				if page == pages-1 {
					notice += "Press n to load more " + command + " records. "
				}
			}
		}
		hidden, deps := false, false
		openAction := "code"
		if user, e := config.LoadUser(configPath); e == nil {
			hidden = user.Value.UI.ShowHidden
			deps = user.Value.UI.ShowDependencies
			openAction = user.Value.UI.OpenAction
		}
		updated, err := tui.BrowseSession(ctx, session, root, vms, profiles, unavailable, hidden, deps, openAction, s.In, s.Out, notice, func(c context.Context, id string) []string { return s.BrowserDetails(c, id, shared) })
		session = &updated
		selected := updated.Result
		if err != nil {
			return 130
		}
		if selected.Cancelled {
			return 0
		}
		r := app.Request{Command: selected.Action, Path: selected.Path, VMID: selected.VMID, Interactive: true, Limit: 100, PassphraseFD: -1}
		if r.VMID != "" {
			r.Path = ""
		}
		chainOpen := false
		switch selected.Action {
		case "profile.assign":
			options := []string{"No Git profile"}
			for _, p := range profiles {
				options = append(options, p.Label+" — "+p.Git.Email)
			}
			choice, e := tui.Choose(ctx, "Choose this VM's Git profile", options, s.In, s.Out)
			if e != nil || choice < 0 {
				continue
			}
			r.Path = ""
			r.VMID = ""
			r.VMIDs = []string{selected.VMID}
			if choice == 0 {
				r.Command = "profile.unassign"
			} else {
				r.Label = profiles[choice-1].Label
			}
		case "more":
			pages++
			continue
		case "profiles":
			if !profileMenu(ctx, s, profiles, prompt, sExecute) {
				continue
			}
			continue
		case "operations":
			result := sExecute(ctx, app.Request{Command: "operations.list", Limit: 1000, PassphraseFD: -1})
			_ = Render(s.Out, result, false)
			id, e := prompt("Operation ID to inspect/resume (blank returns)", "")
			if e != nil || id == "" {
				continue
			}
			which, e := tui.Choose(ctx, "Operation action", []string{"Inspect", "Resume"}, s.In, s.Out)
			if e != nil || which < 0 {
				continue
			}
			r = app.Request{Command: "operations.inspect", ID: id, Interactive: true, Limit: 100, PassphraseFD: -1}
			if which == 1 {
				r.Command = "operations.resume"
			}
		case "resume":
			var selectedVM *model.VM
			for i := range vms {
				if vms[i].ID == selected.VMID {
					selectedVM = &vms[i]
				}
			}
			if selectedVM == nil || len(selectedVM.OperationIDs) == 0 {
				continue
			}
			options := selectedVM.OperationIDs
			choice, e := tui.Choose(ctx, "Resume saved operation", options, s.In, s.Out)
			if e != nil || choice < 0 {
				continue
			}
			r = app.Request{Command: "operations.resume", ID: options[choice], Interactive: true, Limit: 100, PassphraseFD: -1}
		case "create-open", "up":
			r.Command = "up"
			chainOpen = selected.Action == "create-open"
			options := []string{"No Git profile"}
			for _, p := range profiles {
				options = append(options, p.Label+" — "+p.Git.Email)
			}
			options = append(options, "Create a reusable profile…", "Import an existing key…")
			title := "Choose this VM's identity"
			projectPath := filepath.Join(selected.Path, "easy-lima.toml")
			if shared.ProjectConfig != "" {
				projectPath, _ = config.ResolvePath(shared.ProjectConfig, s.CWD)
			}
			if file, e := config.LoadProject(projectPath); e == nil && file.Value.Identity.ProfileLabel != "" {
				title += " · project requests " + file.Value.Identity.ProfileLabel
			}
			choice, e := tui.Choose(ctx, title, options, s.In, s.Out)
			if e != nil || choice < 0 {
				continue
			}
			if choice == 0 {
				r.NoProfile = true
			} else if choice <= len(profiles) {
				r.Profile = profiles[choice-1].Label
			} else {
				pr := app.Request{Command: "profile.create", Interactive: true, PassphraseFD: -1}
				if choice == len(profiles)+2 {
					pr.Command = "profile.import"
				}
				if e = completeInteractive(ctx, s, &pr, prompt); e != nil {
					continue
				}
				res := sExecute(ctx, pr)
				_ = Render(s.Out, res, false)
				if res.ExitCode != 0 {
					_, _ = prompt("Press Enter to return", "")
					continue
				}
				r.Profile = pr.Label
			}
			preset, e := tui.Choose(ctx, "Ubuntu environment", []string{"Use configured environment", "Base tools", "Development: Snap, LXD, Juju", "Docker", "Development and Docker"}, s.In, s.Out)
			if e != nil || preset < 0 {
				continue
			}
			if preset > 0 {
				r.Settings.Environment.Presets = [][]string{{"base"}, {"base", "development"}, {"base", "docker"}, {"base", "development", "docker"}}[preset-1]
			}
		case "rebind":
			r.To, err = prompt("New project path", "")
			if err != nil {
				continue
			}
		case "repair":
			choice, e := tui.Choose(ctx, "Choose a specific repair", []string{"alias", "connection", "identity", "trust-export", "host-key", "ownership-publication"}, s.In, s.Out)
			if e != nil || choice < 0 {
				continue
			}
			r.RepairKind = []string{"alias", "connection", "identity", "trust-export", "host-key", "ownership-publication"}[choice]
			if r.RepairKind == "host-key" {
				r.HostKeyFile, err = prompt("Independently verified host public key file", "")
				if err != nil {
					continue
				}
			}
			r.Apply = true
		case "environment.update":
			r.FromProject = true
			r.Apply = true
		}
		if err = completeInteractive(ctx, s, &r, prompt); err != nil {
			continue
		}
		result := sExecute(ctx, r)
		_ = Render(s.Out, result, false)
		if chainOpen && result.ExitCode == 0 {
			open := app.Request{Command: openAction, Path: selected.Path, Interactive: true, PassphraseFD: -1}
			opened := sExecute(ctx, open)
			if opened.ExitCode != 0 {
				_ = Render(s.Out, opened, false)
				if openAction == "code" && editorMissing(opened) {
					choice, e := tui.Choose(ctx, "Editor unavailable", []string{"Open SSH instead", "Return to projects"}, s.In, s.Out)
					if e == nil && choice == 0 {
						open.Command = "ssh"
						_ = sExecute(ctx, open)
					}
				}
			}
		}
		if r.Command == "code" && editorMissing(result) {
			choice, e := tui.Choose(ctx, "VS Code launcher unavailable", []string{"Open SSH instead", "Return to projects"}, s.In, s.Out)
			if e == nil && choice == 0 {
				r.Command = "ssh"
				opened := sExecute(ctx, r)
				_ = Render(s.Out, opened, false)
			}
		}
		if result.ExitCode != 0 || Specs[r.Command].ReadOnly {
			_, err = prompt("Press Enter to return to projects", "")
			if err != nil {
				return 0
			}
		}
	}
	return 130
}
func profileMenu(ctx context.Context, s app.Service, profiles []model.Profile, prompt prompter, execute func(context.Context, app.Request) model.Result) bool {
	options := []string{"Create a profile…", "Import a key…"}
	for _, p := range profiles {
		options = append(options, p.Label+" — "+p.Git.Email+" ("+p.LockState+")")
	}
	choice, err := tui.Choose(ctx, "Profiles", options, s.In, s.Out)
	if err != nil || choice < 0 {
		return false
	}
	r := app.Request{Interactive: true, PassphraseFD: -1, Limit: 100}
	if choice < 2 {
		r.Command = []string{"profile.create", "profile.import"}[choice]
	} else {
		p := profiles[choice-2]
		actions := []string{"Show public key and capabilities", "Check locally", "Check GitHub account", "Edit desired identity", "Apply desired identity"}
		commands := []string{"profile.show", "profile.check", "profile.check", "profile.edit", "profile.apply"}
		title := p.Label
		if p.AgentManagement == "external" {
			title += " (external agent; lock/unlock/forget are managed externally)"
			actions = append(actions, "Migrate external agent")
			commands = append(commands, "profile.migrate-agent")
		} else {
			actions = append(actions, "Unlock", "Lock", "Forget saved passphrase")
			commands = append(commands, "profile.unlock", "profile.lock", "profile.forget")
		}
		actions = append(actions, "Remove unused profile")
		commands = append(commands, "profile.remove")
		action, e := tui.Choose(ctx, title, actions, s.In, s.Out)
		if e != nil || action < 0 {
			return false
		}
		r.Label = p.Label
		r.Command = commands[action]
		r.Online = action == 2

	}
	if err = completeInteractive(ctx, s, &r, prompt); err != nil {
		return false
	}
	result := execute(ctx, r)
	_ = Render(s.Out, result, false)
	_, _ = prompt("Press Enter to return", "")
	return true
}

func editorMissing(r model.Result) bool {
	for _, e := range r.Errors {
		if e.Code == "editor.unavailable" {
			return true
		}
	}
	return false
}

func executeTask(ctx context.Context, s app.Service, r app.Request) model.Result {
	result, err := tui.RunTask(ctx, strings.ReplaceAll(r.Command, ".", " "), s.In, s.Out, func(c context.Context, h tui.TaskHooks) model.Result {
		task := s
		task.Err = h.Output
		task.Progress = h.Progress
		task.Secret = h.Secret
		task.Confirm = func(plan model.Plan) (bool, error) {
			var b bytes.Buffer
			RenderPlan(&b, plan)
			return h.Confirm("Review changes", b.String())
		}
		return task.Execute(c, r)
	})
	if err != nil && len(result.Errors) == 0 {
		result.SetError(model.Fail("operation.cancelled", "The terminal interaction ended; inspect the saved operation before retrying."))
	}
	return result
}
