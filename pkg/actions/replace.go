// Do a find and replace for a string during a migration
package actions

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/icholy/replace"
	"github.com/sirupsen/logrus"
	"github.com/yargevad/filepathx"
)

type Replace struct {
	BaseDir   string
	OldString string
	NewString string
	Glob      string

	ignoreDirs []string
}

const threadCount = 10

var defaultDenylistedDirectories = []string{".git", ".idea"}

func NewReplaceAction(dir string, description string, input map[string]string, ignoreDirs []string) *Replace {
	glob, hasSpecifiedGlob := input["glob"]
	if !hasSpecifiedGlob {
		glob = "**"
	}

	denyList := []string{}
	denyList = append(denyList, defaultDenylistedDirectories...)
	denyList = append(denyList, ignoreDirs...)

	return &Replace{
		BaseDir:   dir,
		OldString: input["old"],
		NewString: input["new"],
		Glob:      glob,

		ignoreDirs: denyList,
	}
}

func (r *Replace) Run(log *logrus.Entry) error {
	log.Debug("Replace action: ", r.OldString, " --> ", r.NewString)

	files := make(chan string, 512)
	errChan := make(chan error, math.MaxInt8)
	for workerCount := 0; workerCount < threadCount; workerCount++ {
		go r.findAndReplaceWorker(log, files, errChan)
	}

	globPattern := r.BaseDir + "/" + r.Glob
	matches, err := filepathx.Glob(globPattern)
	if err != nil {
		logrus.WithField("pattern", globPattern).Error("Error globbing file path: ", err)
		return err
	}
	matches = r.removeBlacklistedDirectories(matches)

	for _, match := range matches {
		files <- match
	}
	close(files)

	if len(errChan) != 0 {
		finalError := fmt.Errorf("")
		totalErrs := len(errChan)
		for i := 0; i < totalErrs; i++ {
			fileErr := <-errChan
			finalError = errors.Join(finalError, fileErr)
		}
		return finalError
	}

	return nil
}

func (r *Replace) removeBlacklistedDirectories(matches []string) []string {
	newMatches := []string{}
	for _, match := range matches {

		isAllowed := true
		for _, item := range r.ignoreDirs {
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

func (r *Replace) findAndReplaceWorker(log *logrus.Entry, files <-chan string, errChan chan<- error) {
	for file := range files {
		content, readErr := os.ReadFile(file)
		if !strings.Contains(string(content), r.OldString) || readErr != nil {
			continue
		}

		log.Debug("Replacing ", r.OldString, " with ", r.NewString, " in ", file)

		f, err := os.Open(file)
		if err != nil {
			errChan <- errors.New("couldn't open file: " + err.Error())
			continue
		}

		// create temp file
		tmp, err := os.CreateTemp(os.TempDir(), "replace-*")
		if err != nil {
			errChan <- errors.New("couldn't create temporary file: " + err.Error())
			continue
		}

		reader := replace.Chain(f,
			replace.String(r.OldString, r.NewString),
		)

		_, err = io.Copy(tmp, reader)
		if err != nil {
			errChan <- errors.New("couldn't copy file: " + err.Error())
			continue
		}

		// make sure the tmp file was successfully written to
		if err := tmp.Close(); err != nil {
			errChan <- errors.New("couldn't close file: " + err.Error())
			continue
		}

		// close the file we're reading from
		if err := f.Close(); err != nil {
			errChan <- err
			continue
		}

		// overwrite the original file with the temp file
		if err := os.Rename(tmp.Name(), file); err != nil {
			errChan <- errors.New("couldn't rename file: " + err.Error())
			continue
		}
	}
}
