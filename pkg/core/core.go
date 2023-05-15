package core

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/thejokersthief/banshee/pkg/configs"
	localGH "github.com/thejokersthief/banshee/pkg/github"
)

type Banshee struct {
	GlobalConfig    *configs.GlobalConfig
	MigrationConfig *configs.MigrationConfig
	GithubClient    *localGH.GithubClient

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

	// It's a common pattern to set a personal accesstoken in the environment under this name
	if token, tokenPresent := os.LookupEnv("GITHUB_TOKEN"); tokenPresent && config.Github.Token == "" {
		config.Github.Token = token
	}

	return &Banshee{
		GlobalConfig:    &config,
		MigrationConfig: &migConfig,
		GithubClient:    client,

		log: log,
		ctx: ctx,
	}, nil
}
