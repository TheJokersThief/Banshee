// Do a find and replace for a string during a migration
package actions

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

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

	globPattern := r.BaseDir + "/" + r.Glob
	matches, err := filepathx.Glob(globPattern)
	if err != nil {
		log.WithField("pattern", globPattern).Error("Error globbing file path: ", err)
		return fmt.Errorf("glob %q: %w", globPattern, err)
	}
	matches = r.removeBlacklistedDirectories(matches)

	// Buffer the error channel to hold one entry per file so workers never
	// block on it, even if every file produces an error.
	files := make(chan string, len(matches))
	errChan := make(chan error, len(matches))

	var wg sync.WaitGroup
	for workerCount := 0; workerCount < threadCount; workerCount++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.findAndReplaceWorker(log, files, errChan)
		}()
	}

	for _, match := range matches {
		files <- match
	}
	close(files)

	// Wait for all workers to finish before reading the error channel so that
	// no errors are missed.
	wg.Wait()
	close(errChan)

	var finalError error
	for fileErr := range errChan {
		finalError = fmt.Errorf("%w; %w", finalError, fileErr)
	}
	return finalError
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
		if readErr != nil {
			errChan <- fmt.Errorf("couldn't read file %q: %w", file, readErr)
			continue
		}
		if !strings.Contains(string(content), r.OldString) {
			continue
		}

		log.Debug("Replacing ", r.OldString, " with ", r.NewString, " in ", file)

		f, err := os.Open(file)
		if err != nil {
			errChan <- fmt.Errorf("couldn't open file %q: %w", file, err)
			continue
		}

		// create temp file
		tmp, err := os.CreateTemp(os.TempDir(), "replace-*")
		if err != nil {
			f.Close()
			errChan <- fmt.Errorf("couldn't create temporary file: %w", err)
			continue
		}

		reader := replace.Chain(f,
			replace.String(r.OldString, r.NewString),
		)

		_, err = io.Copy(tmp, reader)
		if err != nil {
			tmp.Close()
			_ = os.Remove(tmp.Name())
			f.Close()
			errChan <- fmt.Errorf("couldn't copy file %q: %w", file, err)
			continue
		}

		// make sure the tmp file was successfully written to
		if err := tmp.Close(); err != nil {
			_ = os.Remove(tmp.Name())
			f.Close()
			errChan <- fmt.Errorf("couldn't close temporary file: %w", err)
			continue
		}

		// close the file we're reading from
		if err := f.Close(); err != nil {
			_ = os.Remove(tmp.Name())
			errChan <- fmt.Errorf("couldn't close source file %q: %w", file, err)
			continue
		}

		// overwrite the original file with the temp file
		if err := os.Rename(tmp.Name(), file); err != nil {
			_ = os.Remove(tmp.Name())
			errChan <- fmt.Errorf("couldn't rename temporary file to %q: %w", file, err)
			continue
		}
	}
}
