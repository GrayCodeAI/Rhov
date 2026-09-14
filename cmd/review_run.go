package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	hawkconfig "github.com/GrayCodeAI/hawk/internal/config"
	reviewcontracts "github.com/GrayCodeAI/hawk/internal/contracts/review"
	contracts "github.com/GrayCodeAI/hawk/internal/contracts/types"
	"github.com/GrayCodeAI/hawk/internal/engine"
	"github.com/GrayCodeAI/hawk/internal/types"
	"github.com/GrayCodeAI/hawk/internal/ui/icons"
	"github.com/spf13/cobra"
)

var (
	reviewRunBackground bool
	reviewRunModel      string
	reviewRunConcerns   string
	reviewRunTimeout    time.Duration
)

var reviewRunCmd = &cobra.Command{
	Use:   "run <sha>",
	Short: "Review a specific commit",
	Args:  cobra.ExactArgs(1),
	RunE:  runReviewRun,
}

func init() {
	reviewRunCmd.Flags().BoolVar(&reviewRunBackground, "background", false, "Run silently (for hook use)")
	reviewRunCmd.Flags().StringVar(&reviewRunModel, "model", "", "LLM model for review")
	reviewRunCmd.Flags().StringVar(&reviewRunConcerns, "concerns", "", "Comma-separated concerns")
	reviewRunCmd.Flags().DurationVar(&reviewRunTimeout, "timeout", 3*time.Minute, "Review timeout")
	reviewCmd.AddCommand(reviewRunCmd)
}

func runReviewRun(_ *cobra.Command, args []string) error {
	sha := args[0]

	// Resolve short SHA to full.
	if len(sha) < 40 {
		out, err := exec.CommandContext(context.Background(), "git", "rev-parse", sha).Output() // #nosec G204 -- fixed command 'git' with args, not user-controlled binary
		if err == nil {
			sha = strings.TrimSpace(string(out))
		}
	}

	projectDir, _ := os.Getwd()
	store, err := OpenReviewStore(projectDir)
	if err != nil {
		return silentErr(err, "open review store")
	}
	defer func() { _ = store.Close() }()

	// Check if already reviewed.
	existing, getErr := store.GetBySHA(sha)
	if getErr != nil {
		return silentErr(getErr, "load existing review")
	}
	if existing != nil && existing.Status != ReviewStatusFailed {
		if !reviewRunBackground {
			fmt.Printf("%s\n", auditTint("Commit "+sha[:8]+" already reviewed (status: ", textMuted)+auditTint(string(existing.Status), reviewStatusColor(existing.Status))+auditTint(")", textMuted))
		}
		return nil
	}

	// Create pending record.
	id, err := store.Create(sha)
	if err != nil {
		return silentErr(err, "create review record")
	}
	if err := store.SetStatus(id, ReviewStatusRunning); err != nil {
		return silentErr(err, "mark review running")
	}

	// Get commit diff.
	diff, err := getCommitDiff(sha)
	if err != nil {
		if statusErr := store.SetStatus(id, ReviewStatusFailed); statusErr != nil {
			return silentErr(statusErr, "mark review failed")
		}
		return silentErr(err, "get commit diff")
	}
	if strings.TrimSpace(diff) == "" {
		if statusErr := store.SetStatus(id, ReviewStatusPassed); statusErr != nil {
			return silentErr(statusErr, "mark review passed")
		}
		if !reviewRunBackground {
			fmt.Println(auditTint("Empty diff — nothing to review.", textMuted))
		}
		return nil
	}

	// Live progress for the slow review stages. TTY-aware: animates the active
	// step on a terminal, prints clean static lines when piped, and stays
	// silent in background/hook mode. The deferred Abort guarantees the
	// spinner goroutine never leaks past an error return.
	var prog *CLIProgress
	if !reviewRunBackground {
		prog = NewCLIProgress("Review", []string{"Building model", "Reviewing code", "Saving results"})
		defer prog.Abort()
	}
	step := func(i int) {
		if prog != nil {
			prog.StartStep(i)
		}
	}
	done := func(i int) {
		if prog != nil {
			prog.CompleteStep(i)
		}
	}
	finish := func() {
		if prog != nil {
			prog.Done()
		}
	}

	// Resolve the provider through Hawk's Eyrie engine boundary.
	ctx := context.Background()
	selection := hawkconfig.EffectiveSelection(ctx, hawkconfig.SelectionOptions{
		ProviderOverride: strings.TrimSpace(provider),
		ModelOverride:    strings.TrimSpace(reviewRunModel),
	})
	step(0)
	chatProvider, providerID, err := engine.BuildChatProvider(ctx, selection, strings.TrimSpace(provider))
	if err != nil {
		if statusErr := store.SetStatus(id, ReviewStatusFailed); statusErr != nil {
			return silentErr(statusErr, "mark review failed")
		}
		return silentErr(fmt.Errorf("resolve engine transport: %w", err), "init provider")
	}
	done(0)
	step(1)

	if reviewRunTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, reviewRunTimeout)
		defer cancel()
	}

	// Run Hawk's own multi-concern review pipeline through the provider.
	concerns := DefaultConcerns()
	if strings.TrimSpace(reviewRunConcerns) != "" {
		wanted := map[string]bool{}
		for _, c := range strings.Split(reviewRunConcerns, ",") {
			if c = strings.TrimSpace(c); c != "" {
				wanted[c] = true
			}
		}
		var filtered []ReviewConcern
		for _, c := range concerns {
			if wanted[c.Name] {
				filtered = append(filtered, c)
			}
		}
		if len(filtered) > 0 {
			concerns = filtered
		}
	}

	model := selection.Model
	if reviewRunModel != "" {
		model = reviewRunModel
	}
	chatFn := func(chatCtx context.Context, prompt string) (string, error) {
		resp, chatErr := chatProvider.Chat(chatCtx, []types.EyrieMessage{{Role: "user", Content: prompt}}, types.ChatOptions{
			Provider: providerID,
			Model:    model,
		})
		if chatErr != nil {
			return "", chatErr
		}
		if resp == nil {
			return "", fmt.Errorf("review model returned no response")
		}
		return resp.Content, nil
	}

	findings, report := RunReviewPipeline(ctx, []string{diff}, concerns, chatFn)
	result := reviewResultFromFindings(findings, report, len(concerns))

	// Determine status based on findings.
	status := ReviewStatusPassed
	if len(result.Findings) > 0 {
		status = ReviewStatusOpen
	}

	done(1)
	step(2)

	if err := store.Update(id, status, result); err != nil {
		return silentErr(err, "store result")
	}
	done(2)
	finish()

	if !reviewRunBackground {
		printReviewSummary(sha, result)
	}
	return nil
}

