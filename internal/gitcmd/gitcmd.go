// Package gitcmd centralizes hardened git invocation.
//
// Git honors repository-local configuration that can execute arbitrary
// programs: diff.external, core.fsmonitor, core.pager, and core.editor all
// name executables that git will run. A malicious .git/config in a cloned
// repository therefore turns an ordinary `git diff` or `git status` into code
// execution. Command passes -c overrides that take precedence over repository
// config so these hooks cannot run.
package gitcmd

import (
	"context"
	"os"
	"os/exec"
)

// repoConfigOverrides neutralize repository-local git configuration that can
// execute arbitrary programs. They are passed with -c so they take precedence
// over any value in the repository's .git/config.
var repoConfigOverrides = []string{
	"-c", "core.fsmonitor=false",
	"-c", "core.pager=cat",
	"-c", "core.editor=true",
	"-c", "diff.external=",
}

// Env returns the process environment with system git config disabled. Global
// user config is preserved so user.name/user.email continue to work.
func Env() []string {
	return append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
}

// Command builds a git command with hardened configuration. Callers may set
// Dir/Stdout/Stderr on the returned command as usual.
func Command(ctx context.Context, args ...string) *exec.Cmd {
	full := make([]string, 0, len(repoConfigOverrides)+len(args))
	full = append(full, repoConfigOverrides...)
	full = append(full, args...)
	cmd := exec.CommandContext(ctx, "git", full...) // #nosec G204 -- executable is the fixed "git" binary; args are config overrides plus caller-supplied git arguments
	cmd.Env = Env()
	return cmd
}
