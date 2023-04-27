package core

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/thejokersthief/banshee/pkg/configs"
	localGH "github.com/thejokersthief/banshee/pkg/github"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

type Banshee struct {
	GlobalConfig    *configs.GlobalConfig
	MigrationConfig *configs.MigrationConfig
	GithubClient    *localGH.GithubClient

	log *logrus.Entry
	ctx context.Context
}

func NewBanshee(config configs.GlobalConfig, migConfig configs.MigrationConfig) (*Banshee, error) {
	log := logrus.WithField("command", "unset")

	ctx := context.Background()
	client, err := localGH.NewGithubClient(config, ctx, log)
	if err != nil {
		return nil, err
	}

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