// reviewResultFromFindings converts the pipeline's findings into the neutral
// review contract, populating stats so downstream surfaces (status, show,
// graph observations) see a complete result.
func reviewResultFromFindings(findings []ReviewFinding, report string, concerns int) *reviewcontracts.Result {
	bySeverity := map[contracts.Severity]int{}
	byConcern := map[string]int{}
	out := make([]reviewcontracts.Finding, 0, len(findings))
	var confidenceSum float64
	for _, f := range findings {
		sev, err := contracts.ParseSeverityStrict(f.Severity)
		if err != nil {
			sev = contracts.SeverityInfo
		}
		bySeverity[sev]++
		byConcern[f.Concern]++
		confidenceSum += 0.8
		out = append(out, reviewcontracts.Finding{
			Concern:    f.Concern,
			Severity:   sev,
			File:       f.File,
			Line:       f.Line,
			Message:    f.Message,
			Fix:        f.Fix,
			Confidence: 0.8,
		})
	}
	avg := 0.0
	if len(out) > 0 {
		avg = confidenceSum / float64(len(out))
	}
	return &reviewcontracts.Result{
		Findings: out,
		Report:   report,
		Stats: reviewcontracts.Stats{
			FindingsTotal:     len(out),
			BySeverity:        bySeverity,
			ByConcern:         byConcern,
			AverageConfidence: avg,
		},
	}
}

func getCommitDiff(sha string) (string, error) {
	// For the first commit, diff against empty tree.
	out, err := exec.CommandContext(context.Background(), "git", "diff-tree", "-p", sha).Output() // #nosec G204 -- fixed git executable
	if err != nil {
		// Fallback: diff against parent.
		out, err = exec.CommandContext(context.Background(), "git", "diff", sha+"^", sha).Output() // #nosec G204 -- fixed command 'git' with args, not user-controlled binary
		if err != nil {
			return "", fmt.Errorf("git diff for %s: %w", sha[:8], err)
		}
	}
	return string(out), nil
}

func printReviewSummary(sha string, result *reviewcontracts.Result) {
	if len(result.Findings) == 0 {
		fmt.Printf("%s %s — no issues found\n",
			auditTint(icons.CheckBold(), doneGreen),
			auditTint(sha[:8], textPrimary))
		return
	}
	maxSev := result.MaxSeverity()
	fmt.Printf("%s %s — %d findings (max severity: %s)\n",
		auditTint(icons.Alert(), errorCoral),
		auditTint(sha[:8], textPrimary),
		len(result.Findings),
		auditTint(maxSev.String(), reviewSeverityColor(maxSev)))
	for _, f := range result.Findings {
		fmt.Printf("  %s %s\n",
			auditTint(fmt.Sprintf("[%s]", f.Severity.String()), reviewSeverityColor(f.Severity)),
			auditTint(fmt.Sprintf("%s:%d", f.File, f.Line), textPrimary)+auditTint(" — "+f.Message, textMuted))
	}
}

// silentErr suppresses errors in background mode, prints otherwise.
func silentErr(err error, context string) error {
	if reviewRunBackground {
		return nil
	}
	return fmt.Errorf("%s: %w", context, err)
}
