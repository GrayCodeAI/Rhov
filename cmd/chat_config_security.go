package cmd

import (
	"strings"
	"sync"

	"github.com/GrayCodeAI/rho/internal/engine/safety"
)

var (
	configNoticeRedactorOnce sync.Once
	configNoticeRedactor     *safety.OutputRedactor
)

func configNoticeRedact() *safety.OutputRedactor {
	configNoticeRedactorOnce.Do(func() {
		configNoticeRedactor = safety.NewOutputRedactor()
	})
	return configNoticeRedactor
}

// sanitizeConfigNotice redacts API keys and tokens before showing errors in the TUI.
func sanitizeConfigNotice(notice string) string {
	notice = strings.TrimSpace(notice)
	if notice == "" {
		return ""
	}
	return configNoticeRedact().Redact(notice)
}

func (m *chatModel) wipeConfigKeyInput() {
	m.configInput.Reset()
	m.configInput.SetValue("")
}
