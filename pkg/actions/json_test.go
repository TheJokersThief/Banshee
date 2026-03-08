package actions

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

const jsonFileContent = `{
  "name": "my-app",
  "version": "1.0.0",
  "scripts": {
    "build": "tsc",
    "test": "jest"
  },
  "dependencies": {
    "lodash": "^4.17.21"
  },
  "tags": ["alpha", "beta"]
}`

func json_test_setup(t *testing.T) (*JSON, *logrus.Entry, string) {
	t.Helper()

	tempDir := t.TempDir()

	jsonFile := filepath.Join(tempDir, "package.json")
	err := os.WriteFile(jsonFile, []byte(jsonFileContent), 0644)
	assert.NoError(t, err)

	j := &JSON{
		Glob:      filepath.Join(tempDir, "*.json"),
		Path:      "version",
		SubAction: "replace",
		Value:     "2.0.0",
	}

	var buf bytes.Buffer
	logger := logrus.New()
	logger.Out = &buf

	return j, logger.WithField("action", "json"), jsonFile
}

func TestJSON_Run_Replace(t *testing.T) {
	j, logEntry, jsonFile := json_test_setup(t)

	err := j.Run(logEntry)
	assert.NoError(t, err)

	content, err := os.ReadFile(jsonFile)
	assert.NoError(t, err)

	var doc map[string]interface{}
	assert.NoError(t, json.Unmarshal(content, &doc))
	assert.Equal(t, "2.0.0", doc["version"])
}

func TestJSON_Run_Add(t *testing.T) {
	j, logEntry, jsonFile := json_test_setup(t)
	j.SubAction = "add"
	j.Path = "author"
	j.Value = "Jane Doe"

	err := j.Run(logEntry)
	assert.NoError(t, err)

	content, err := os.ReadFile(jsonFile)
	assert.NoError(t, err)

	var doc map[string]interface{}
	assert.NoError(t, json.Unmarshal(content, &doc))
	assert.Equal(t, "Jane Doe", doc["author"])
}

func TestJSON_Run_Delete(t *testing.T) {
	j, logEntry, jsonFile := json_test_setup(t)
	j.SubAction = "delete"
	j.Path = "version"
	j.Value = ""

	err := j.Run(logEntry)
	assert.NoError(t, err)

	content, err := os.ReadFile(jsonFile)
	assert.NoError(t, err)

	var doc map[string]interface{}
	assert.NoError(t, json.Unmarshal(content, &doc))
	_, hasVersion := doc["version"]
	assert.False(t, hasVersion)
}

func TestJSON_Run_ListAppend(t *testing.T) {
	j, logEntry, jsonFile := json_test_setup(t)
	j.SubAction = "list_append"
	j.Path = "tags"
	j.Value = "gamma"

	err := j.Run(logEntry)
	assert.NoError(t, err)

	content, err := os.ReadFile(jsonFile)
	assert.NoError(t, err)

	var doc map[string]interface{}
	assert.NoError(t, json.Unmarshal(content, &doc))

	tags, ok := doc["tags"].([]interface{})
	assert.True(t, ok)
	assert.Equal(t, []interface{}{"alpha", "beta", "gamma"}, tags)
}
