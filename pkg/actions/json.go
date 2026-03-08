// Manipulate a JSON document
package actions

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"github.com/yargevad/filepathx"
)

type JSON struct {
	SubAction string
	Path      string
	Value     string
	Glob      string
}

func NewJSONAction(dir string, description string, input map[string]string) *JSON {
	glob, hasSpecifiedGlob := input["glob"]
	if !hasSpecifiedGlob {
		glob = "**/*.json"
	}

	return &JSON{
		SubAction: input["sub_action"],
		Path:      input["jsonpath"],
		Value:     input["value"],
		Glob:      filepath.Join(dir, glob),
	}
}

// Run executes the JSON action.
// It searches for files that match the specified glob pattern,
// reads each file, applies the specified sub-action, and writes
// the modified content back to the file.
// If any errors occur during the process, they are logged and
// the execution continues with the next file.
func (j *JSON) Run(log *logrus.Entry) error {
	matches, err := filepathx.Glob(j.Glob)
	if err != nil {
		log.WithField("pattern", j.Glob).Error("Error globbing file path: ", err)
		return err
	}

	for _, file := range matches {
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			log.Errorf("error reading %s: %s", file, readErr)
			continue
		}

		var out []byte
		var actionErr error

		switch j.SubAction {
		case subActionReplace, subActionAdd:
			out, actionErr = sjson.SetBytes(content, j.Path, j.Value)
		case subActionDelete:
			out, actionErr = sjson.DeleteBytes(content, j.Path)
		case subActionListAppend:
			current := gjson.GetBytes(content, j.Path)
			if !current.Exists() || !current.IsArray() {
				log.Errorf("list_append requires an existing array at path %q in %s", j.Path, file)
				continue
			}
			arr := current.Array()
			items := make([]interface{}, 0, len(arr)+1)
			for _, item := range arr {
				items = append(items, item.Value())
			}
			items = append(items, j.Value)
			out, actionErr = sjson.SetBytes(content, j.Path, items)
		default:
			return fmt.Errorf("unknown sub action: %s", j.SubAction)
		}

		if actionErr != nil {
			log.Errorf("error applying action to %s: %s", file, actionErr)
			continue
		}

		writeErr := os.WriteFile(file, out, 0644)
		if writeErr != nil {
			log.Errorf("error writing %s: %s", file, writeErr)
			continue
		}
	}

	return nil
}
