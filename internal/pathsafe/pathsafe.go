// Package pathsafe centralizes sensitive-path policy so low-level packages
// (for example config) can consult it without depending on the large tool
// package. It blocks reads and writes to credential-bearing files such as
// ~/.ssh keys, cloud credentials, and rho's own provider/env files.
package pathsafe

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/GrayCodeAI/rho/internal/env"
	"github.com/GrayCodeAI/rho/internal/home"
	"github.com/GrayCodeAI/rho/internal/storage"
)

// BlockedPathSuffixes are path suffixes that should never be read or written.
var BlockedPathSuffixes = []string{
	"/.ssh/id_rsa",
	"/.ssh/id_ed25519",
	"/.ssh/id_ecdsa",
	"/.ssh/id_dsa",
	"/.ssh/config",
	"/.ssh/known_hosts",
	"/.ssh/authorized_keys",
	"/.aws/credentials",
}

// BlockedBasenames are file basenames that are blocked regardless of directory.
var BlockedBasenames = []string{
	".env",
	"credentials.json",
	".npmrc",
	".netrc",
	".pgpass",
	"kubeconfig",
	"token.json",
	"service-account.json",
	"credentials.yaml",
	"credentials.yml",
	"credentials.xml",
	"secrets.txt",
	"secrets.yaml",
	"secrets.yml",
	"secrets.json",
	".git-credentials",
	".htpasswd",
	"id_rsa",
	"id_ed25519",
	"id_ecdsa",
	"id_dsa",
}

// ResolvePath returns the absolute, symlink-resolved path.
// If resolution fails it falls back to filepath.Abs.
func ResolvePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// If the file does not exist yet (Write), resolve the parent.
		dir := filepath.Dir(abs)
		base := filepath.Base(abs)
		if rdir, err2 := filepath.EvalSymlinks(dir); err2 == nil {
			return filepath.Join(rdir, base), nil
		}
		return abs, nil
	}
	return resolved, nil
}

func matchesResolvedPath(cleanPath, candidate string) bool {
	resolved := candidate
	if canonical, err := ResolvePath(candidate); err == nil {
		resolved = canonical
	}
	return cleanPath == filepath.Clean(resolved)
}

// IsSensitivePath returns a non-empty reason when path points to a file that
// should be blocked for security. The path is cleaned and, when possible,
// resolved through symlinks before checking.
func IsSensitivePath(path string) string {
	// Resolve to absolute + follow symlinks when possible, including a
	// symlinked parent for a file that does not exist yet (the Write case).
	resolved := path
	if canonical, err := ResolvePath(path); err == nil {
		resolved = canonical
	}
	clean := filepath.Clean(resolved)

	homeDir := home.MustDir()

	if homeDir != "" {
		rhoProv := filepath.Join(homeDir, ".rho", "provider.json")
		if clean == rhoProv {
			return "access to ~/.rho/provider.json is blocked for security (API credentials)"
		}
		rhoEnv := filepath.Join(homeDir, ".rho", "env")
		if clean == rhoEnv {
			return "access to ~/.rho/env is blocked for security (API keys)"
		}
		rhoDotEnv := filepath.Join(homeDir, ".rho", ".env")
		if clean == rhoDotEnv {
			return "access to ~/.rho/.env is blocked for security (API keys)"
		}
	}

	if matchesResolvedPath(clean, storage.ProviderConfigPath()) {
		return "access to provider.json is blocked for security (API credentials)"
	}

	if cfgDir := strings.TrimSpace(env.Getenv("RHO_CONFIG_DIR")); cfgDir != "" {
		customEnv := filepath.Join(cfgDir, "env")
		if matchesResolvedPath(clean, customEnv) {
			return "access to rho env file is blocked for security (API keys)"
		}
		customDotEnv := filepath.Join(cfgDir, ".env")
		if matchesResolvedPath(clean, customDotEnv) {
			return "access to rho .env is blocked for security (API keys)"
		}
	}

	// Check suffix-based blocks (e.g. ~/.ssh/*)
	for _, suffix := range BlockedPathSuffixes {
		blocked := filepath.Join(homeDir, suffix[1:]) // strip leading /
		if clean == blocked {
			return fmt.Sprintf("access to %s is blocked for security", suffix)
		}
	}

	// ~/.ssh/* catch-all — block everything inside ~/.ssh
	if homeDir != "" {
		sshDir := filepath.Join(homeDir, ".ssh")
		if strings.HasPrefix(clean, sshDir+string(filepath.Separator)) || clean == sshDir {
			return "access to ~/.ssh is blocked for security"
		}
	}

	// ~/.env
	if homeDir != "" && clean == filepath.Join(homeDir, ".env") {
		return "access to ~/.env is blocked for security"
	}

	// Basename checks — blocks */.env and */credentials.json everywhere,
	// plus common .env variants (.env.local, .env.production, .env.backup, etc.)
	base := filepath.Base(clean)
	for _, b := range BlockedBasenames {
		if base == b {
			return fmt.Sprintf("access to %s files is blocked for security", b)
		}
	}
	// Block any file starting with ".env" (catches .env.local, .env.production, .env.backup, etc.)
	if strings.HasPrefix(base, ".env") && base != ".envrc" {
		return fmt.Sprintf("access to %s files is blocked for security", base)
	}

	return ""
}
