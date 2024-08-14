// Do a find and replace for a string during a migration
package actions

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/sirupsen/logrus"
	"github.com/thejokersthief/banshee/pkg/configs"
)

const defaultShell = "bash"

type RunCommand struct {
	BaseDir      string
	Command      string
	GlobalConfig *configs.GlobalConfig
}

func NewRunCommandAction(dir string, description string, input map[string]string, globalConfig *configs.GlobalConfig) *RunCommand {
	return &RunCommand{
		BaseDir:      dir,
		Command:      input["command"],
		GlobalConfig: globalConfig,
	}
}

func (r *RunCommand) Run(log *logrus.Entry) error {
	log.Debug("Running ", defaultShell, " -c `", r.Command, "`")

	cmd := exec.Command(defaultShell, "-c", r.Command)
	cmd.Stdout = log.WriterLevel(logrus.DebugLevel)
	cmd.Stderr = log.WriterLevel(logrus.ErrorLevel)
	cmd.Env = append(
		os.Environ(),
		"MIGRATION_DIR="+r.GlobalConfig.MigrationDir,
	)
	cmd.Dir = r.BaseDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed running `%s`: %w", r.Command, err)
	}

	return nil
}
