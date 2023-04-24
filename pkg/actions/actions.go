// Base action
package actions

import "fmt"

type ActionRunner interface {
	Run() error
}

type Action struct {
	Directory string

	Description string            `fig:"description"`
	Action      string            `fig:"action"`
	Input       map[string]string `fig:"input"`
}

func RunAction(actionID string, dir string, description string, input map[string]string) error {
	var action ActionRunner

	switch actionID {
	case "add_file":
		action = NewAddFileAction(dir, description, input)
	case "replace":
		action = NewReplaceAction(dir, description, input)
	case "run_command":
		action = NewRunCommandAction(dir, description, input)
	default:
		return fmt.Errorf("Unrecognised command: %s", actionID)
	}

	return action.Run()
}
