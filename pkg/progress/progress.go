package progress

import (
	"encoding/json"
	"errors"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/gosimple/slug"
	"github.com/sirupsen/logrus"
	"github.com/thejokersthief/banshee/v2/pkg/configs"
)

type Progress struct {
	ID     string
	Dir    string
	Config *configs.ProgressConfig

	log *logrus.Entry
	mu  sync.Mutex
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
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, repo := range repos {
		p.Config.Repos[repo] = &configs.RepoProgress{}
	}
	writeErr := p.writeProgress()
	if writeErr != nil {
		p.log.Error(writeErr)
	}
}

func (p *Progress) GetRepos() []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	repos := []string{}
	for repo := range p.Config.Repos {
		repos = append(repos, repo)
	}

	return repos
}

func (p *Progress) ProgressFile() string {
	return path.Join(p.Dir, p.ID+".json")
}

func (p *Progress) loadProgress() error {
	if _, err := os.Stat(p.ProgressFile()); errors.Is(err, os.ErrNotExist) {
		// If the file doesn't exist, that's okay - we'll write it
		p.log.Info("Didn't find any progress file at ", p.ProgressFile(), ". Creating new one...")
		p.Config = &configs.ProgressConfig{Repos: make(map[string]*configs.RepoProgress)}
		return p.writeProgress()
	}

	data, readErr := os.ReadFile(p.ProgressFile())
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
	p.log.Debug("Writing progress to ", p.ProgressFile())
	p.Config.LastUpdated = p.hrTimestamp()
	jsonStr, jsonErr := json.Marshal(p.Config)
	if jsonErr != nil {
		return jsonErr
	}

	writeErr := os.WriteFile(p.ProgressFile(), jsonStr, 0666)
	if writeErr != nil {
		return writeErr
	}

	return nil
}

func (p *Progress) hrTimestamp() string {
	return time.Now().String()
}

// reposWhere returns all repos whose progress satisfies the provided predicate.
func (p *Progress) reposWhere(predicate func(*configs.RepoProgress) bool) []string {
	repos := []string{}
	for repo, progress := range p.Config.Repos {
		if predicate(progress) {
			repos = append(repos, repo)
		}
	}
	return repos
}
