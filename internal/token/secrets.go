package token

import (
	"regexp"
	"sort"
	"strings"
	"sync"
)

// SecretMatch represents a detected secret within text.
type SecretMatch struct {
	Type     string
	Value    string
	Masked   string
	StartPos int
	EndPos   int
	// Verified reports whether a live-credential check confirmed this secret
	// is active. nil means verification was not attempted.
	Verified *bool
}

type secretPattern struct {
	Type    string
	Pattern *regexp.Regexp
}

// SecretDetector detects and redacts secrets from text using compiled regex
// patterns.
type SecretDetector struct {
	patterns []secretPattern
}

var (
	defaultDetector     *SecretDetector
	defaultDetectorOnce sync.Once
)

// DefaultSecretDetector returns the singleton SecretDetector instance with all
// built-in patterns. It is safe for concurrent use.
func DefaultSecretDetector() *SecretDetector {
	defaultDetectorOnce.Do(func() {
		defaultDetector = NewSecretDetector()
	})
	return defaultDetector
}

// NewSecretDetector creates a SecretDetector with all built-in patterns.
func NewSecretDetector() *SecretDetector {
	return &SecretDetector{patterns: compileSecretPatterns()}
}

func compileSecretPatterns() []secretPattern {
	raw := []struct {
		Type    string
		Pattern string
	}{
		{"AWS Access Key", `\b((?:AKIA|ASIA)[A-Z0-9]{16})\b`},
		{"AWS Secret Key", `(?i)(?:aws_secret_access_key|aws_secret)\s*[:=]\s*['"]?([A-Za-z0-9/+=]{40})['"]?`},
		{"GitHub Token", `\b(gh[oprs]_[A-Za-z0-9_]{36,})\b`},
		{"GitHub Fine-grained Token", `\b(github_pat_[A-Za-z0-9_]{22,})\b`},
		{"GitLab Token", `\b(glpat-[A-Za-z0-9_-]{20,})\b`},
		{"Slack Bot Token", `\b(xoxb-[A-Za-z0-9-]{10,})\b`},
		{"Slack User Token", `\b(xoxp-[A-Za-z0-9-]{10,})\b`},
		{"Slack Token", `\b(xox[bprs]-[A-Za-z0-9-]{10,})\b`},
		{"Slack Webhook", `https://hooks\.slack\.com/services/T[A-Z0-9]+/B[A-Z0-9]+/[A-Za-z0-9]+`},
		{"Google API Key", `\b(AIza[A-Za-z0-9_-]{35})\b`},
		{"Google OAuth", `\b([0-9]+-[A-Za-z0-9_]{32}\.apps\.googleusercontent\.com)\b`},
		{"Stripe Secret Key", `\b(sk_(?:live|test)_[A-Za-z0-9]{24,})\b`},
		{"Stripe Publishable Key", `\b(pk_(?:live|test)_[A-Za-z0-9]{24,})\b`},
		{"OpenAI API Key", `\b(sk-[A-Za-z0-9]{20,})\b`},
		{"Anthropic API Key", `\b(sk-ant-[A-Za-z0-9_-]{20,})\b`},
		{"Azure AD Client Secret", `(?i)(?:client_secret|clientsecret)\s*[:=]\s*([A-Za-z0-9~._-]{34,})`},
		{"JWT Token", `\b(eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*)\b`},
		{"RSA Private Key", `(-----BEGIN RSA PRIVATE KEY-----[\s\S]*?-----END RSA PRIVATE KEY-----)`},
		{"EC Private Key", `(-----BEGIN EC PRIVATE KEY-----[\s\S]*?-----END EC PRIVATE KEY-----)`},
		{"OpenSSH Private Key", `(-----BEGIN OPENSSH PRIVATE KEY-----[\s\S]*?-----END OPENSSH PRIVATE KEY-----)`},
		{"Private Key", `(-----BEGIN (?:RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----[\s\S]*?-----END (?:RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----)`},
		{"SendGrid API Key", `\b(SG\.[A-Za-z0-9_-]{22,}\.[A-Za-z0-9_-]{43,})\b`},
		{"Twilio Account SID", `\b(AC[a-f0-9]{32})\b`},
		{"Heroku API Key", `(?i)heroku[_-]?api[_-]?key\s*[:=]\s*['"]?([a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})['"]?`},
		{"DigitalOcean Token", `\b(dop_v1_[a-f0-9]{64})\b`},
		{"npm Token", `\b(npm_[A-Za-z0-9]{36,})\b`},
		{"PyPI Token", `\b(pypi-[A-Za-z0-9_-]{50,})\b`},
		{"Docker Registry Token", `(?i)"auth"\s*:\s*"([A-Za-z0-9+/=]{20,})"`},
		{"Generic API Key", `(?i)(?:api_key|apikey|api-key|access_token|auth_token|secret_key)\s*[:=]\s*['"]?([A-Za-z0-9_\-]{20,})['"]?`},
		{"Generic Password", `(?i)(?:password|passwd|pwd)\s*[:=]\s*['"]?([^\s'"]{8,})['"]?`},
		{"Generic Secret", `(?i)(?:secret|client_secret)\s*[:=]\s*['"]?([^\s'"]{8,})['"]?`},
		{"Database Connection String", `(?i)((?:mongodb|postgres|mysql|redis|amqp|mssql)://[^\s]+)`},
		{"Bearer Token", `(?i)bearer\s+([A-Za-z0-9_\-\.]+)`},
	}

	patterns := make([]secretPattern, 0, len(raw))
	for _, r := range raw {
		compiled, err := regexp.Compile(r.Pattern)
		if err != nil {
			continue
		}
		patterns = append(patterns, secretPattern{Type: r.Type, Pattern: compiled})
	}
	return patterns
}

