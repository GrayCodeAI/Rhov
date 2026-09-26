package testaudit

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// These tests run the real install.sh end to end against a fake GitHub release
// served by a stub `curl`, with `uname` and (optionally) `cosign` stubbed too.
// PATH is restricted to the stubs plus a curated set of system tools, so the
// presence or absence of cosign on the host never changes the outcome.

const installTestVersion = "0.3.0"

// installFixture is one fake release tree plus the sandboxed tool directory.
type installFixture struct {
	t          *testing.T
	root       string // repo root (install.sh lives here)
	releaseDir string // <releaseDir>/<tag>/<asset>, <releaseDir>/latest.json
	stubDir    string
	toolDir    string
	prefix     string
	curlLog    string
	cosignLog  string
	env        map[string]string
}

type installResult struct {
	exitCode int
	stdout   string
	stderr   string
}

func (r installResult) output() string { return r.stdout + r.stderr }

// systemTools are linked into the sandbox PATH. Missing optional tools are
// skipped; install.sh needs one of sha256sum/shasum/openssl.
var systemTools = []string{
	"sh", "awk", "sed", "grep", "tr", "mktemp", "rm", "mkdir", "mv", "cp", "ln",
	"chmod", "cat", "head", "wc", "tar", "gzip", "unzip", "sha256sum", "shasum",
	"openssl", "perl", "basename", "dirname", "env",
}

func newInstallFixture(t *testing.T) *installFixture {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("install.sh is a POSIX shell script; covered on Linux and macOS")
	}
	for _, tool := range []string{"sh", "tar", "gzip", "awk", "sed"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("install.sh test needs %s on PATH: %v", tool, err)
		}
	}
	base := t.TempDir()
	f := &installFixture{
		t:          t,
		root:       repoRoot(t),
		releaseDir: filepath.Join(base, "release"),
		stubDir:    filepath.Join(base, "stubs"),
		toolDir:    filepath.Join(base, "tools"),
		prefix:     filepath.Join(base, "prefix"),
		curlLog:    filepath.Join(base, "curl.log"),
		cosignLog:  filepath.Join(base, "cosign.log"),
		env: map[string]string{
			"FAKE_UNAME_S": "Linux",
			"FAKE_UNAME_M": "x86_64",
		},
	}
	for _, dir := range []string{f.releaseDir, f.stubDir, f.toolDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, tool := range systemTools {
		path, err := exec.LookPath(tool)
		if err != nil {
			continue
		}
		if err := os.Symlink(path, filepath.Join(f.toolDir, tool)); err != nil {
			t.Fatal(err)
		}
	}
	f.writeStub("curl", fakeCurl)
	f.writeStub("uname", fakeUname)
	return f
}

func (f *installFixture) writeStub(name, body string) {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.stubDir, name), []byte(body), 0o755); err != nil {
		f.t.Fatal(err)
	}
}

// withCosign installs a cosign stub that records its arguments and exits with
// the given code.
func (f *installFixture) withCosign(exitCode int) {
	f.t.Helper()
	f.writeStub("cosign", fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$@\" > %q\nexit %d\n", f.cosignLog, exitCode))
}

