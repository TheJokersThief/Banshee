package actions

import (
	"bytes"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/thejokersthief/banshee/pkg/configs"
)

func TestRunCommand_Run(t *testing.T) {
	// Create a new RunCommand instance
	rc := &RunCommand{
		Command:      "echo 'Hello, World!'",
		BaseDir:      "./",
		GlobalConfig: &configs.GlobalConfig{MigrationDir: "/tmp"},
	}

	// Create a buffer to capture the command output
	var buf bytes.Buffer
	logger := logrus.New()
	logger.Out = &buf

	// Run the command
	err := rc.Run(logger.WithField("action", "run_command"))
	assert.NoError(t, err)

	// Check if the command output matches the expected value
	expectedOutput := ""
	assert.Equal(t, expectedOutput, buf.String())
}
