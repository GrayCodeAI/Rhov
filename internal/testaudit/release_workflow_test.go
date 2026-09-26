package testaudit

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Release pipeline invariants: the files that together produce, sign and
// install a rho release must agree with each other. See docs/RELEASING.md.

func TestGoreleaserConfigShipsVerifiableArchives(t *testing.T) {
	var cfg struct {
		ProjectName string `yaml:"project_name"`
		Builds      []struct {
			Main   string   `yaml:"main"`
			Binary string   `yaml:"binary"`
			Goos   []string `yaml:"goos"`
			Goarch []string `yaml:"goarch"`
			Ignore []any    `yaml:"ignore"`
		} `yaml:"builds"`
		Archives []struct {
			NameTemplate string `yaml:"name_template"`
		} `yaml:"archives"`
		Checksum struct {
			NameTemplate string `yaml:"name_template"`
		} `yaml:"checksum"`
		Signs []struct {
			Cmd       string   `yaml:"cmd"`
			Artifacts string   `yaml:"artifacts"`
			Signature string   `yaml:"signature"`
			Args      []string `yaml:"args"`
		} `yaml:"signs"`
		Sboms []any `yaml:"sboms"`
	}
	raw := readRepoFile(t, ".goreleaser.yml")
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parse .goreleaser.yml: %v", err)
	}
	var top map[string]any
	if err := yaml.Unmarshal(raw, &top); err != nil {
		t.Fatalf("parse .goreleaser.yml: %v", err)
	}
	// Neither the Homebrew tap nor the npm packages exist; publishing to them
	// would fail the release after the GitHub release is created.
	for _, key := range []string{"brews", "homebrew_casks", "nfpms", "npms"} {
		if _, ok := top[key]; ok {
			t.Errorf(".goreleaser.yml configures %q, but that channel does not exist (see docs/RELEASING.md)", key)
		}
	}

	if cfg.ProjectName != "rho" {
		t.Errorf("project_name = %q, want rho (install.sh downloads rho_<version>_<os>_<arch>)", cfg.ProjectName)
	}
	if len(cfg.Builds) != 1 || cfg.Builds[0].Main != "./cmd/rho" || cfg.Builds[0].Binary != "rho" {
		t.Fatalf("want exactly one build of ./cmd/rho named rho, got %+v", cfg.Builds)
	}
	b := cfg.Builds[0]
	if strings.Join(b.Goos, ",") != "linux,darwin,windows" || strings.Join(b.Goarch, ",") != "amd64,arm64" || len(b.Ignore) != 0 {
		t.Errorf("builds must cover linux/darwin/windows × amd64/arm64 with no ignores (README promises all six), got goos=%v goarch=%v ignore=%v", b.Goos, b.Goarch, b.Ignore)
	}
	if len(cfg.Archives) != 1 || strings.TrimSpace(cfg.Archives[0].NameTemplate) != "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}" {
		t.Errorf("archive name_template changed; install.sh and .github/actions/rho depend on rho_<version>_<os>_<arch>: %+v", cfg.Archives)
	}
	if cfg.Checksum.NameTemplate != "checksums.txt" {
		t.Errorf("checksum name = %q, want checksums.txt", cfg.Checksum.NameTemplate)
	}
	if len(cfg.Sboms) == 0 {
		t.Error("sboms stanza missing; release.yml installs syft for it")
	}
	if len(cfg.Signs) != 1 {
		t.Fatalf("want one signs entry (cosign over checksums.txt), got %d", len(cfg.Signs))
	}
	s := cfg.Signs[0]
	if s.Cmd != "cosign" || s.Artifacts != "checksum" || s.Signature != "${artifact}.sigstore.json" {
		t.Errorf("signs = %+v, want cosign over the checksum artifact producing ${artifact}.sigstore.json", s)
	}
	if !slices.Contains(s.Args, "sign-blob") || !slices.Contains(s.Args, "--bundle=${signature}") {
		t.Errorf("cosign args = %q, want sign-blob --bundle=${signature}", s.Args)
	}

	install := string(readRepoFile(t, "install.sh"))
	for _, want := range []string{
		"checksums.txt.sigstore.json",
		`${BINARY}_${VERSION}_${OS}_${ARCH}.${ARCHIVE_EXT}`,
		".github/workflows/release.yml@refs/tags/",
	} {
		if !strings.Contains(install, want) {
			t.Errorf("install.sh does not reference %q; it must match .goreleaser.yml and release.yml", want)
		}
	}
}

