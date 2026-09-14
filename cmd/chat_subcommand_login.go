package cmd

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// loginSubcommand implements the /login slash command. Tau uses /login as its
// provider-auth entry point; Rho routes it to the existing config panel, which
// owns credential storage (OS keychain) and provider/model selection.
//
// `/login <provider>` preselects a provider before opening the panel so the
// common "log in to X" flow is one command.
type loginSubcommand struct{}

func (l *loginSubcommand) Name() string      { return "login" }
func (l *loginSubcommand) Aliases() []string { return []string{"auth"} }
func (l *loginSubcommand) Description() string {
	return "authenticate a provider (opens the config panel)"
}
func (l *loginSubcommand) Usage() string { return "/login [provider]" }

func (l *loginSubcommand) Handle(m *chatModel, args []string, text string) (tea.Model, tea.Cmd) {
	if len(args) > 0 {
		provider := strings.TrimSpace(args[0])
		if provider != "" {
			return m.handleConfigCommand([]string{"config", "provider", provider}, "")
		}
	}
	// No provider given: open the config panel, which is the credential and
	// model setup surface.
	return m.handleConfigCommand([]string{"config"}, "")
}

func init() {
	subcommandRegistry.Register(&loginSubcommand{})
}
