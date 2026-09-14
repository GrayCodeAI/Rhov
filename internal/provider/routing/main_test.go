package routing

import (
	"os"
	"testing"

	"github.com/GrayCodeAI/rho/internal/catalogtest"
)

func TestMain(m *testing.M) {
	cleanup := catalogtest.InstallGlobal()
	defer cleanup()
	os.Exit(m.Run())
}
