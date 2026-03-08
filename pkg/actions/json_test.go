package actions

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func jsonTestSetup(t *testing.T) (*JSON, *logrus.Entry, string) {
	t.Helper()

	tempDir := t.TempDir()

	jsonFile := filepath.Join(tempDir, "package.json")
	require.NoError(t, os.WriteFile(jsonFile, []byte(jsonFileContent), 0644))

	j := &JSON{
		Glob:      filepath.Join(tempDir, "*.json"),
		Path:      "version",
		SubAction: "replace",
		Value:     "2.0.0",
	}

	logger := logrus.New()
	logger.Out = io.Discard

	return j, logger.WithField("action", "json"), jsonFile
}

func TestJSON_Run_Replace(t *testing.T) {
	j, logEntry, jsonFile := jsonTestSetup(t)

	err := j.Run(logEntry)
	assert.NoError(t, err)

	content, err := os.ReadFile(jsonFile)
	require.NoError(t, err)

	var doc map[string]interface{}
	require.NoError(t, json.Unmarshal(content, &doc))
	assert.Equal(t, "2.0.0", doc["version"])
	// Verify sibling keys are intact
	assert.Equal(t, "my-app", doc["name"])
	scripts, ok := doc["scripts"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "tsc", scripts["build"])
}

func TestJSON_Run_Add(t *testing.T) {
	j, logEntry, jsonFile := jsonTestSetup(t)
	j.SubAction = "add"
	j.Path = "author"
	j.Value = "Jane Doe"

	err := j.Run(logEntry)
	assert.NoError(t, err)

	content, err := os.ReadFile(jsonFile)
	require.NoError(t, err)

	var doc map[string]interface{}
	require.NoError(t, json.Unmarshal(content, &doc))
	assert.Equal(t, "Jane Doe", doc["author"])
	// Verify pre-existing keys are intact
	assert.Equal(t, "my-app", doc["name"])
	assert.Equal(t, "1.0.0", doc["version"])
}

func TestJSON_Run_Delete(t *testing.T) {
	j, logEntry, jsonFile := jsonTestSetup(t)
	j.SubAction = "delete"
	j.Path = "version"
	j.Value = ""

	err := j.Run(logEntry)
	assert.NoError(t, err)

	content, err := os.ReadFile(jsonFile)
	require.NoError(t, err)

	var doc map[string]interface{}
	require.NoError(t, json.Unmarshal(content, &doc))
	_, hasVersion := doc["version"]
	assert.False(t, hasVersion)
	// Verify surviving keys are intact
	assert.Equal(t, "my-app", doc["name"])
	_, hasScripts := doc["scripts"]
	assert.True(t, hasScripts)
	_, hasTags := doc["tags"]
	assert.True(t, hasTags)
}

func TestJSON_Run_ListAppend(t *testing.T) {
	j, logEntry, jsonFile := jsonTestSetup(t)
	j.SubAction = "list_append"
	j.Path = "tags"
	j.Value = "gamma"

	err := j.Run(logEntry)
	assert.NoError(t, err)

	content, err := os.ReadFile(jsonFile)
	require.NoError(t, err)

	var doc map[string]interface{}
	require.NoError(t, json.Unmarshal(content, &doc))

	tags, ok := doc["tags"].([]interface{})
	assert.True(t, ok)
	assert.Equal(t, []interface{}{"alpha", "beta", "gamma"}, tags)
}

func TestJSON_Run_ListAppend_NonArray(t *testing.T) {
	j, logEntry, jsonFile := jsonTestSetup(t)
	j.SubAction = "list_append"
	j.Path = "version" // a string, not an array
	j.Value = "extra"

	err := j.Run(logEntry)
	// Run returns nil (continue-on-error); the file should be unchanged
	assert.NoError(t, err)

	content, err := os.ReadFile(jsonFile)
	require.NoError(t, err)

	var doc map[string]interface{}
	require.NoError(t, json.Unmarshal(content, &doc))
	assert.Equal(t, "1.0.0", doc["version"])
}

func TestJSON_Run_UnknownSubAction(t *testing.T) {
	j, logEntry, _ := jsonTestSetup(t)
	j.SubAction = "upsert"

	err := j.Run(logEntry)
	assert.ErrorContains(t, err, "unknown sub action")
}