// addAsset writes a release asset for tag.
func (f *installFixture) addAsset(tag, name string, data []byte) {
	f.t.Helper()
	dir := filepath.Join(f.releaseDir, tag)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// addRelease publishes a complete fake release (archive, checksums, bundle) for
// version on goos/goarch and returns the archive name.
func (f *installFixture) addRelease(version, goos, goarch string) string {
	f.t.Helper()
	tag := "v" + version
	var archive []byte
	var name string
	if goos == "windows" {
		name = fmt.Sprintf("rho_%s_%s_%s.zip", version, goos, goarch)
		archive = makeZip(f.t, "rho.exe", "fake rho "+version)
	} else {
		name = fmt.Sprintf("rho_%s_%s_%s.tar.gz", version, goos, goarch)
		archive = makeTarGz(f.t, "rho", "#!/bin/sh\necho fake rho "+version+"\n")
	}
	sum := sha256.Sum256(archive)
	checksums := fmt.Sprintf("%s  %s\n%s  rho_%s_source.tar.gz\n",
		hex.EncodeToString(sum[:]), name, strings.Repeat("0", 64), version)
	f.addAsset(tag, name, archive)
	f.addAsset(tag, "checksums.txt", []byte(checksums))
	f.addAsset(tag, "checksums.txt.sigstore.json", []byte(`{"mediaType":"application/vnd.dev.sigstore.bundle.v0.3+json"}`))
	return name
}

func (f *installFixture) setLatest(tag string) {
	f.t.Helper()
	body := fmt.Sprintf("{\n  \"url\": \"https://api.github.com/repos/GrayCodeAI/rho/releases/1\",\n  \"tag_name\": %q,\n  \"name\": %q\n}\n", tag, tag)
	if err := os.WriteFile(filepath.Join(f.releaseDir, "latest.json"), []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *installFixture) run(args ...string) installResult {
	f.t.Helper()
	cmdArgs := append([]string{filepath.Join(f.root, "install.sh")}, args...)
	cmd := exec.Command(filepath.Join(f.toolDir, "sh"), cmdArgs...)
	env := []string{
		"PATH=" + f.stubDir + string(os.PathListSeparator) + f.toolDir,
		"HOME=" + filepath.Join(filepath.Dir(f.prefix), "home"),
		"FAKE_RELEASE_DIR=" + f.releaseDir,
		"FAKE_CURL_LOG=" + f.curlLog,
	}
	for k, v := range f.env {
		env = append(env, k+"="+v)
	}
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	res := installResult{stdout: stdout.String(), stderr: stderr.String()}
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			f.t.Fatalf("running install.sh: %v", err)
		}
		res.exitCode = exitErr.ExitCode()
	}
	return res
}

func (f *installFixture) requireInstalled(res installResult, prefix, version string) {
	f.t.Helper()
	if res.exitCode != 0 {
		f.t.Fatalf("install.sh exit %d, want 0\n%s", res.exitCode, res.output())
	}
	versioned := filepath.Join(prefix, "bin", "rho-"+version)
	if _, err := os.Stat(versioned); err != nil {
		f.t.Fatalf("versioned binary missing: %v\n%s", err, res.output())
	}
	link, err := os.Readlink(filepath.Join(prefix, "bin", "rho"))
	if err != nil {
		f.t.Fatalf("rho symlink missing: %v", err)
	}
	if link != "rho-"+version {
		f.t.Fatalf("rho symlink points at %q, want %q", link, "rho-"+version)
	}
}

func (f *installFixture) curlCalls() []string {
	f.t.Helper()
	data, err := os.ReadFile(f.curlLog)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		f.t.Fatal(err)
	}
	return strings.Fields(string(data))
}

func TestInstallScriptWithoutCosignVerifiesChecksumAndWarns(t *testing.T) {
	f := newInstallFixture(t)
	f.addRelease(installTestVersion, "linux", "amd64")

	res := f.run("--version", installTestVersion, "--prefix", f.prefix)
	f.requireInstalled(res, f.prefix, installTestVersion)
	if !strings.Contains(res.stdout, "checksum:  verified") {
		t.Errorf("summary does not report the checksum verification:\n%s", res.stdout)
	}
	if !strings.Contains(res.stdout, "signature: NOT verified (cosign not installed)") {
		t.Errorf("summary does not say the signature was not verified:\n%s", res.stdout)
	}
	if !strings.Contains(res.stderr, "WARNING: cosign is not installed") {
		t.Errorf("missing cosign warning on stderr:\n%s", res.stderr)
	}
	for _, call := range f.curlCalls() {
		if strings.HasSuffix(call, ".sigstore.json") {
			t.Errorf("bundle downloaded although cosign is absent: %s", call)
		}
	}
}

func TestInstallScriptRequireCosignFailsClosed(t *testing.T) {
	f := newInstallFixture(t)
	f.addRelease(installTestVersion, "linux", "amd64")
	f.env["RHO_REQUIRE_COSIGN"] = "1"

	res := f.run("--version", installTestVersion, "--prefix", f.prefix)
	if res.exitCode == 0 {
		t.Fatalf("install succeeded without cosign although RHO_REQUIRE_COSIGN=1\n%s", res.output())
	}
	if !strings.Contains(res.stderr, "RHO_REQUIRE_COSIGN=1 but cosign is not installed") {
		t.Errorf("unexpected error:\n%s", res.stderr)
	}
	if _, err := os.Stat(filepath.Join(f.prefix, "bin")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("install directory created despite failure (stat err=%v)", err)
	}
}

