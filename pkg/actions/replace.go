// Do a find and replace for a string during a migration
package actions

type Replace struct {
	BaseDir   string
	OldString string `fig:"old"`
	NewString string `fig:"new"`
}

func NewReplaceAction(dir string, description string, input map[string]string) *Replace {
	return &Replace{
		BaseDir:   dir,
		OldString: input["old"],
		NewString: input["new"],
	}
}

func (r *Replace) Run() error {
	return nil
}
