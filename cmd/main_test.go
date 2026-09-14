package cmd

import (
	"os"
	"testing"

	"github.com/GrayCodeAI/rho/internal/catalogtest"
	"github.com/GrayCodeAI/rho/internal/testutil"
)

func TestMain(m *testing.M) {
	cleanupStorage, err := testutil.InstallHermeticStorage()
	if err != nil {
		os.Exit(1)
	}
	cleanup := catalogtest.InstallGlobal()
	code := m.Run()
	cleanup()
	cleanupStorage()
	os.Exit(code)
}