// DetectSecrets finds all secrets in the given text.
func (sd *SecretDetector) DetectSecrets(text string) []SecretMatch {
	var matches []SecretMatch
	for _, p := range sd.patterns {
		for _, loc := range p.Pattern.FindAllStringSubmatchIndex(text, -1) {
			groupStart, groupEnd := loc[0], loc[1]
			if len(loc) >= 4 && loc[2] >= 0 {
				groupStart, groupEnd = loc[2], loc[3]
			}
			value := text[groupStart:groupEnd]
			matches = append(matches, SecretMatch{
				Type:     p.Type,
				Value:    value,
				Masked:   maskSecretValue(value, p.Type),
				StartPos: groupStart,
				EndPos:   groupEnd,
			})
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].StartPos < matches[j].StartPos })
	return matches
}

// RedactSecrets replaces detected secrets with [REDACTED] markers.
func (sd *SecretDetector) RedactSecrets(text string) string {
	matches := sd.DetectSecrets(text)
	if len(matches) == 0 {
		return text
	}

	var sb strings.Builder
	lastIdx := 0
	for _, m := range matches {
		if m.EndPos <= lastIdx {
			continue
		}
		start := m.StartPos
		if start < lastIdx {
			start = lastIdx
		}
		sb.WriteString(text[lastIdx:start])
		sb.WriteString("[REDACTED:")
		sb.WriteString(m.Type)
		sb.WriteString("]")
		lastIdx = m.EndPos
	}
	sb.WriteString(text[lastIdx:])
	return sb.String()
}

// MaskSecret masks a secret value for safe display.
func MaskSecret(value, secretType string) string {
	return maskSecretValue(value, secretType)
}

func maskSecretValue(value, secretType string) string {
	if strings.Contains(value, "-----BEGIN") && strings.Contains(value, "-----END") {
		lines := strings.SplitN(value, "\n", 2)
		if len(lines) == 2 {
			return lines[0] + "\n[REDACTED]"
		}
		return "[REDACTED]"
	}
	return maskString(value)
}

func maskString(s string) string {
	if len(s) <= 8 {
		return strings.Repeat("*", len(s))
	}
	return s[:4] + strings.Repeat("*", len(s)-8) + s[len(s)-4:]
}
