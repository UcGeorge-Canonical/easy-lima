package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/UcGeorge-Canonical/easy-lima/assets"
	"github.com/UcGeorge-Canonical/easy-lima/internal/connection"
	"github.com/UcGeorge-Canonical/easy-lima/internal/host/process"
	"github.com/UcGeorge-Canonical/easy-lima/internal/model"
)

type Bundle struct {
	Action     string            `json:"action"`
	RevisionID string            `json:"revision_id"`
	Files      map[string]string `json:"files"`
}

func gitValue(v string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`).Replace(v) + `"`
}
func Build(vmID, home, revisionID, publicKey string, git model.GitIdentity) (Bundle, error) {
	base := path.Join(home, ".config/easy-lima")
	socket := connection.GuestSocket(vmID)
	known, err := assets.Read("github-known-hosts")
	if err != nil {
		return Bundle{}, err
	}
	wrapper, err := assets.Read("guest/signing-wrapper.sh")
	if err != nil {
		return Bundle{}, err
	}
	sshconfig := fmt.Sprintf("Host github.com\n  HostName github.com\n  User git\n  IdentityAgent %s\n  IdentityFile %s\n  IdentitiesOnly yes\n  AddKeysToAgent no\n  ForwardAgent no\n  StrictHostKeyChecking yes\n  GlobalKnownHostsFile /dev/null\n  UserKnownHostsFile %s\n  VerifyHostKeyDNS no\n  UpdateHostKeys no\n  PasswordAuthentication no\n  KbdInteractiveAuthentication no\n  BatchMode yes\nHost *\n  IdentityAgent none\n  IdentitiesOnly yes\n  ForwardAgent no\n  BatchMode yes\n", gitValue(socket), gitValue(path.Join(base, "public_key")), gitValue(path.Join(base, "github_known_hosts")))
	gitconfig := fmt.Sprintf("[user]\n\tname = %s\n\temail = %s\n\tuseConfigOnly = true\n\tsigningKey = %s\n[gpg]\n\tformat = ssh\n[gpg \"ssh\"]\n\tprogram = %s\n\tallowedSignersFile = %s\n[commit]\n\tgpgSign = %t\n[tag]\n\tgpgSign = %t\n[core]\n\tsshCommand = %s\n[url \"git@github.com:\"]\n\tinsteadOf = https://github.com/\n", gitValue(git.Name), gitValue(git.Email), gitValue(path.Join(base, "public_key")), gitValue(path.Join(base, "signing-wrapper")), gitValue(path.Join(base, "allowed_signers")), git.SignCommits, git.SignTags, gitValue("ssh -F "+process.Quote(path.Join(base, "ssh_config"))))
	return Bundle{Action: "apply", RevisionID: revisionID, Files: map[string]string{"gitconfig": gitconfig, "public_key": publicKey + "\n", "allowed_signers": git.Email + " namespaces=\"git\" " + publicKey + "\n", "ssh_config": sshconfig, "github_known_hosts": string(known), "signing-wrapper": strings.ReplaceAll(string(wrapper), "@SOCKET@", socket)}}, nil
}
func Apply(ctx context.Context, c *connection.Client, b Bundle) error {
	script, err := assets.Read("guest/identity.py")
	if err != nil {
		return err
	}
	raw, err := json.Marshal(b)
	if err != nil {
		return err
	}
	var out, stderr bytes.Buffer
	code, err := c.Run(ctx, []string{"python3", "-c", string(script)}, bytes.NewReader(raw), &out, &stderr)
	if err != nil {
		return err
	}
	if code != 0 {
		return model.Fail("settings.edited", "Guest identity settings were edited or publication is incomplete; existing user files were preserved.")
	}
	return nil
}

type RepositoryCheck struct {
	Path        string `json:"path"`
	Repository  bool   `json:"repository"`
	Unavailable bool   `json:"unavailable"`
	Conflicts   []struct {
		Field  string `json:"field"`
		Origin string `json:"origin"`
	} `json:"conflicts"`
}
type RepositoryCoverage struct {
	Repositories []RepositoryCheck `json:"repositories"`
	Truncated    bool              `json:"truncated"`
}

func CheckRepositories(ctx context.Context, c *connection.Client, git model.GitIdentity, repositories []string, discover bool) (RepositoryCoverage, error) {
	var result RepositoryCoverage
	home, err := c.Output(ctx, "sh", "-c", `printf '%s' "$HOME"`)
	if err != nil {
		return result, err
	}
	base := path.Join(string(home), ".config/easy-lima")
	expected := map[string]string{"user.name": git.Name, "user.email": git.Email, "user.useConfigOnly": "true", "user.signingKey": path.Join(base, "public_key"), "gpg.format": "ssh", "gpg.ssh.program": path.Join(base, "signing-wrapper"), "gpg.ssh.allowedSignersFile": path.Join(base, "allowed_signers"), "commit.gpgSign": fmt.Sprint(git.SignCommits), "tag.gpgSign": fmt.Sprint(git.SignTags), "core.sshCommand": "ssh -F " + process.Quote(path.Join(base, "ssh_config")), "url.git@github.com:.insteadOf": "https://github.com/"}
	script, err := assets.Read("guest/git-check.py")
	if err != nil {
		return result, err
	}
	input, _ := json.Marshal(map[string]any{"expected": expected, "repositories": repositories, "discover": discover})
	var out, stderr bytes.Buffer
	code, err := c.Run(ctx, []string{"python3", "-c", string(script)}, bytes.NewReader(input), &out, &stderr)
	if err != nil {
		return result, err
	}
	if code != 0 || out.Len() > 1<<20 || json.Unmarshal(out.Bytes(), &result) != nil {
		return result, model.Fail("check.incomplete", "Effective Git settings could not be inspected.")
	}
	return result, nil
}
func VerifyGit(ctx context.Context, c *connection.Client, git model.GitIdentity) error {
	if err := Apply(ctx, c, Bundle{Action: "check", Files: map[string]string{}}); err != nil {
		return err
	}
	coverage, err := CheckRepositories(ctx, c, git, []string{"/workspace"}, false)
	if err != nil {
		return err
	}
	for _, repo := range coverage.Repositories {
		if repo.Unavailable {
			return model.Fail("check.incomplete", "The workspace is unavailable for identity verification.")
		}
		if len(repo.Conflicts) > 0 {
			conflict := repo.Conflicts[0]
			return model.Fail("settings.edited", fmt.Sprintf("Git field %s conflicts in %s (source: %s). User overrides were preserved.", conflict.Field, repo.Path, conflict.Origin))
		}
	}
	return nil
}

func RetireLegacy(ctx context.Context, c *connection.Client, git model.GitIdentity, public, account string) error {
	script, e := assets.Read("guest/adopt-identity.py")
	if e != nil {
		return e
	}
	raw, e := json.Marshal(map[string]string{"name": git.Name, "email": git.Email, "public_key": strings.TrimSpace(public), "github_user": account})
	if e != nil {
		return e
	}
	var out, stderr bytes.Buffer
	code, e := c.Run(ctx, []string{"python3", "-c", string(script)}, bytes.NewReader(raw), &out, &stderr)
	if e != nil {
		return e
	}
	if code != 0 {
		return model.Fail("settings.edited", "Legacy identity differs from its adopted public settings; user edits were preserved.")
	}
	return nil
}