func TestInstallScriptCosignUsesExactReleaseIdentity(t *testing.T) {
	f := newInstallFixture(t)
	f.addRelease(installTestVersion, "linux", "amd64")
	f.withCosign(0)

	res := f.run("--version", "v"+installTestVersion, "--prefix", f.prefix)
	f.requireInstalled(res, f.prefix, installTestVersion)

	data, err := os.ReadFile(f.cosignLog)
	if err != nil {
		t.Fatalf("cosign was not invoked: %v\n%s", err, res.output())
	}
	args := strings.Split(strings.TrimSpace(string(data)), "\n")
	want := map[string]string{
		"--certificate-identity":    "https://github.com/GrayCodeAI/rho/.github/workflows/release.yml@refs/tags/v" + installTestVersion,
		"--certificate-oidc-issuer": "https://token.actions.githubusercontent.com",
	}
	if len(args) == 0 || args[0] != "verify-blob" {
		t.Fatalf("cosign args = %q, want verify-blob first", args)
	}
	for i, a := range args {
		if a == "--certificate-identity-regexp" {
			t.Errorf("cosign called with a regexp identity; want the exact --certificate-identity")
		}
		if v, ok := want[a]; ok {
			if i+1 >= len(args) || args[i+1] != v {
				t.Errorf("%s = %q, want %q", a, args[min(i+1, len(args)-1)], v)
			}
			delete(want, a)
		}
		if a == "--bundle" && (i+1 >= len(args) || !strings.HasSuffix(args[i+1], "/checksums.txt.sigstore.json")) {
			t.Errorf("--bundle does not point at the downloaded checksums.txt.sigstore.json: %q", args)
		}
	}
	for flag := range want {
		t.Errorf("cosign not called with %s", flag)
	}
	if !strings.HasSuffix(args[len(args)-1], "/checksums.txt") {
		t.Errorf("cosign must verify checksums.txt, last arg = %q", args[len(args)-1])
	}
	if !strings.Contains(res.stdout, "signature: verified with cosign") {
		t.Errorf("summary does not report the signature verification:\n%s", res.stdout)
	}
}

func TestInstallScriptCosignFailuresAbort(t *testing.T) {
	t.Run("missing bundle", func(t *testing.T) {
		f := newInstallFixture(t)
		f.addRelease(installTestVersion, "linux", "amd64")
		if err := os.Remove(filepath.Join(f.releaseDir, "v"+installTestVersion, "checksums.txt.sigstore.json")); err != nil {
			t.Fatal(err)
		}
		f.withCosign(0)
		res := f.run("--version", installTestVersion, "--prefix", f.prefix)
		if res.exitCode == 0 || !strings.Contains(res.stderr, "no checksums.txt.sigstore.json signature bundle") {
			t.Fatalf("want refusal for an unsigned release, got exit %d\n%s", res.exitCode, res.output())
		}
	})
	t.Run("invalid signature", func(t *testing.T) {
		f := newInstallFixture(t)
		f.addRelease(installTestVersion, "linux", "amd64")
		f.withCosign(1)
		res := f.run("--version", installTestVersion, "--prefix", f.prefix)
		if res.exitCode == 0 || !strings.Contains(res.stderr, "cosign could not verify checksums.txt") {
			t.Fatalf("want refusal for a bad signature, got exit %d\n%s", res.exitCode, res.output())
		}
		if _, err := os.Stat(filepath.Join(f.prefix, "bin", "rho")); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("rho installed despite a failed signature check (stat err=%v)", err)
		}
	})
}

func TestInstallScriptChecksumFailuresAbort(t *testing.T) {
	t.Run("mismatch", func(t *testing.T) {
		f := newInstallFixture(t)
		name := f.addRelease(installTestVersion, "linux", "amd64")
		f.addAsset("v"+installTestVersion, name, makeTarGz(t, "rho", "tampered"))
		res := f.run("--version", installTestVersion, "--prefix", f.prefix)
		if res.exitCode == 0 || !strings.Contains(res.stderr, "SHA-256 mismatch") {
			t.Fatalf("want checksum mismatch failure, got exit %d\n%s", res.exitCode, res.output())
		}
	})
	t.Run("no entry", func(t *testing.T) {
		f := newInstallFixture(t)
		f.addRelease(installTestVersion, "linux", "amd64")
		f.addAsset("v"+installTestVersion, "checksums.txt", []byte(strings.Repeat("a", 64)+"  rho_0.3.0_linux_amd64Xtar.gz\n"))
		res := f.run("--version", installTestVersion, "--prefix", f.prefix)
		if res.exitCode == 0 || !strings.Contains(res.stderr, "no entry for rho_0.3.0_linux_amd64.tar.gz") {
			t.Fatalf("want missing-entry failure (no regex wildcard match), got exit %d\n%s", res.exitCode, res.output())
		}
	})
}

