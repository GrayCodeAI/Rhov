package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillsLockRoundTrip(t *testing.T) {
	lockPath := SkillsLockPath("user")

	lock, err := LoadSkillsLock("user")
	if err != nil {
		t.Fatalf("load missing lock: %v", err)
	}
	if len(lock.Skills) != 0 {
		t.Fatalf("expected empty lock, got %+v", lock.Skills)
	}

	lock.Set("go-review", SkillsLockEntry{
		Source:       "GrayCodeAI/graycode-skills",
		SourceType:   "github",
		SkillPath:    "skills/go-review/SKILL.md",
		Commit:       "abc123",
		ComputedHash: HashSkillContent([]byte("# hi")),
	})
	if err := lock.Save("user"); err != nil {
		t.Fatalf("save: %v", err)
	}

	data, err := os.ReadFile(lockPath) // #nosec G304 -- test-owned temp path
	if err != nil {
		t.Fatalf("lockfile not written: %v", err)
	}
	if data[len(data)-1] != '\n' {
		t.Fatal("lockfile should end with newline")
	}

	reloaded, err := LoadSkillsLock("user")
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	got, ok := reloaded.Skills["go-review"]
	if !ok || got.Commit != "abc123" || got.SourceType != "github" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if reloaded.Version != skillsLockVersion {
		t.Fatalf("version should be normalized on load, got %d", reloaded.Version)
	}
}

func TestSkillsLockDelete(t *testing.T) {
	l := &SkillsLock{Skills: map[string]SkillsLockEntry{}}
	l.Set("a", SkillsLockEntry{ComputedHash: "x"})
	if !l.Delete("a") {
		t.Fatal("Delete should report removal")
	}
	if l.Delete("a") {
		t.Fatal("second Delete should be false")
	}
	if l.Delete("never-there") {
		t.Fatal("Delete of absent skill should be false")
	}
}

func TestHashSkillContentStable(t *testing.T) {
	h1 := HashSkillContent([]byte("same"))
	h2 := HashSkillContent([]byte("same"))
	if h1 != h2 || len(h1) != 64 {
		t.Fatalf("hash should be stable sha256 hex, got %q vs %q", h1, h2)
	}
	if HashSkillContent([]byte("other")) == h1 {
		t.Fatal("different content must hash differently")
	}
}

func TestSkillsLockVerify(t *testing.T) {
	dir := t.TempDir()
	writeSkill := func(name, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(dir, name), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name, "SKILL.md"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeSkill("good", "# good")
	writeSkill("tampered", "# tampered")

	lock := &SkillsLock{Skills: map[string]SkillsLockEntry{
		"good":     {ComputedHash: HashSkillContent([]byte("# good"))},
		"tampered": {ComputedHash: HashSkillContent([]byte("# original"))},
		"missing":  {ComputedHash: HashSkillContent([]byte("# gone"))},
		"unpinned": {},
	}}
	got := lock.Verify(dir)
	if len(got) != 3 {
		t.Fatalf("Verify() = %v, want 3 drift lines", got)
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{`"tampered"`, `"missing"`, `"unpinned"`} {
		if !strings.Contains(joined, want) {
			t.Errorf("drift lines %v missing %s", got, want)
		}
	}
	if strings.Contains(joined, `"good"`) {
		t.Errorf("matching skill reported as drift: %v", got)
	}
}

func TestSkillsLockVerifyEmpty(t *testing.T) {
	if got := (&SkillsLock{Skills: map[string]SkillsLockEntry{}}).Verify(t.TempDir()); len(got) != 0 {
		t.Fatalf("empty lock Verify() = %v, want none", got)
	}
	var nilLock *SkillsLock
	if got := nilLock.Verify(t.TempDir()); len(got) != 0 {
		t.Fatalf("nil lock Verify() = %v, want none", got)
	}
}
