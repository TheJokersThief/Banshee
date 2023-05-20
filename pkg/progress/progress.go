package progress

import (
	"encoding/json"
	"errors"
	"os"
	"path"
	"strings"
	"time"

	"github.com/gosimple/slug"
	"github.com/sirupsen/logrus"
	"github.com/thejokersthief/banshee/pkg/configs"
)

type Progress struct {
	ID     string
	Dir    string
	Config *configs.ProgressConfig

	log *logrus.Entry
}

func GenerateProgressID(org, branchName string) string {
	return slug.Make(strings.Join([]string{org, branchName}, "_"))
}

func NewProgress(log *logrus.Entry, progressDir, progressID string) (*Progress, error) {
	progress := Progress{
		ID:  progressID,
		Dir: progressDir,
		log: log,
	}

	loadErr := progress.loadProgress()
	return &progress, loadErr
}

func (p *Progress) AddRepos(repos []string) {
	for _, repo := range repos {
		p.Config.Repos[repo] = &configs.RepoProgress{}
	}
	p.writeProgress()
}

func (p *Progress) GetRepos() []string {
	repos := []string{}
	for repo := range p.Config.Repos {
		repos = append(repos, repo)
	}

	return repos
}

func (p *Progress) progressFile() string {
	return path.Join(p.Dir, p.ID+".json")
}

func (p *Progress) loadProgress() error {
	if _, err := os.Stat(p.progressFile()); errors.Is(err, os.ErrNotExist) {
		// If the file doesn't exist, that's okay - we'll write it
		p.log.Info("Didn't find any progress file at ", p.progressFile(), ". Creating new one...")
		p.Config = &configs.ProgressConfig{Repos: make(map[string]*configs.RepoProgress)}
		return p.writeProgress()
	}

	data, readErr := os.ReadFile(p.progressFile())
	if readErr != nil {
		return readErr
	}

	var progressConf configs.ProgressConfig
	jsonErr := json.Unmarshal(data, &progressConf)
	if jsonErr != nil {
		return jsonErr
	}
	p.Config = &progressConf
	return nil
}

func (p *Progress) writeProgress() error {
	p.log.Debug("Writing progress to ", p.progressFile())
	p.Config.LastUpdated = p.hrTimestamp()
	jsonStr, jsonErr := json.Marshal(p.Config)
	if jsonErr != nil {
		return jsonErr
	}

	writeErr := os.WriteFile(p.progressFile(), jsonStr, 0666)
	if writeErr != nil {
		return writeErr
	}

	return nil
}

func (p *Progress) hrTimestamp() string {
	return time.Now().String()
}
