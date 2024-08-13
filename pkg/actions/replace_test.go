package actions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestReplace_findAndReplaceWorker(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a temporary file with the old string
	oldFile := filepath.Join(tempDir, "old.txt")
	err := os.WriteFile(oldFile, []byte("This is the old string"), 0644)
	assert.NoError(t, err)

	// Create a temporary file without the old string
	newFile := filepath.Join(tempDir, "new.txt")
	err = os.WriteFile(newFile, []byte("This is a new file"), 0644)
	assert.NoError(t, err)

	// Create a Replace instance
	r := &Replace{
		OldString: "old string",
		NewString: "new string",
	}

	// Create a logrus logger
	logger := logrus.New()

	// Create channels for files and errors
	files := make(chan string)
	errChan := make(chan error)

	// Start the worker in a goroutine
	go r.findAndReplaceWorker(logger.WithField("action", "replace"), files, errChan)

	// Send the files to the worker
	files <- oldFile
	files <- newFile
	close(files)

	// Wait for the worker to finish
	if len(errChan) != 0 {
		for err := range errChan {
			assert.NoError(t, err)
		}
	}

	// Check if the old string was replaced in the old file
	content, err := os.ReadFile(oldFile)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(string(content), "new string"))

	// Check if the new file remains unchanged
	content, err = os.ReadFile(newFile)
	assert.NoError(t, err)
	assert.False(t, strings.Contains(string(content), "new string"))
}
