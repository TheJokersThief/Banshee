// Do a find and replace for a string during a migration
package actions

import (
	"fmt"
	"io"
	"os"

	"github.com/icholy/replace"
	"github.com/yargevad/filepathx"
)

type Replace struct {
	BaseDir   string
	OldString string `fig:"old"`
	NewString string `fig:"new"`
	Glob      string `fig:"glob" default:"./"`
}

const threadCount = 10

func NewReplaceAction(dir string, description string, input map[string]string) *Replace {
	glob, hasSpecifiedGlob := input["glob"]
	if !hasSpecifiedGlob {
		glob = "./"
	}

	return &Replace{
		BaseDir:   dir,
		OldString: input["old"],
		NewString: input["new"],
		Glob:      glob,
	}
}

func (r *Replace) Run() error {

	files := make(chan string)
	errors := make(chan error)
	for workerCount := 0; workerCount < threadCount; workerCount++ {
		go r.findAndReplaceWorker(files, errors)
	}

	matches, err := filepathx.Glob(r.Glob)
	if err != nil {
		return err
	}

	for _, match := range matches {
		files <- match
	}
	close(files)

	if len(errors) != 0 {
		finalError := fmt.Errorf("")
		for i := 0; i < len(errors); i++ {
			fileErr := <-errors
			finalError = fmt.Errorf("%s\n%s", finalError, fileErr)
		}
		return finalError
	}

	return nil
}

func (r *Replace) findAndReplaceWorker(files <-chan string, errors chan<- error) {
	for file := range files {
		f, err := os.Open(file)
		if err != nil {
			errors <- err
			continue
		}
		defer f.Close()

		// create temp file
		tmp, err := os.CreateTemp(os.TempDir(), "replace-*")
		if err != nil {
			errors <- err
			continue
		}
		defer tmp.Close()

		reader := replace.Chain(f,
			replace.String(r.OldString, r.NewString),
		)

		_, err = io.Copy(tmp, reader)
		if err != nil {
			errors <- err
			continue
		}

		// make sure the tmp file was successfully written to
		if err := tmp.Close(); err != nil {
			errors <- err
			continue
		}

		// close the file we're reading from
		if err := f.Close(); err != nil {
			errors <- err
			continue
		}

		// overwrite the original file with the temp file
		if err := os.Rename(tmp.Name(), file); err != nil {
			errors <- err
			continue
		}
	}
}
