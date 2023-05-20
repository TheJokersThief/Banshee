package core

import (
	"errors"

	"github.com/sirupsen/logrus"
)

// Perform a migration
func (b *Banshee) Clone() error {

	b.log = logrus.WithField("command", "clone")

	if !b.GlobalConfig.Options.CacheRepos.Enabled {
		return errors.New("Please set options.cache.enabled to `true` if you want to preclone all the repos")
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

	if b.GlobalConfig.Options.SaveProgress.Enabled {
		repos = b.Progress.GetReposNotCloned()
		if (b.GlobalConfig.Options.SaveProgress.Batch) > 0 {
			repos = repos[:b.GlobalConfig.Options.SaveProgress.Batch]
		}
	}

	b.log.Info("Cloning ", len(repos), " repos")

	for _, repo := range repos {
		_, _, _, cloneErr := b.cloneRepo(b.log, org, repo)
		if cloneErr != nil {
			return cloneErr
		}

		if b.GlobalConfig.Options.SaveProgress.Enabled {
			saveErr := b.Progress.MarkCloned(repo)
			if saveErr != nil {
				b.log.Error(saveErr)
			}
		}
	}
	return nil
}
