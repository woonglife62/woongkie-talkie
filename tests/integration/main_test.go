//go:build integration

package integration

import (
	"os"
	"testing"

	"github.com/woonglife62/woongkie-talkie/pkg/logger"
)

func TestMain(m *testing.M) {
	logger.Initialize(true)
	os.Exit(m.Run())
}