func TestReleaseWorkflowToolingAndPermissions(t *testing.T) {
	var wf struct {
		Permissions map[string]string `yaml:"permissions"`
		Jobs        map[string]struct {
			Steps []struct {
				Name string            `yaml:"name"`
				Uses string            `yaml:"uses"`
				Run  string            `yaml:"run"`
				With map[string]string `yaml:"with"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	raw := readRepoFile(t, ".github/workflows/release.yml")
	if err := yaml.Unmarshal(raw, &wf); err != nil {
		t.Fatalf("parse release.yml: %v", err)
	}
	wantPerms := map[string]string{"contents": "write", "id-token": "write"}
	if len(wf.Permissions) != len(wantPerms) {
		t.Errorf("release.yml permissions = %v, want exactly %v", wf.Permissions, wantPerms)
	}
	for k, v := range wantPerms {
		if wf.Permissions[k] != v {
			t.Errorf("release.yml permissions[%s] = %q, want %q", k, wf.Permissions[k], v)
		}
	}
	job, ok := wf.Jobs["goreleaser"]
	if !ok {
		t.Fatal("release.yml has no goreleaser job")
	}
	index := func(pred func(i int) bool) int {
		for i := range job.Steps {
			if pred(i) {
				return i
			}
		}
		return -1
	}
	usesPrefix := func(prefix string) int {
		return index(func(i int) bool { return strings.HasPrefix(job.Steps[i].Uses, prefix) })
	}
	goreleaser := usesPrefix("goreleaser/goreleaser-action@")
	syft := usesPrefix("anchore/sbom-action/download-syft@")
	cosign := usesPrefix("sigstore/cosign-installer@")
	guard := index(func(i int) bool { return strings.Contains(job.Steps[i].Run, "scripts/check-release-tag.sh") })
	switch {
	case goreleaser < 0:
		t.Fatal("release.yml does not run goreleaser")
	case syft < 0 || syft > goreleaser:
		t.Error("syft must be installed before GoReleaser (the sboms stanza shells out to it)")
	case cosign < 0 || cosign > goreleaser:
		t.Error("cosign must be installed before GoReleaser (the signs stanza shells out to it)")
	case guard < 0 || guard > goreleaser:
		t.Error("scripts/check-release-tag.sh must run before GoReleaser")
	}
	if args := job.Steps[max(goreleaser, 0)].With["args"]; !strings.Contains(args, "--release-notes=") {
		t.Errorf("goreleaser args = %q, want --release-notes from the CHANGELOG section", args)
	}
	for _, step := range job.Steps {
		if strings.Contains(step.Uses, "checkout-flux") {
			t.Error("release.yml must build from go.mod/go.sum, not sibling checkouts")
		}
		if strings.Contains(step.Run, "-D dist") || strings.Contains(step.Run, "--dir dist") {
			t.Errorf("step %q downloads into goreleaser's dist/ directory", step.Name)
		}
	}
	for _, pinned := range []int{goreleaser, syft, cosign} {
		if pinned >= 0 && !regexp.MustCompile(`@[0-9a-f]{40}$`).MatchString(job.Steps[pinned].Uses) {
			t.Errorf("%s is not pinned to a commit SHA", job.Steps[pinned].Uses)
		}
	}
	if bytes.Contains(raw, []byte("release-please")) || bytes.Contains(raw, []byte("HOMEBREW_TAP_TOKEN")) {
		t.Error("release.yml references release-please or Homebrew tap tokens that do not exist")
	}
}

func TestReleaseTagGuard(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("scripts/check-release-tag.sh is a bash script; covered on Linux and macOS")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skipf("bash not available: %v", err)
	}
	script := filepath.Join(repoRoot(t), "scripts", "check-release-tag.sh")
	run := func(t *testing.T, version, changelog string, args ...string) (int, string) {
		t.Helper()
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte(version+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte(changelog), 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(bash, append([]string{script}, args...)...)
		cmd.Env = append(os.Environ(), "RELEASE_CHECK_ROOT="+root)
		out, err := cmd.CombinedOutput()
		var exitErr *exec.ExitError
		switch {
		case err == nil:
			return 0, string(out)
		case errors.As(err, &exitErr):
			return exitErr.ExitCode(), string(out)
		default:
			t.Fatalf("run guard: %v", err)
			return -1, ""
		}
	}
	const good = "# Changelog\n\n## [Unreleased]\n\n## [0.3.0] — 2026-09-30\n\n### Added\n- first rho release\n\n## [hawk 0.2.0] — 2026-07-13\n\n- old\n"

	t.Run("matching tag writes notes", func(t *testing.T) {
		notes := filepath.Join(t.TempDir(), "notes.md")
		code, out := run(t, "0.3.0", good, "--notes", notes, "v0.3.0")
		if code != 0 {
			t.Fatalf("exit %d: %s", code, out)
		}
		got, err := os.ReadFile(notes)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(got), "- first rho release") || strings.Contains(string(got), "hawk") {
			t.Errorf("notes should hold only the 0.3.0 section, got:\n%s", got)
		}
	})
	failures := []struct {
		name, version, changelog, tag, want string
	}{
		{"tag differs from VERSION", "0.3.0", good, "v0.3.1", "does not match VERSION"},
		{"tag without v", "0.3.0", good, "0.3.0", "does not match VERSION"},
		{"VERSION not semver", "0.3", good, "v0.3", "not a SemVer version"},
		{"no changelog section", "0.3.0", "# Changelog\n\n## [Unreleased]\n\n- pending\n", "v0.3.0", "no non-empty '## [0.3.0]' section"},
		{"empty changelog section", "0.3.0", "## [Unreleased]\n\n## [0.3.0] — 2026-09-30\n\n## [hawk 0.2.0]\n- old\n", "v0.3.0", "no non-empty"},
		{"dot is not a wildcard", "0.3.0", "## [0x3x0] — 2026-09-30\n\n- wrong\n", "v0.3.0", "no non-empty"},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			code, out := run(t, tc.version, tc.changelog, tc.tag)
			if code == 0 || !strings.Contains(out, tc.want) {
				t.Fatalf("want failure containing %q, got exit %d: %s", tc.want, code, out)
			}
		})
	}
	t.Run("usage", func(t *testing.T) {
		if code, out := run(t, "0.3.0", good); code != 2 {
			t.Fatalf("want usage exit 2, got %d: %s", code, out)
		}
	})
}
