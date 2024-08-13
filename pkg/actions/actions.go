// Base action
package actions

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/thejokersthief/banshee/pkg/configs"
)

type ActionRunner interface {
	Run(log *logrus.Entry) error
}

func RunAction(log *logrus.Entry, globalConfig *configs.GlobalConfig, actionID string, dir string, description string, input map[string]string) error {
	var action ActionRunner

	actionLog := log.WithField("action", actionID)

	switch actionID {
	case "add_file":
		action = NewAddFileAction(dir, description, input)
	case "replace":
		action = NewReplaceAction(dir, description, input, globalConfig.Options.IgnoreDirectories)
	case "run_command":
		action = NewRunCommandAction(dir, description, input)
	case "yaml":
		action = NewYAMLAction(dir, description, input)
	default:
		return fmt.Errorf("unrecognised command: %s", actionID)
	}

	return action.Run(actionLog)
}