func TestInstallScriptResolvesLatestRelease(t *testing.T) {
	f := newInstallFixture(t)
	f.addRelease("0.3.1", "darwin", "arm64")
	f.setLatest("v0.3.1")
	f.env["FAKE_UNAME_S"] = "Darwin"
	f.env["FAKE_UNAME_M"] = "arm64"

	res := f.run("--prefix", f.prefix)
	f.requireInstalled(res, f.prefix, "0.3.1")
	calls := strings.Join(f.curlCalls(), "\n")
	if !strings.Contains(calls, "https://github.com/GrayCodeAI/rho/releases/download/v0.3.1/rho_0.3.1_darwin_arm64.tar.gz") {
		t.Errorf("unexpected download URLs:\n%s", calls)
	}
}

func TestInstallScriptRefusesPreRenameReleases(t *testing.T) {
	f := newInstallFixture(t)
	f.setLatest("v0.2.0")

	res := f.run("--prefix", f.prefix)
	if res.exitCode == 0 || !strings.Contains(res.stderr, "v0.2.0 predates the rename to rho") {
		t.Fatalf("want a clear refusal for the hawk-era v0.2.0, got exit %d\n%s", res.exitCode, res.output())
	}
	for _, call := range f.curlCalls() {
		if strings.Contains(call, "/releases/download/") {
			t.Errorf("downloaded an asset for a refused version: %s", call)
		}
	}
}

func TestInstallScriptFlagParsing(t *testing.T) {
	cases := []struct {
		name string
		args func(prefix string) []string
	}{
		{"version then prefix", func(p string) []string { return []string{"--version", installTestVersion, "--prefix", p} }},
		{"prefix then version", func(p string) []string { return []string{"--prefix", p, "--version", installTestVersion} }},
		{"equals forms", func(p string) []string { return []string{"--prefix=" + p, "--version=v" + installTestVersion} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newInstallFixture(t)
			f.addRelease(installTestVersion, "linux", "amd64")
			res := f.run(tc.args(f.prefix)...)
			f.requireInstalled(res, f.prefix, installTestVersion)
		})
	}

	t.Run("RHO_HOME and RHO_VERSION", func(t *testing.T) {
		f := newInstallFixture(t)
		f.addRelease(installTestVersion, "linux", "amd64")
		f.env["RHO_HOME"] = f.prefix
		f.env["RHO_VERSION"] = "v" + installTestVersion
		f.requireInstalled(f.run(), f.prefix, installTestVersion)
	})

	t.Run("unknown flag", func(t *testing.T) {
		f := newInstallFixture(t)
		res := f.run("--prefx", f.prefix)
		if res.exitCode != 2 || !strings.Contains(res.stderr, "unknown argument: --prefx") {
			t.Fatalf("want usage error exit 2, got exit %d\n%s", res.exitCode, res.output())
		}
	})

	t.Run("missing value", func(t *testing.T) {
		f := newInstallFixture(t)
		res := f.run("--version")
		if res.exitCode != 2 || !strings.Contains(res.stderr, "--version needs a value") {
			t.Fatalf("want usage error exit 2, got exit %d\n%s", res.exitCode, res.output())
		}
	})

	t.Run("help", func(t *testing.T) {
		f := newInstallFixture(t)
		res := f.run("--help")
		if res.exitCode != 0 || !strings.Contains(res.stdout, "Usage: install.sh") {
			t.Fatalf("want usage on stdout, got exit %d\n%s", res.exitCode, res.output())
		}
		if len(f.curlCalls()) != 0 {
			t.Errorf("--help made network calls: %q", f.curlCalls())
		}
	})
}

