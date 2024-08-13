// Manipulate a YAML document
package actions

import (
	"fmt"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/sirupsen/logrus"
	"github.com/yargevad/filepathx"
)

type YAML struct {
	SubAction string
	Old       string
	New       string
	Glob      string
}

func NewYAMLAction(dir string, description string, input map[string]string) *YAML {
	glob, hasSpecifiedGlob := input["glob"]
	if !hasSpecifiedGlob {
		glob = "**/*.yaml"
	}
	globPattern := dir + "/" + glob

	return &YAML{
		SubAction: input["sub_action"],
		Old:       input["yamlpath"],
		New:       input["value"],
		Glob:      globPattern,
	}
}

// Run executes the YAML action.
// It searches for files that match the specified glob pattern,
// reads each file, unmarshals its content into a YAML document,
// performs the specified subaction (e.g., delete), and then
// writes the modified document back to the file.
// If any errors occur during the process, they are logged and
// the execution continues with the next file.
func (r *YAML) Run(log *logrus.Entry) error {

	matches, err := filepathx.Glob(r.Glob)
	if err != nil {
		logrus.WithField("pattern", r.Glob).Error("Error globbing file path: ", err)
		return err
	}

	for _, file := range matches {
		var doc map[string]interface{}
		cm := yaml.CommentMap{}

		content, readErr := os.ReadFile(file)
		if readErr != nil {
			log.Errorf("error changing %s: %s", file, readErr)
			continue
		}

		var err error
		if err = yaml.UnmarshalWithOptions(content, &doc, yaml.CommentToMap(cm)); err != nil {
			return err
		}

		parent, lastKey, traversalErr := r.traverseViaDotNotation(&doc, r.Old)
		if traversalErr != nil {
			log.Errorf("error changing %s: %s", file, traversalErr)
			continue
		}

		switch r.SubAction {
		case "delete":
			delete(*parent, lastKey)
		case "replace":
			(*parent)[lastKey] = r.New
		case "add":
			(*parent)[lastKey] = r.New
		case "list_append":
			if parentList, ok := (*parent)[lastKey].([]interface{}); ok {
				(*parent)[lastKey] = append(parentList, r.New)
			}
		default:
			// If the subaction is unknown, we won't proveed any further
			return fmt.Errorf("unknown sub action: %s", r.SubAction)
		}

		out, marhsalErr := yaml.MarshalWithOptions(doc, yaml.WithComment(cm))
		if marhsalErr != nil {
			return marhsalErr
		}

		writeErr := os.WriteFile(file, out, 0644)
		if writeErr != nil {
			log.Errorf("error changing %s: %s", file, writeErr)
			continue
		}
	}

	return nil
}

// traverseViaDotNotation traverses a YAML data structure using dot notation.
// It takes a pointer to an anonymous YAML struc and a path string in dot notation.
// It returns a pointer to the local parent in the YAML doc, the last key in the dot notation, and an error if any.
func (r *YAML) traverseViaDotNotation(data *map[string]interface{}, path string) (*map[string]interface{}, string, error) {
	keys := strings.Split(path, ".")

	lastKey := keys[len(keys)-1]
	parentKeys := keys[:len(keys)-1]

	parent := *data
	for _, k := range parentKeys {
		value, ok := parent[k]
		if !ok {
			return nil, "", fmt.Errorf("path not found: %s", path)
		}

		parent, ok = value.(map[string]interface{})
		if !ok {
			return nil, "", fmt.Errorf("invalid key: %s", path)
		}
	}

	return &parent, lastKey, nil
}
