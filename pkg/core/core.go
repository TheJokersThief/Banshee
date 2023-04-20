package core

import (
	"context"
	"fmt"
	"os"

	"github.com/thejokersthief/banshee/pkg/configs"
	localGH "github.com/thejokersthief/banshee/pkg/github"
)

type Banshee struct {
	GlobalConfig    *configs.GlobalConfig
	MigrationConfig *configs.MigrationConfig
	GithubClient    *localGH.GithubClient

	ctx context.Context
}

func NewBanshee(config configs.GlobalConfig, migConfig configs.MigrationConfig) (*Banshee, error) {
	ctx := context.Background()
	client, err := localGH.NewGithubClient(config, ctx)
	if err != nil {
		return nil, err
	}

	if token, tokenPresent := os.LookupEnv("GITHUB_TOKEN"); tokenPresent {
		config.Github.Token = token
	}

	return &Banshee{
		GlobalConfig:    &config,
		MigrationConfig: &migConfig,
		GithubClient:    client,

		ctx: ctx,
	}, nil
}

func (b *Banshee) Migrate() error {

	org := b.MigrationConfig.Organisation
	if b.MigrationConfig.Organisation == "" {
		org = b.GlobalConfig.Defaults.Organisation
	}

	query := fmt.Sprintf("org:%s %s", org, b.MigrationConfig.SearchQuery)
	repos, err := b.GithubClient.GetMatchingRepos(query)
	if err != nil {
		return err
	}

	for _, repo := range repos {
		fmt.Println(repo)
	}

	// Get list of repos
	// For every repo:
	//		Shallow clone repo
	//		Create new git branch
	//		for each action
	// 			perform the action
	//			add changed files and commit with action description
	// 		Create a PR the changes
	return nil
}
