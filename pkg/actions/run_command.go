// Do a find and replace for a string during a migration
package actions

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
	return nil
}
