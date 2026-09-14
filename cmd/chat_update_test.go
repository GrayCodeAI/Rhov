package cmd

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestCtrlCTwiceExits(t *testing.T) {
	m := newTestChatModel()

	first, _ := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	cm := requireChatModel(t, first)
	if cm.quitting {
		t.Fatal("first Ctrl+C must not mark the CLI as quitting")
	}

	cm.lastCtrlC = time.Now()
	second, secondCmd := cm.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	exited := requireChatModel(t, second)
	if !exited.quitting {
		t.Fatal("second Ctrl+C should exit the CLI")
	}
	if secondCmd == nil {
		t.Fatal("second Ctrl+C should return tea.Quit")
	}
	if _, ok := secondCmd().(tea.QuitMsg); !ok {
		t.Fatalf("second Ctrl+C command = %T, want tea.QuitMsg", secondCmd())
	}
}

func TestCtrlCTwiceWhileStreamingCancelsThenExits(t *testing.T) {
	m := newTestChatModel()
	m.waiting = true
	cancelled := false
	m.cancel = func() { cancelled = true }

	first, firstCmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	cm := requireChatModel(t, first)
	if !cancelled {
		t.Fatal("first Ctrl+C should cancel the active stream")
	}
	if firstCmd != nil {
		t.Fatal("first Ctrl+C should cancel without quitting")
	}
	if cm.waiting {
		t.Fatal("first Ctrl+C should leave the model idle")
	}

	second, secondCmd := cm.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	exited := requireChatModel(t, second)
	if !exited.quitting {
		t.Fatal("second Ctrl+C should exit after cancelling the stream")
	}
	if secondCmd == nil {
		t.Fatal("second Ctrl+C should return tea.Quit")
	}
	if _, ok := secondCmd().(tea.QuitMsg); !ok {
		t.Fatalf("second Ctrl+C command = %T, want tea.QuitMsg", secondCmd())
	}
}

func TestExitAndQuitCommandsExit(t *testing.T) {
	for _, command := range []string{"/exit", "/quit"} {
		t.Run(command, func(t *testing.T) {
			m := newTestChatModel()

			next, cmd := m.handleSessionCommand(command, nil, command)
			cm := requireChatModel(t, next)
			if !cm.quitting {
				t.Fatalf("%s should mark the CLI as quitting", command)
			}
			if cmd == nil {
				t.Fatalf("%s should return tea.Quit", command)
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatalf("%s command = %T, want tea.QuitMsg", command, cmd())
			}
		})
	}
}
