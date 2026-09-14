package token

import (
	"strings"
	"testing"
)

func TestSecretDetectorDetectsCommonPatterns(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"aws access key", "key=AKIAIOSFODNN7EXAMPLE", "AWS Access Key"},
		{"github token", "token: ghp_abcdefghijklmnopqrstuvwxyz0123456789", "GitHub Token"},
		{"anthropic key", "sk-ant-api03-abcdefghijklmnopqrstuvwxyz", "Anthropic API Key"},
		{"slack token", "xoxb-1234567890-abcdefghij", "Slack Bot Token"},
		{"private key", "-----BEGIN RSA PRIVATE KEY-----\nMIIE\n-----END RSA PRIVATE KEY-----", "RSA Private Key"},
		{"generic api key", `api_key = "abcdefghijklmnopqrstuvwxyz"`, "Generic API Key"},
	}
	det := DefaultSecretDetector()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			matches := det.DetectSecrets(tc.text)
			found := false
			for _, m := range matches {
				if m.Type == tc.want {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("DetectSecrets(%q) did not find %q; got %+v", tc.text, tc.want, matches)
			}
		})
	}
}

func TestSecretDetectorNoFalsePositiveOnProse(t *testing.T) {
	det := DefaultSecretDetector()
	matches := det.DetectSecrets("The quick brown fox jumps over the lazy dog.")
	if len(matches) != 0 {
		t.Errorf("unexpected matches on plain prose: %+v", matches)
	}
}

func TestSecretDetectorRedact(t *testing.T) {
	det := DefaultSecretDetector()
	text := "key=AKIAIOSFODNN7EXAMPLE and more"
	out := det.RedactSecrets(text)
	if strings.Contains(out, "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("RedactSecrets leaked the secret: %q", out)
	}
	if !strings.Contains(out, "[REDACTED:") {
		t.Errorf("RedactSecrets did not insert a marker: %q", out)
	}
}

func TestSecretDetectorRedactNoMatch(t *testing.T) {
	det := DefaultSecretDetector()
	text := "nothing sensitive here"
	if got := det.RedactSecrets(text); got != text {
		t.Errorf("RedactSecrets changed clean text: %q", got)
	}
}

func TestSecretMatchMasked(t *testing.T) {
	det := DefaultSecretDetector()
	matches := det.DetectSecrets("AKIAIOSFODNN7EXAMPLE")
	if len(matches) == 0 {
		t.Fatal("expected a match")
	}
	m := matches[0]
	if m.Masked == "" || m.Masked == m.Value {
		t.Errorf("Masked = %q, want a redacted form of %q", m.Masked, m.Value)
	}
	if !strings.Contains(m.Masked, "*") {
		t.Errorf("Masked = %q, want asterisks", m.Masked)
	}
}

func TestMaskSecretPrivateKey(t *testing.T) {
	value := "-----BEGIN RSA PRIVATE KEY-----\nSECRETBODY\n-----END RSA PRIVATE KEY-----"
	got := MaskSecret(value, "RSA Private Key")
	if strings.Contains(got, "SECRETBODY") {
		t.Errorf("MaskSecret leaked the key body: %q", got)
	}
	if !strings.Contains(got, "-----BEGIN RSA PRIVATE KEY-----") {
		t.Errorf("MaskSecret dropped the header: %q", got)
	}
}

func TestNewSecretDetectorIndependent(t *testing.T) {
	a := NewSecretDetector()
	b := NewSecretDetector()
	if a == b {
		t.Error("NewSecretDetector returned the same instance")
	}
	first := DefaultSecretDetector()
	second := DefaultSecretDetector()
	if first != second {
		t.Error("DefaultSecretDetector should be a singleton")
	}
}
