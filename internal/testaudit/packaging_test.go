package testaudit

import (
	"strings"
	"testing"
)

// parseSystemdUnit returns directive -> values for a systemd unit file,
// joining backslash-continued lines and skipping comments.
func parseSystemdUnit(t *testing.T, data string) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	var logical []string
	var cur strings.Builder
	for _, line := range strings.Split(data, "\n") {
		trimmed := strings.TrimSpace(line)
		if cur.Len() == 0 && (trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";")) {
			continue
		}
		if strings.HasSuffix(trimmed, "\\") {
			cur.WriteString(strings.TrimSuffix(trimmed, "\\") + " ")
			continue
		}
		cur.WriteString(trimmed)
		logical = append(logical, cur.String())
		cur.Reset()
	}
	for _, l := range logical {
		if strings.HasPrefix(l, "[") {
			continue
		}
		key, value, ok := strings.Cut(l, "=")
		if !ok {
			t.Fatalf("malformed unit line %q", l)
		}
		out[key] = append(out[key], value)
	}
	return out
}

func TestSystemdUnitStateIsReachableUnderHardening(t *testing.T) {
	unit := parseSystemdUnit(t, string(readRepoFile(t, "packaging/systemd/rho-daemon.service")))
	one := func(key string) string {
		t.Helper()
		if len(unit[key]) != 1 {
			t.Fatalf("want exactly one %s=, got %q", key, unit[key])
		}
		return unit[key][0]
	}

	// In a system unit %h is always /root (systemd.unit(5)), and
	// ProtectHome=true makes /home, /root and /run/user inaccessible, so no
	// path the daemon needs may live there.
	if one("ProtectHome") == "true" {
		for _, key := range []string{"ReadWritePaths", "ReadOnlyPaths", "BindPaths", "BindReadOnlyPaths", "WorkingDirectory", "Environment"} {
			for _, v := range unit[key] {
				for _, hidden := range []string{"%h", "/home", "/root", "/run/user"} {
					if strings.Contains(v, hidden) {
						t.Errorf("%s=%s points under %s, which ProtectHome=true hides", key, v, hidden)
					}
				}
			}
		}
	}

	if one("User") == "" || one("User") == "root" {
		t.Error("the daemon must run as a dedicated unprivileged User=")
	}
	state := one("StateDirectory")
	env := map[string]string{}
	for _, e := range unit["Environment"] {
		k, v, _ := strings.Cut(e, "=")
		env[k] = v
	}
	if want := "/var/lib/" + state; env["HOME"] != want || !strings.HasPrefix(env["RHO_STATE_DIR"], want) || !strings.HasPrefix(env["RHO_CONFIG_DIR"], want) {
		t.Errorf("HOME/RHO_STATE_DIR/RHO_CONFIG_DIR must live in the StateDirectory %s, got %v", want, env)
	}
	if _, ok := env["RHO_DAEMON_API_KEY"]; ok {
		t.Error("do not set RHO_DAEMON_API_KEY inline; load it from the EnvironmentFile")
	}

	exec := one("ExecStart")
	if strings.Contains(exec, "--api-key") {
		t.Errorf("ExecStart passes the API key on the command line (visible in ps): %s", exec)
	}
	if !strings.Contains(exec, " daemon start ") || !strings.Contains(exec, "--host 127.0.0.1") {
		t.Errorf("ExecStart should start the daemon on loopback by default: %s", exec)
	}

	for _, doc := range strings.Fields(one("Documentation")) {
		if !strings.HasPrefix(doc, "https://github.com/GrayCodeAI/rho/") {
			t.Errorf("Documentation=%s must point at the rho repository", doc)
		}
	}
}
