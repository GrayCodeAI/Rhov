package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"slices"
	"strings"
	"time"
	"unicode"

	cloud "github.com/GrayCodeAI/rho/internal/platform/cloud"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// cloudEndpointFlagHelp documents the endpoint precedence shared by the
// commands that create a new GrayCode Cloud connection.
const cloudEndpointFlagHelp = "GrayCode Cloud API endpoint (default: $" + cloud.EndpointEnv + ", else " + cloud.DefaultEndpoint + ")"

// loadCloudClient loads the saved GrayCode Cloud connection; tests replace it.
var loadCloudClient = cloud.LoadClient

var cloudCmd = &cobra.Command{Use: "cloud", Short: "Manage optional Rho Cloud synchronization"}

func newCloudConnectCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "connect",
		Short: "Connect this Rho device to Rho Cloud",
		Long: `Save an existing GrayCode Cloud device connection. Prefer "rho cloud login",
which needs no token. The device token is read with --token-stdin or from a
hidden prompt, never from the command line:

  rho cloud connect --device-id <id> --project-id <id> --token-stdin < token.txt`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			endpointFlag, _ := cmd.Flags().GetString("endpoint")
			endpoint, err := cloud.ResolveEndpoint(endpointFlag)
			if err != nil {
				return err
			}
			deviceID, _ := cmd.Flags().GetString("device-id")
			projectID, _ := cmd.Flags().GetString("project-id")
			if deviceID == "" || projectID == "" {
				return fmt.Errorf("device-id and project-id are required")
			}
			if !cloud.ValidOpaqueID(deviceID) || !cloud.ValidOpaqueID(projectID) {
				return fmt.Errorf("device-id and project-id must be GrayCode Cloud identifiers (16-128 letters, digits, '.', '_', ':' or '-')")
			}
			token, err := readDeviceToken(cmd)
			if err != nil {
				return err
			}
			if err := saveCloudDevice(cloud.DeviceConfig{Endpoint: endpoint, DeviceID: deviceID, ProjectID: projectID}, token); err != nil {
				return err
			}
			warnIfPlaintextTokenStore(cmd)
			cmd.Println(auditTint("Rho Cloud connected. Usage synchronization is opt-in and fail-open.", doneGreen))
			return nil
		},
	}
	command.Flags().String("endpoint", "", cloudEndpointFlagHelp)
	command.Flags().String("device-id", "", "Rho Cloud device ID")
	command.Flags().String("project-id", "", "Rho Cloud project ID")
	command.Flags().Bool("token-stdin", false, "Read the device token from standard input")
	command.Flags().String("token", "", "Device token (deprecated: exposed in shell history and process listings)")
	_ = command.Flags().MarkDeprecated("token", "it exposes the device token in shell history and process listings; use --token-stdin or the interactive prompt")
	return command
}

// maxDeviceTokenInput bounds how much is read for a device token.
const maxDeviceTokenInput = 4 << 10

// Credential seams; tests replace them.
var (
	saveCloudDevice   = cloud.SaveDeviceConfig
	cloudTokenStorage = cloud.TokenStorage
	readHiddenSecret  = func() ([]byte, error) { return term.ReadPassword(int(os.Stdin.Fd())) }
)

// warnIfPlaintextTokenStore tells the user, every time a device token is
// saved, when this platform has no OS credential store and the token went to
// the 0600 plaintext fallback file instead.
func warnIfPlaintextTokenStore(cmd *cobra.Command) {
	where, plaintext := cloudTokenStorage()
	if !plaintext {
		return
	}
	cmd.PrintErrln(auditTint("Warning: rho has no OS credential store integration on this platform, so the GrayCode Cloud device token was saved in the "+where+
		". Any process running as your user, and any backup of that file, can read it. Keep the file private and revoke the device in GrayCode Cloud if this machine is shared or lost.", warnAmber))
}

// readDeviceToken returns the device token for `rho cloud connect` from, in
// order: --token-stdin, the deprecated --token flag, or a hidden prompt when
// stdin is a terminal. The token never has to appear in argv, where it would
// land in shell history and be visible to other users via ps.
func readDeviceToken(cmd *cobra.Command) (string, error) {
	flagToken, _ := cmd.Flags().GetString("token")
	fromStdin, _ := cmd.Flags().GetBool("token-stdin")
	var raw string
	switch {
	case fromStdin && flagToken != "":
		return "", errors.New("use either --token-stdin or --token, not both")
	case fromStdin:
		data, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), maxDeviceTokenInput+1))
		if err != nil {
			return "", fmt.Errorf("read device token from stdin: %w", err)
		}
		if len(data) > maxDeviceTokenInput {
			return "", errors.New("device token on stdin is too long")
		}
		raw = string(data)
	case flagToken != "":
		raw = flagToken
	case stdinIsTerminal():
		_, _ = fmt.Fprint(cmd.ErrOrStderr(), "GrayCode Cloud device token: ")
		data, err := readHiddenSecret()
		_, _ = fmt.Fprintln(cmd.ErrOrStderr())
		if err != nil {
			return "", fmt.Errorf("read device token: %w", err)
		}
		raw = string(data)
	default:
		return "", errors.New("a device token is required: pipe it with --token-stdin, or run in a terminal to be prompted (`rho cloud login` needs no token)")
	}
	token := strings.TrimSpace(raw)
	if token == "" {
		return "", errors.New("the device token is empty")
	}
	if strings.ContainsFunc(token, func(r rune) bool { return !unicode.IsGraphic(r) || unicode.IsSpace(r) }) {
		return "", errors.New("the device token must be a single line without spaces")
	}
	return token, nil
}

func newCloudLoginCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "login",
		Short: "Sign in to Rho Cloud in a browser",
		Long: `Start a browser device login against GrayCode Cloud and store the issued
device token in the OS credential store.

The endpoint defaults to ` + cloud.DefaultEndpoint + `. Override it with
--endpoint or ` + cloud.EndpointEnv + ` (for example a local Worker at
http://127.0.0.1:8787). Only https:// endpoints are accepted, except plain
http:// on localhost, 127.0.0.1 and [::1].`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			endpointFlag, _ := cmd.Flags().GetString("endpoint")
			endpoint, err := cloud.ResolveEndpoint(endpointFlag)
			if err != nil {
				return err
			}
			label, _ := cmd.Flags().GetString("label")
			if label == "" {
				label, _ = os.Hostname()
			}
			client := cloud.New(cloud.Config{Endpoint: endpoint})
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			start, err := client.StartDeviceLogin(ctx, label, runtime.GOOS, version)
			if err != nil {
				return err
			}
			approvalURL, err := start.ApprovalURL()
			if err != nil {
				return err
			}
			cmd.Printf("%s\n", auditTint("Open ", textPrimary)+auditTint(start.VerificationURI, infoSky)+auditTint(" and enter code ", textPrimary)+auditTint(start.UserCode, rhoColor))
			if err := openBrowser(approvalURL); err != nil {
				cmd.Printf("%s\n", auditTint(fmt.Sprintf("Could not open the browser automatically: %v", err), textMuted))
			}
			interval := time.Duration(start.Interval) * time.Second
			if interval < time.Second || interval > 30*time.Second {
				interval = 5 * time.Second
			}
			prog := NewCLIProgress("Cloud", []string{"Waiting for browser approval"})
			defer prog.Abort()
			prog.StartStep(0)
			for {
				poll, pollErr := client.PollDeviceLogin(ctx, start.DeviceCode)
				if pollErr != nil {
					prog.FailStep(0, pollErr.Error())
					return pollErr
				}
				if poll.Status == cloud.DeviceLoginPending {
					select {
					case <-ctx.Done():
						prog.FailStep(0, ctx.Err().Error())
						return fmt.Errorf("waiting for browser approval: %w", ctx.Err())
					case <-time.After(interval):
					}
					continue
				}
				// PollDeviceLogin reports expired, consumed, unknown and
				// incomplete states as errors, so this is a complete approval.
				if err := saveCloudDevice(cloud.DeviceConfig{Endpoint: endpoint, DeviceID: poll.DeviceID, ProjectID: poll.ProjectID}, poll.Token); err != nil {
					prog.FailStep(0, err.Error())
					return err
				}
				prog.CompleteStep(0)
				prog.Done()
				warnIfPlaintextTokenStore(cmd)
				cmd.Println(auditTint("Rho Cloud connected for project ", doneGreen) + auditTint(poll.ProjectID, textPrimary) + auditTint(".", doneGreen))
				return nil
			}
		},
	}
	command.Flags().String("endpoint", "", cloudEndpointFlagHelp)
	command.Flags().String("label", "", "Name for this Rho device")
	return command
}

func newCloudStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use: "status", Short: "Show Rho Cloud connection status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, cfg, err := loadCloudClient()
			if errors.Is(err, cloud.ErrNotConnected) {
				cmd.Println(auditTint("GrayCode Cloud is not connected. Run `rho cloud login` to connect.", textMuted))
				return nil
			}
			if err != nil {
				return err
			}
			if !client.Enabled() {
				return cloud.ErrNotConnected
			}
			where, plaintext := cloudTokenStorage()
			storageLine := auditTint("Device token: "+where, textMuted)
			if plaintext {
				storageLine = auditTint("Device token: "+where+" (not an OS credential store)", warnAmber)
			}
			cmd.Println(auditTint("Rho Cloud connected: ", doneGreen) + auditTint(cfg.Endpoint, textPrimary) + auditTint(fmt.Sprintf(" (device %s, project %s)", cfg.DeviceID, cfg.ProjectID), textMuted))
			cmd.Println(storageLine)
			return nil
		},
	}
}

func newCloudContextCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "context",
		Short: "Sync repository context to Rho Cloud",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, cfg, err := loadCloudClient()
			if err != nil {
				return err
			}
			detected, detectErr := detectGitContext(cmd.Context())
			repository, _ := cmd.Flags().GetString("repository")
			if repository == "" {
				repository = detected.Repository
			}
			if repository == "" {
				return detectErr
			}
			contextProvider, _ := cmd.Flags().GetString("provider")
			externalID, _ := cmd.Flags().GetString("external-id")
			branch, _ := cmd.Flags().GetString("branch")
			commit, _ := cmd.Flags().GetString("commit")
			ciRunID, _ := cmd.Flags().GetString("ci-run")
			ciStatus, _ := cmd.Flags().GetString("ci-status")
			ciWorkflow, _ := cmd.Flags().GetString("ci-workflow")
			deploymentID, _ := cmd.Flags().GetString("deployment")
			deploymentStatus, _ := cmd.Flags().GetString("deployment-status")
			deploymentEnvironment, _ := cmd.Flags().GetString("deployment-environment")
			if err := checkFlagChoice("--ci-status", ciStatus, cloud.CIRunStatuses); err != nil {
				return err
			}
			if err := checkFlagChoice("--deployment-status", deploymentStatus, cloud.DeploymentStatuses); err != nil {
				return err
			}
			if contextProvider == "" {
				contextProvider = detected.Provider
			}
			if contextProvider == "" {
				contextProvider = "git"
			}
			if branch == "" {
				branch = detected.Branch
			}
			if commit == "" {
				commit = detected.Commit
			}
			if externalID == "" {
				externalID = repository
			}
			event := cloud.DeliveryContext{ProjectID: cfg.ProjectID, Branch: branch, CommitSHA: commit}
			event.Repository.Provider, event.Repository.ExternalID, event.Repository.Name = contextProvider, externalID, repository
			if ciRunID == "" {
				ciRunID, ciWorkflow = os.Getenv("GITHUB_RUN_ID"), firstValue(ciWorkflow, os.Getenv("GITHUB_WORKFLOW"))
			}
			if ciRunID != "" {
				if ciStatus == "" {
					ciStatus = "running"
				}
				ciProvider := contextProvider
				if os.Getenv("GITHUB_RUN_ID") != "" && ciProvider == "git" {
					ciProvider = "github"
				}
				event.CIRun = &cloud.CIRunContext{Provider: ciProvider, ExternalID: ciRunID, Workflow: ciWorkflow, Status: ciStatus}
			}
			if deploymentID != "" {
				if deploymentStatus == "" {
					deploymentStatus = "running"
				}
				if deploymentEnvironment == "" {
					return fmt.Errorf("deployment-environment is required with --deployment")
				}
				event.Deployment = &cloud.DeploymentContext{Provider: contextProvider, ExternalID: deploymentID, Environment: deploymentEnvironment, Status: deploymentStatus}
			}
			if err := client.SendDeliveryContext(cmd.Context(), event); err != nil {
				return err
			}
			cmd.Println(auditTint("Repository context synced to GrayCode Cloud.", doneGreen))
			return nil
		},
	}
	command.Flags().String("repository", "", "Repository name (auto-detected from Git when omitted)")
	command.Flags().String("provider", "", "Repository provider (auto-detected when omitted)")
	command.Flags().String("external-id", "", "Provider repository identifier (defaults to repository)")
	command.Flags().String("branch", "", "Current branch (auto-detected when omitted)")
	command.Flags().String("commit", "", "Current commit SHA (auto-detected when omitted)")
	command.Flags().String("ci-run", "", "CI run identifier (defaults to GITHUB_RUN_ID)")
	command.Flags().String("ci-status", "", "CI status: "+strings.Join(cloud.CIRunStatuses, ", ")+" (default running)")
	command.Flags().String("ci-workflow", "", "CI workflow name (defaults to GITHUB_WORKFLOW)")
	command.Flags().String("deployment", "", "Deployment identifier")
	command.Flags().String("deployment-status", "", "Deployment status: "+strings.Join(cloud.DeploymentStatuses, ", ")+" (default running)")
	command.Flags().String("deployment-environment", "", "Deployment environment, for example production")
	return command
}

// checkFlagChoice rejects a non-empty flag value outside allowed, naming the
// flag so the user can fix the invocation before anything is sent.
func checkFlagChoice(flag, value string, allowed []string) error {
	if value == "" || slices.Contains(allowed, value) {
		return nil
	}
	return fmt.Errorf("%s %q must be one of: %s", flag, value, strings.Join(allowed, ", "))
}

func firstValue(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func init() {
	cloudCmd.AddCommand(newCloudLoginCmd(), newCloudConnectCmd(), newCloudStatusCmd(), newCloudContextCmd())
	rootCmd.AddCommand(cloudCmd)
}
