package cmd

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// versionLine is the single user-facing version format shared by
// `rho --version` and `rho version`.
func versionLine() string {
	ver := DisplayVersion()
	if ver != "" && !strings.HasPrefix(ver, "v") && !strings.HasPrefix(ver, "V") {
		ver = "v" + ver
	}
	line := auditTint("rho", textPrimary) + " " + auditTint(ver, rhoColor)
	if d := strings.TrimSpace(buildDate); d != "" && d != "unknown" {
		line += auditTint(" (built "+d+")", textMuted)
	}
	return line
}

// DisplayVersion returns the user-facing version string for banners and /version.
//
// In order of preference:
//  1. the version injected via ldflags (release archives, `make build`);
//  2. the module version recorded by `go install .../cmd/rho@vX.Y.Z`;
//  3. the repository VERSION file (local `go build` / `go run` checkouts);
//  4. "dev".
func DisplayVersion() string {
	v := strings.TrimSpace(version)
	if v != "" && v != "dev" {
		return v
	}
	if mv := releaseModuleVersion(); mv != "" {
		return mv
	}
	if fromFile := readRepoVERSIONFile(); fromFile != "" {
		return fromFile
	}
	if v != "" {
		return v
	}
	return "dev"
}

// readBuildInfo is debug.ReadBuildInfo, replaceable in tests.
var readBuildInfo = debug.ReadBuildInfo

// releaseModuleVersion returns the main module's version without the leading
// "v" when the binary was built from a tagged module (`go install
// github.com/GrayCodeAI/rho/cmd/rho@v0.3.0`), and "" for development builds:
// "(devel)", pseudo-versions (untagged commits such as @main) and versions
// with build metadata (e.g. "+dirty" from a modified checkout).
func releaseModuleVersion() string {
	info, ok := readBuildInfo()
	if !ok || info == nil {
		return ""
	}
	mv := info.Main.Version
	if !semver.IsValid(mv) || module.IsPseudoVersion(mv) || semver.Build(mv) != "" {
		return ""
	}
	return strings.TrimPrefix(mv, "v")
}

func readRepoVERSIONFile() string {
	candidates := versionFileCandidates()
	for _, path := range candidates {
		data, err := os.ReadFile(path) // #nosec G304 -- path from internal candidate list, not external input
		if err != nil {
			continue
		}
		v := strings.TrimSpace(string(data))
		if v != "" {
			return v
		}
	}
	return ""
}

func versionFileCandidates() []string {
	var out []string
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for i := 0; i < 4; i++ {
			out = append(out, filepath.Join(dir, "VERSION"))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		dir := cwd
		for i := 0; i < 4; i++ {
			out = append(out, filepath.Join(dir, "VERSION"))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return out
}
