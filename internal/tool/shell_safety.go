package tool

import (
	"context"
	"fmt"
	"strings"
)

// validateShellCommand applies the full Bash safety stack to a shell command
// string. It is shared by BashTool and the persistent-terminal tools so that
// every path that hands a model-supplied string to a shell gets the same
// destructive-command, AST, and obfuscation checks.
//
// A nil error means the command passed the static safety layer. It does not
// imply the command is safe to run without a permission prompt.
func validateShellCommand(ctx context.Context, command string) error {
	if command == "" {
		return fmt.Errorf("command is required")
	}

	// Explore/plan hard gate: segment-aware read-only allowlist (PACK-02).
	if tc := GetToolContext(ctx); tc != nil && tc.ReadOnlyBash {
		if err := ExploreBashAllowed(command); err != nil {
			return fmt.Errorf("blocked (explore/plan read-only bash): %w", err)
		}
	}

	// Safety layer: block destructive commands before any execution.
	if IsDestructiveCommand(command) {
		return fmt.Errorf("blocked: destructive command pattern detected — %s", command)
	}

	// AST safety layer: walk the bash AST looking for nested dangers
	// (substitution bodies containing destructive commands, heredoc
	// bodies with eval/exec, process substitutions). This is the
	// second-pass safety check that catches what the regex layer
	// misses — for example, the regex layer flags `echo $(rm -rf /)`
	// because the outer string contains "rm -rf", but it does NOT flag
	// the safer-looking `echo $(date +%Y)`. The AST layer is the one
	// that actually checks the INNER content. The findings are
	// surfaced as a hard-block error so a future sub-agent turn cannot
	// build on top of a command that contains a nested destructive
	// command.
	astFindings := bashASTAnalyze(command)
	if len(astFindings) > 0 {
		// Format findings as a single error message.
		var parts []string
		for _, f := range astFindings {
			parts = append(parts, f.String())
		}
		return fmt.Errorf("blocked: AST safety layer flagged %d finding(s): %s", len(astFindings), strings.Join(parts, "; "))
	}

	// Normalize command to prevent trivial bypass of dangerous-command detection.
	normalized := normalizeCommand(command)

	// Hard block: always-dangerous patterns
	lower := strings.ToLower(normalized)
	for _, pat := range dangerousSubstrings {
		if strings.Contains(lower, pat) {
			return fmt.Errorf("blocked: dangerous command pattern detected")
		}
	}

	// Hard-block the most-dangerous suspicious patterns even when no
	// permission prompt is in scope (e.g. run_in_background=true skips the
	// human-in-the-loop approval). This is a strict subset of the
	// suspiciousPatterns list — it deliberately excludes
	// "writing to absolute paths" and "curl/wget" which are common in
	// legitimate agent tasks.
	if isHardDeny(command) {
		return fmt.Errorf("blocked: hard-deny pattern (e.g. eval/command-substitution) cannot run in hard-deny contexts like run_in_background — %s", command)
	}

	// Block zsh zmodload which enables dangerous modules
	if zmodloadRe.MatchString(command) {
		return fmt.Errorf("blocked: zmodload can enable dangerous zsh modules")
	}

	// Block process substitution
	if processSubstitutionRe.MatchString(command) {
		return fmt.Errorf("blocked: process substitution requires approval")
	}

	// Block IFS injection
	if ifsInjectionRe.MatchString(command) {
		return fmt.Errorf("blocked: IFS variable usage bypasses security validation")
	}

	// Block carriage return
	if strings.Contains(command, "\r") {
		return fmt.Errorf("blocked: carriage return can cause shell-quote/bash tokenization differential")
	}

	// Block /proc/*/environ access
	if procEnvironRe.MatchString(command) {
		return fmt.Errorf("blocked: /proc/*/environ access can expose environment variables")
	}
	if envDumpRe.MatchString(command) {
		return fmt.Errorf("blocked: dumping environment variables can expose API keys")
	}
	if rhoEnvReadRe.MatchString(command) {
		return fmt.Errorf("blocked: reading ~/.rho env files can expose API keys")
	}
	if apiKeyEchoRe.MatchString(command) {
		return fmt.Errorf("blocked: echoing API key environment variables is not allowed")
	}

	// Block heredoc in substitution (complex validation)
	if heredocSubstitutionRe.MatchString(command) {
		return fmt.Errorf("blocked: heredoc in command substitution requires approval")
	}

	// Block ANSI-C quoting
	if ansiCQuotingRe.MatchString(command) {
		return fmt.Errorf("blocked: ANSI-C quoting can hide dangerous characters")
	}

	// Block empty quote pairs before dash
	if emptyQuotePairRe.MatchString(command) {
		return fmt.Errorf("blocked: empty quote pair before dash can hide flags")
	}

	// Block consecutive quotes
	if consecutiveQuotesExecRe.MatchString(command) {
		return fmt.Errorf("blocked: consecutive quotes indicate obfuscation attempt")
	}

	return nil
}

// validateTerminalInput applies the destructive-command subset of the shell
// safety stack to input sent to a persistent terminal. It deliberately omits
// the interactive-hostile checks (carriage return, quote obfuscation) that
// would break raw keystroke input, while still blocking destructive commands,
// nested AST dangers, and hard-deny patterns.
func validateTerminalInput(input string) error {
	if strings.TrimSpace(input) == "" {
		return nil
	}
	if IsDestructiveCommand(input) {
		return fmt.Errorf("blocked: destructive command pattern detected — %s", input)
	}
	if findings := bashASTAnalyze(input); len(findings) > 0 {
		var parts []string
		for _, f := range findings {
			parts = append(parts, f.String())
		}
		return fmt.Errorf("blocked: AST safety layer flagged %d finding(s): %s", len(findings), strings.Join(parts, "; "))
	}
	lower := strings.ToLower(normalizeCommand(input))
	for _, pat := range dangerousSubstrings {
		if strings.Contains(lower, pat) {
			return fmt.Errorf("blocked: dangerous command pattern detected")
		}
	}
	if isHardDeny(input) {
		return fmt.Errorf("blocked: hard-deny pattern (e.g. eval/command-substitution) is not allowed")
	}
	return nil
}