func TestInstallScriptRejectsBadInput(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(f *installFixture) []string
		wantErr string
	}{
		{"shell metacharacters in version", func(f *installFixture) []string {
			return []string{"--version", "0.3.0;touch /tmp/pwned", "--prefix", f.prefix}
		}, "invalid version"},
		{"unsupported architecture", func(f *installFixture) []string {
			f.env["FAKE_UNAME_M"] = "mips"
			return []string{"--version", installTestVersion, "--prefix", f.prefix}
		}, "unsupported architecture: mips"},
		{"unsupported os", func(f *installFixture) []string {
			f.env["FAKE_UNAME_S"] = "SunOS"
			return []string{"--version", installTestVersion, "--prefix", f.prefix}
		}, "unsupported operating system: sunos"},
		{"malformed repo override", func(f *installFixture) []string {
			f.env["RHO_INSTALL_REPO"] = "evil/../../x y"
			return []string{"--version", installTestVersion, "--prefix", f.prefix}
		}, "RHO_INSTALL_REPO must look like owner/repo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newInstallFixture(t)
			res := f.run(tc.setup(f)...)
			if res.exitCode == 0 || !strings.Contains(res.stderr, tc.wantErr) {
				t.Fatalf("want failure containing %q, got exit %d\n%s", tc.wantErr, res.exitCode, res.output())
			}
			if len(f.curlCalls()) != 0 {
				t.Errorf("made network calls before rejecting input: %q", f.curlCalls())
			}
		})
	}
}

func TestInstallScriptForkIdentityFollowsRepo(t *testing.T) {
	f := newInstallFixture(t)
	f.env["RHO_INSTALL_REPO"] = "example/rho-fork"
	f.addRelease(installTestVersion, "linux", "arm64")
	f.env["FAKE_UNAME_M"] = "aarch64"
	f.withCosign(0)

	res := f.run("--version", installTestVersion, "--prefix", f.prefix)
	f.requireInstalled(res, f.prefix, installTestVersion)
	data, err := os.ReadFile(f.cosignLog)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "https://github.com/example/rho-fork/.github/workflows/release.yml@refs/tags/v0.3.0\n") {
		t.Errorf("cosign identity does not follow RHO_INSTALL_REPO:\n%s", data)
	}
	if !strings.Contains(strings.Join(f.curlCalls(), "\n"), "https://github.com/example/rho-fork/releases/download/v0.3.0/") {
		t.Errorf("downloads did not use the fork: %q", f.curlCalls())
	}
}

func TestInstallScriptWindowsArchive(t *testing.T) {
	if _, err := exec.LookPath("unzip"); err != nil {
		t.Skipf("unzip not available on this host: %v", err)
	}
	f := newInstallFixture(t)
	f.addRelease(installTestVersion, "windows", "arm64")
	f.env["FAKE_UNAME_S"] = "MINGW64_NT-10.0-26100"
	f.env["FAKE_UNAME_M"] = "arm64"

	res := f.run("--version", installTestVersion, "--prefix", f.prefix)
	if res.exitCode != 0 {
		t.Fatalf("install.sh exit %d\n%s", res.exitCode, res.output())
	}
	for _, name := range []string{"rho-" + installTestVersion + ".exe", "rho.exe"} {
		if _, err := os.Stat(filepath.Join(f.prefix, "bin", name)); err != nil {
			t.Errorf("%s not installed: %v", name, err)
		}
	}
}

// fakeCurl serves https://github.com/<repo>/releases/download/<tag>/<asset>
// from $FAKE_RELEASE_DIR/<tag>/<asset> and the releases/latest API from
// $FAKE_RELEASE_DIR/latest.json, exiting 22 (curl -f) for anything missing.
const fakeCurl = `#!/bin/sh
out=""
url=""
while [ $# -gt 0 ]; do
  case "$1" in
    -o) out=$2; shift 2 ;;
    https://*) url=$1; shift ;;
    *) shift ;;
  esac
done
printf '%s\n' "$url" >> "$FAKE_CURL_LOG"
case "$url" in
  https://api.github.com/repos/*/releases/latest) src="$FAKE_RELEASE_DIR/latest.json" ;;
  https://github.com/*/releases/download/*) src="$FAKE_RELEASE_DIR/${url#*/releases/download/}" ;;
  *) echo "fake curl: unexpected URL $url" >&2; exit 3 ;;
esac
[ -f "$src" ] || { echo "curl: (22) The requested URL returned error: 404" >&2; exit 22; }
if [ -n "$out" ]; then cp "$src" "$out"; else cat "$src"; fi
`

const fakeUname = `#!/bin/sh
case "$1" in
  -s) printf '%s\n' "$FAKE_UNAME_S" ;;
  -m) printf '%s\n' "$FAKE_UNAME_M" ;;
  *) printf '%s\n' "$FAKE_UNAME_S" ;;
esac
`

func makeTarGz(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	files := []struct {
		name, body string
		mode       int64
	}{{name, content, 0o755}, {"README.md", "readme\n", 0o644}}
	for _, file := range files {
		if err := tw.WriteHeader(&tar.Header{Name: file.name, Mode: file.mode, Size: int64(len(file.body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(file.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func makeZip(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
