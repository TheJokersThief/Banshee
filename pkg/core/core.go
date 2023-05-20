package core

import (
	"context"
	"errors"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/thejokersthief/banshee/pkg/configs"
	localGH "github.com/thejokersthief/banshee/pkg/github"
	"github.com/thejokersthief/banshee/pkg/progress"
)

type Banshee struct {
	GlobalConfig    *configs.GlobalConfig
	MigrationConfig *configs.MigrationConfig
	GithubClient    *localGH.GithubClient
	Progress        *progress.Progress

	log *logrus.Entry
	ctx context.Context
}

func NewBanshee(config configs.GlobalConfig, migConfig configs.MigrationConfig) (*Banshee, error) {

	lvl, lvlErr := logrus.ParseLevel(config.Options.LogLevel)
	if lvlErr != nil {
		return nil, lvlErr
	}
	logrus.SetLevel(lvl)

	logger := logrus.New()
	log := logger.WithField("command", "unset")
	log.Logger.SetLevel(lvl)

	ctx := context.Background()
	client, err := localGH.NewGithubClient(config, ctx, log)
	if err != nil {
		return nil, err
	}

	b := Banshee{
		GlobalConfig:    &config,
		MigrationConfig: &migConfig,
		GithubClient:    client,

		log: log,
		ctx: ctx,
	}

	progressID := progress.GenerateProgressID(b.getOrgName(), b.MigrationConfig.BranchName)
	createErr := b.createCacheRepo(b.log, config.Options.SaveProgress.Directory)
	if createErr != nil {
		return nil, createErr
	}

	progress, progressErr := progress.NewProgress(log, config.Options.SaveProgress.Directory, progressID)
	if progressErr != nil {
		return nil, progressErr
	}

	b.Progress = progress
	return &b, nil
}

func (b *Banshee) getOrgName() string {
	org := b.MigrationConfig.Organisation
	if b.MigrationConfig.Organisation == "" {
		org = b.GlobalConfig.Defaults.Organisation
		b.log.Debug("No organisation chosen, using ", org)
	}

	return org
}

func (b *Banshee) createCacheRepo(log *logrus.Entry, path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		log.Debug("Creating cache directory ", path)
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			return err
		}
	}

	return nil
}
