package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// CI run and deployment statuses accepted by POST /v1/delivery-context.
var (
	CIRunStatuses      = []string{"queued", "running", "succeeded", "failed", "cancelled"}
	DeploymentStatuses = []string{"queued", "running", "succeeded", "failed", "cancelled", "rolled_back"}
)

// SendDeliveryContext uploads repository and delivery metadata for the
// connected project. It backs the explicit `rho cloud context` command, so
// validation failures, transport errors and non-2xx responses are all
// returned; nothing is reported as synced unless GrayCode Cloud accepted it.
func (c *Client) SendDeliveryContext(ctx context.Context, event DeliveryContext) error {
	if err := c.checkEndpoint(); err != nil {
		return err
	}
	if c.token == "" {
		return ErrNotConnected
	}
	event = event.normalized()
	if err := event.Validate(); err != nil {
		return err
	}
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal delivery context: %w", err)
	}
	req, err := c.newJSONRequest(ctx, "/v1/delivery-context", body, true)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("sync delivery context to GrayCode Cloud: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return readAPIError("delivery context sync", resp)
	}
	return nil
}

// normalized trims every text field, as the Worker's schema does before its
// length checks.
func (e DeliveryContext) normalized() DeliveryContext {
	trim := strings.TrimSpace
	e.ProjectID = trim(e.ProjectID)
	e.Repository.Provider = trim(e.Repository.Provider)
	e.Repository.ExternalID = trim(e.Repository.ExternalID)
	e.Repository.Name = trim(e.Repository.Name)
	e.Repository.URL = trim(e.Repository.URL)
	e.Repository.DefaultBranch = trim(e.Repository.DefaultBranch)
	e.Branch = trim(e.Branch)
	e.CommitSHA = trim(e.CommitSHA)
	if e.CIRun != nil {
		run := *e.CIRun
		run.Provider, run.ExternalID, run.Workflow, run.Status = trim(run.Provider), trim(run.ExternalID), trim(run.Workflow), trim(run.Status)
		e.CIRun = &run
	}
	if e.Deployment != nil {
		deployment := *e.Deployment
		deployment.Provider, deployment.ExternalID = trim(deployment.Provider), trim(deployment.ExternalID)
		deployment.Environment, deployment.Status = trim(deployment.Environment), trim(deployment.Status)
		e.Deployment = &deployment
	}
	return e
}

// Validate checks the event against the POST /v1/delivery-context schema so
// the user gets a specific message instead of the Worker's generic
// "Invalid delivery context". Call it on a normalized (trimmed) event.
func (e DeliveryContext) Validate() error {
	if !ValidOpaqueID(e.ProjectID) {
		return fmt.Errorf("project ID %q is not a valid GrayCode Cloud identifier", e.ProjectID)
	}
	checks := []error{
		requireText("repository provider", e.Repository.Provider, 50),
		requireText("repository external ID", e.Repository.ExternalID, 200),
		requireText("repository name", e.Repository.Name, 300),
		optionalText("repository default branch", e.Repository.DefaultBranch, 200),
		optionalText("branch", e.Branch, 200),
		optionalText("commit SHA", e.CommitSHA, 128),
	}
	if e.Repository.URL != "" {
		if u, err := url.Parse(e.Repository.URL); err != nil || u.Scheme == "" || len(e.Repository.URL) > 2000 {
			checks = append(checks, fmt.Errorf("repository URL %q is not a valid URL", e.Repository.URL))
		}
	}
	if run := e.CIRun; run != nil {
		checks = append(
			checks,
			requireText("CI run provider", run.Provider, 50),
			requireText("CI run ID", run.ExternalID, 200),
			optionalText("CI workflow", run.Workflow, 200),
			requireOneOf("CI run status", run.Status, CIRunStatuses),
		)
	}
	if deployment := e.Deployment; deployment != nil {
		checks = append(
			checks,
			requireText("deployment provider", deployment.Provider, 50),
			requireText("deployment ID", deployment.ExternalID, 200),
			requireText("deployment environment", deployment.Environment, 100),
			requireOneOf("deployment status", deployment.Status, DeploymentStatuses),
		)
	}
	for _, err := range checks {
		if err != nil {
			return err
		}
	}
	return nil
}
