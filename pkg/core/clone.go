package core

import (
	"errors"

	"github.com/sirupsen/logrus"
)

// Perform a migration
func (b *Banshee) Clone() error {

	b.log = logrus.WithField("command", "clone")

	if !b.GlobalConfig.Options.CacheRepos.Enabled {
		return errors.New("options.cache_repos.enabled must be true to use the clone command")
	}

	if validationErr := b.validateMigrateCommand(); validationErr != nil {
		return validationErr
	}

	if cacheErr := b.CreateCacheRepoIfEnabled(); cacheErr != nil {
		return cacheErr
	}

	b.log.Info("Fetching list of repos to clone")
	org, repos, optionsErr := b.migrationOptions()
	if optionsErr != nil {
		return optionsErr
	}

	if b.Progress != nil {
		repos = b.applyBatchLimit(b.Progress.GetReposNotCloned())
	}

	b.log.Info("Cloning ", len(repos), " repos")

	for _, repo := range repos {
		_, _, cloneErr := b.cloneRepo(b.log, org, repo)
		if cloneErr != nil {
			return cloneErr
		}

		if b.Progress != nil {
			saveErr := b.Progress.MarkCloned(repo)
			if saveErr != nil {
				b.log.Error(saveErr)
			}
		}
	}
	return nil
}
