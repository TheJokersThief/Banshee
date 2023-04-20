package core

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/github"
	"github.com/thejokersthief/banshee/pkg/configs"
	"golang.org/x/oauth2"
)

type Banshee struct {
	GlobalConfig    *configs.GlobalConfig
	MigrationConfig *configs.MigrationConfig
	GithubClient    *github.Client
}

func NewBanshee(config configs.GlobalConfig, migConfig configs.MigrationConfig) (*Banshee, error) {
	client, err := initGithubClient(config)
	if err != nil {
		return nil, err
	}

	return &Banshee{
		GlobalConfig:    &config,
		MigrationConfig: &migConfig,
		GithubClient:    client,
	}, nil
}

func (b *Banshee) Migrate() error {
	return nil
}

func initGithubClient(globalConf configs.GlobalConfig) (*github.Client, error) {
	if globalConf.Github.UseGithubApp {

		configMissing := (globalConf.Github.AppID == 0 ||
			globalConf.Github.AppInstallationID == 0 ||
			globalConf.Github.AppPrivateKeyPath == "")
		if configMissing {
			return nil, fmt.Errorf("Config missing for AppID, InstallationID or PrivateKeyPath")
		}

		itr, err := ghinstallation.NewKeyFromFile(
			http.DefaultTransport,
			globalConf.Github.AppID,
			globalConf.Github.AppInstallationID,
			globalConf.Github.AppPrivateKeyPath,
		)

		if err != nil {
			return nil, err
		}

		// Use installation transport with client.
		return github.NewClient(&http.Client{Transport: itr}), nil
	} else {

		configMissing := (globalConf.Github.Token == "")
		if configMissing {
			return nil, fmt.Errorf("Config missing for GitHub Token")
		}

		ctx := context.Background()
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: globalConf.Github.Token},
		)
		tc := oauth2.NewClient(ctx, ts)

		return github.NewClient(tc), nil
	}
}
