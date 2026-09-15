package cmd

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// System-message handlers extracted from chatModel.Update. Each
// handles one terminal message type (every path returns); the dispatch
// switch in Update keeps only the one-line call.

func (m chatModel) updateSessionSaveResult(msg sessionSaveResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.recordWALError(msg.err)
		return m, nil
	}
	// Remove the WAL only when no append happened after the snapshot.
	// Doing this in the background save closure could race with a new
	// submission and lose messages that were not in the saved session.
	if msg.id == m.sessionID && msg.seq == m.walSeq && m.wal != nil {
		if err := m.wal.Remove(); err != nil {
			m.recordWALError(err)
		} else {
			m.wal = nil
		}
	}
	return m, nil
}

func (m chatModel) updateFocusMsg(msg tea.FocusMsg) (tea.Model, tea.Cmd) {
	m.viewDirty = true
	m.updateViewportContent()
	if focus := m.ensurePromptInputFocus(); focus != nil {
		return m, focus
	}
	return m, nil
}

func (m chatModel) updateBlurMsg(msg tea.BlurMsg) (tea.Model, tea.Cmd) {
	if m.uiFocus == focusPrompt && !m.configOpen && !m.useConfigInput {
		m.input.Blur()
	}
	// Track that the terminal lost focus during a turn so we can notify
	// the user when the agent completes.
	if m.waiting {
		m.backgrounded = true
	}
	m.viewDirty = true
	m.updateViewportContent()
	return m, nil
}

func (m chatModel) updateInterruptMsg(msg tea.InterruptMsg) (tea.Model, tea.Cmd) {
	// External SIGINT delivered while the terminal is not in raw mode
	// (e.g. `kill -INT`, tmux/screen `prefix` + ctrl+c). Bubble Tea would
	// otherwise exit without saving the session.
	return m.quitModel()
}

func (m chatModel) updateQuitMsg(msg tea.QuitMsg) (tea.Model, tea.Cmd) {
	// SIGTERM (e.g. `kill <pid>`, terminal close on some platforms).
	// Exit through the same save-and-cleanup path as Ctrl+C.
	return m.quitModel()
}

func (m chatModel) updatePromptKeepAliveMsg(msg promptKeepAliveMsg) (tea.Model, tea.Cmd) {
	if m.uiFocus == focusPrompt && !m.configOpen && !m.useConfigInput {
		if !m.input.Focused() {
			m.viewDirty = true
			m.updateViewportContent()
			return m, tea.Batch(promptKeepAliveCmd(), m.input.Focus())
		}
	}
	return m, promptKeepAliveCmd()
}

func (m chatModel) updateStatusLeftPRsMsg(msg statusLeftPRsMsg) (tea.Model, tea.Cmd) {
	// Async PR lookup result — only apply if we're still on the same branch.
	if m.statusLeftBranch == msg.branch {
		m.statusLeftPRs = msg.nums
		m.viewDirty = true
		m.updateViewportContent()
	}
	return m, nil
}

func (m chatModel) updateEyeBlinkTickMsg(msg eyeBlinkTickMsg) (tea.Model, tea.Cmd) {
	if m.showWelcomeBanner() {
		m.eyeFrame = 1
		m.rebuildWelcomeCache()
		if len(m.messages) > 0 && m.messages[0].role == "welcome" {
			m.messages[0].content = m.welcomeCache
		}
		m.viewDirty = true
		m.updateViewportContent()
		return m, tea.Batch(eyeBlinkTickCmd(), eyeFrameNextCmd(2, 60*time.Millisecond))
	}
	return m, eyeBlinkTickCmd()
}

func (m chatModel) updateEyeFrameNextMsg(msg eyeFrameNextMsg) (tea.Model, tea.Cmd) {
	if m.showWelcomeBanner() {
		m.eyeFrame = msg.frame
		m.rebuildWelcomeCache()
		if len(m.messages) > 0 && m.messages[0].role == "welcome" {
			m.messages[0].content = m.welcomeCache
		}
		m.viewDirty = true
		m.updateViewportContent()
		switch msg.frame {
		case 2:
			return m, eyeFrameNextCmd(3, 100*time.Millisecond)
		case 3:
			return m, eyeFrameNextCmd(0, 60*time.Millisecond)
		}
	} else {
		m.eyeFrame = 0
	}
	return m, nil
}

func (m chatModel) updateProcessArrowTickMsg(msg processArrowTickMsg) (tea.Model, tea.Cmd) {
	// A matching seq means no newer arrow keypress has arrived since this
	// tick was armed, so the burst is over — clear the flag unconditionally.
	// Without this, a burst's final keypress (which arms no tick of its
	// own) would leave arrowBurstActive stuck true forever, silently
	// swallowing every subsequent keystroke (see applyPromptArrowKey).
	if m.arrowSeq == msg.seq {
		m.arrowBurstActive = false
	}
	if m.pendingArrow != nil && m.arrowSeq == msg.seq {
		msgToProcess := *m.pendingArrow
		m.pendingArrow = nil
		m.arrowBurstActive = false
		m.processingGenuineArrow = true
		next, cmd := m.Update(msgToProcess)
		if nextModel, ok := next.(chatModel); ok {
			nextModel.processingGenuineArrow = false
			return nextModel, cmd
		}
		return next, cmd
	}
	return m, nil
}

func (m chatModel) updatePasteMsg(msg tea.PasteMsg) (tea.Model, tea.Cmd) {
	if m.configOpen {
		var cmd tea.Cmd
		m.configInput, cmd = m.configInput.Update(msg)
		m.viewDirty = true
		m.updateViewportContent()
		return m, cmd
	}
	if m.uiFocus == focusPrompt {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.viewDirty = true
		m.updateViewportContent()
		return m, cmd
	}
	return m, nil
}
