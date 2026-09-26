package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	cloud "github.com/GrayCodeAI/rho/internal/platform/cloud"
)

// cloudUsageWait bounds how long a command waits, after writing its output,
// for the fail-open GrayCode Cloud usage upload to finish. Tests shorten it.
var cloudUsageWait = 3 * time.Second

// startCloudUsage uploads one usage event in the background when GrayCode
// Cloud is connected and returns a function that waits for the upload,
// bounded by cloudUsageWait. Callers defer the returned function so the
// process does not exit (and kill the upload) before the request completes.
// Upload failures never change the command's result; a server rejection is
// reported on stderr, while transport errors (for example offline) stay
// silent.
func startCloudUsage(stderr io.Writer, build func(cloud.DeviceConfig) cloud.UsageEvent) (wait func()) {
	client, cfg, err := loadCloudClient()
	if err != nil || !client.Enabled() {
		return func() {}
	}
	event := build(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), cloudUsageWait)
	done := make(chan error, 1)
	go func() { done <- client.RecordUsage(ctx, event) }()
	return func() {
		defer cancel()
		var apiErr *cloud.APIError
		if err := <-done; errors.As(err, &apiErr) && !IsQuiet() {
			_, _ = fmt.Fprintln(stderr, auditTint("warning: "+apiErr.Error(), textMuted))
		}
	}
}
