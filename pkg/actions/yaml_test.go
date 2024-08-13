package actions

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

const yamlFileContent = `
key1: value1
# Comment
key2:
  subkey: value2
  arrayList:
    - item1
    - item2
`

func TestYAML_Run_Delete(t *testing.T) {
	y, logEntry, yamlFile := test_setup(t)

	// Run the YAML action
	err := y.Run(logEntry)
	assert.NoError(t, err)

	// Check if the key was deleted from the YAML file
	content, err := os.ReadFile(yamlFile)
	assert.NoError(t, err)
	expectedOutput := `# Comment
key2:
  arrayList:
  - item1
  - item2
  subkey: value2
`
	assert.Equal(t, expectedOutput, string(content))
}

func TestYAML_Run_Add(t *testing.T) {
	y, logEntry, yamlFile := test_setup(t)
	y.SubAction = "add"
	y.Old = "key2.newkey"
	y.New = "new value"

	// Run the YAML action
	err := y.Run(logEntry)
	assert.NoError(t, err)

	// Check if the key was deleted from the YAML file
	content, err := os.ReadFile(yamlFile)
	assert.NoError(t, err)
	expectedOutput := `key1: value1
# Comment
key2:
  arrayList:
  - item1
  - item2
  newkey: new value
  subkey: value2
`
	assert.Equal(t, expectedOutput, string(content))
}

func TestYAML_Run_Replace(t *testing.T) {
	y, logEntry, yamlFile := test_setup(t)
	y.SubAction = "add"
	y.Old = "key2.subkey"
	y.New = "new value"

	// Run the YAML action
	err := y.Run(logEntry)
	assert.NoError(t, err)

	// Check if the key was deleted from the YAML file
	content, err := os.ReadFile(yamlFile)
	assert.NoError(t, err)
	expectedOutput := `key1: value1
# Comment
key2:
  arrayList:
  - item1
  - item2
  subkey: new value
`
	assert.Equal(t, expectedOutput, string(content))
}

func TestYAML_Run_ListAppend(t *testing.T) {
	y, logEntry, yamlFile := test_setup(t)
	y.SubAction = "list_append"
	y.Old = "key2.arrayList"
	y.New = "item3"

	// Run the YAML action
	err := y.Run(logEntry)
	assert.NoError(t, err)

	// Check if the key was deleted from the YAML file
	content, err := os.ReadFile(yamlFile)
	assert.NoError(t, err)
	expectedOutput := `key1: value1
# Comment
key2:
  arrayList:
  - item1
  - item2
  - item3
  subkey: value2
`
	assert.Equal(t, expectedOutput, string(content))
}

func test_setup(t *testing.T) (*YAML, *logrus.Entry, string) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a temporary YAML file
	yamlFile := filepath.Join(tempDir, "test.yaml")
	err := os.WriteFile(yamlFile, []byte(yamlFileContent), 0644)
	assert.NoError(t, err)

	// Create a new YAML instance
	y := &YAML{
		Glob:      filepath.Join(tempDir, "*.yaml"),
		Old:       "key1",
		SubAction: "delete",
	}

	// Create a buffer to capture the log output
	var buf bytes.Buffer
	logger := logrus.New()
	logger.Out = &buf

	return y, logger.WithField("action", "yaml"), yamlFile
}
