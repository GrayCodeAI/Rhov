//go:build !media_engine

package gateway

import fluxengine "github.com/GrayCodeAI/flux/engine"

// wireOptionalBackends is a no-op when the media_engine build tag is disabled.
// The full implementation (media_backends.go) is only compiled with
// -tags media_engine and requires a flux checkout that exposes the
// GenerateImage/Transcribe engine facade (not present in the published v0.0.1).
func wireOptionalBackends(eng *fluxengine.Engine) {}
