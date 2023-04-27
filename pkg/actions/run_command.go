// Do a find and replace for a string during a migration
package actions

import (
	"fmt"
	"os/exec"

	"github.com/sirupsen/logrus"
)

const defaultShell = "bash"

type RunCommand struct {
	BaseDir string
	Command string `fig:"command"`
}

func NewRunCommandAction(dir string, description string, input map[string]string) *RunCommand {
	return &RunCommand{
		BaseDir: dir,
		Command: input["command"],
	}
}

func (r *RunCommand) Run(log *logrus.Entry) error {
	log.Debug("Running ", defaultShell, " -c `", r.Command, "`")

	cmd := exec.Command(defaultShell, "-c", r.Command)
	cmd.Stdout = log.WriterLevel(logrus.DebugLevel)
	cmd.Stderr = log.WriterLevel(logrus.ErrorLevel)
	cmd.Dir = r.BaseDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Failed running `%s`: %s", r.Command, err)
	}

	return nil
}
