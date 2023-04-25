// Do a find and replace for a string during a migration
package actions

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/icholy/replace"
	"github.com/sirupsen/logrus"
	"github.com/yargevad/filepathx"
)

type Replace struct {
	BaseDir   string
	OldString string `fig:"old"`
	NewString string `fig:"new"`
	Glob      string `fig:"glob" default:"**"`
}

const threadCount = 10

var blacklistedDirectories = []string{".git"}

func NewReplaceAction(dir string, description string, input map[string]string) *Replace {
	glob, hasSpecifiedGlob := input["glob"]
	if !hasSpecifiedGlob {
		glob = "**"
	}

	return &Replace{
		BaseDir:   dir,
		OldString: input["old"],
		NewString: input["new"],
		Glob:      glob,
	}
}

func (r *Replace) Run(log *logrus.Entry) error {

	files := make(chan string)
	errors := make(chan error)
	for workerCount := 0; workerCount < threadCount; workerCount++ {
		go r.findAndReplaceWorker(log, files, errors)
	}

	matches, err := filepathx.Glob(r.BaseDir + "/" + r.Glob)
	if err != nil {
		return err
	}
	matches = r.removeBlacklistedDirectories(matches)

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

func (r *Replace) removeBlacklistedDirectories(matches []string) []string {
	newMatches := []string{}
	for _, match := range matches {

		isAllowed := true
		for _, item := range blacklistedDirectories {
			if strings.Contains(match, item) {
				isAllowed = false
				break
			}
		}

		if isAllowed {
			newMatches = append(newMatches, match)
		}
	}

	return newMatches
}

func (r *Replace) findAndReplaceWorker(log *logrus.Entry, files <-chan string, errors chan<- error) {
	for file := range files {
		content, _ := os.ReadFile(file)
		if !strings.Contains(string(content), r.OldString) {
			continue
		}

		log.Debug("Replacing ", r.OldString, " with ", r.NewString, " in ", file)

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
