package cmd

import (
	"fmt"
	"strings"

	rhoconfig "github.com/GrayCodeAI/rho/internal/config"
	"github.com/GrayCodeAI/rho/internal/errhint"
	"github.com/GrayCodeAI/rho/internal/rhoerr"
)

// friendlyErrorMessage returns the user-friendly message for an error.
// Delegates to the shared rhoerr.ClassifyErrorMessage for the base message,
// then enriches specific cases with dynamic hints that require the config
// package (rhoerr can't import internal/config without a cycle).
func friendlyErrorMessage(err error) string {
	msg := rhoerr.ClassifyErrorMessage(err)

	if err == nil {
		return msg
	}

	ec := rhoerr.ClassifyError(err)
	low := strings.ToLower(err.Error())

	switch ec.ExitCode {
	case rhoerr.ExitNotFound:
		// Enrich model-not-found errors with concrete examples from the catalog.
		if strings.Contains(low, "model") || strings.Contains(low, "unknown") ||
			strings.Contains(low, "does not exist") {
			ex1, ex2 := rhoconfig.ExampleModelHints()
			msg = fmt.Sprintf(
				"Model not found. Check your model name with /model.\n  Examples from the flux catalog: %s, %s\n  Use /models to list all models, or /config to change provider.",
				ex1, ex2,
			)
		}
	case rhoerr.ExitAuth:
		msg += "\n  Check your API key with /config. Keys can expire or be revoked."
	case rhoerr.ExitNetwork:
		msg += "\n  Check your internet connection. If you're behind a proxy, configure it with /config."
	case rhoerr.ExitTimeout:
		msg += "\n  The request took too long. Try again, or use /model to switch to a faster provider."
	}

	// Provider-specific one-line hint. errhint.Classify is deliberately
	// conservative (gates on a provider-origin marker), so local errors draw no
	// hint here.
	if h := errhint.TUIHint(err); h != "" {
		msg += "\n  " + h
	}

	return msg
}
