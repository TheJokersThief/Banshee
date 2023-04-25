// Do a find and replace for a string during a migration
package actions

import (
	"os/exec"
	"strings"
)

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

func (r *RunCommand) Run() error {
	commandPieces := strings.Split(r.Command, " ")
	command := commandPieces[0]
	args := commandPieces[1 : len(commandPieces)-1]

	cmd := exec.Command(command, args...)
	cmd.Dir = r.BaseDir
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
