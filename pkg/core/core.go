package core

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/thejokersthief/banshee/pkg/actions"
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

	if token, tokenPresent := os.LookupEnv("GITHUB_TOKEN"); tokenPresent && config.Github.Token == "" {
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

	// query := fmt.Sprintf("org:%s %s", org, b.MigrationConfig.SearchQuery)
	// repos, err := b.GithubClient.GetMatchingRepos(query)
	// if err != nil {
	// 	return err
	// }

	repos := []string{fmt.Sprintf("%s/containers", org)}

	for _, repo := range repos {
		madeChanges := false

		dir, err := os.MkdirTemp(os.TempDir(), strings.Replace(repo, "/", "-", -1))
		if err != nil {
			log.Fatal(err)
		}

		defer os.RemoveAll(dir) // clean up
		fmt.Printf("created %s\n", dir)

		gitRepo, cloneErr := b.GithubClient.ShallowClone(repo, dir, b.MigrationConfig.BranchName)
		if cloneErr != nil {
			return cloneErr
		}
		for _, action := range b.MigrationConfig.Actions {
			actionErr := actions.RunAction(action.Action, dir, action.Description, action.Input)
			if actionErr != nil {
				return actionErr
			}

			tree, _ := gitRepo.Worktree()
			state, _ := tree.Status()
			// check if git dirty
			if !state.IsClean() {
				madeChanges = true
				// if dirty, commit with action.Description as message
				tree.AddGlob(dir)
				tree.Commit(action.Description, &git.CommitOptions{
					Author: &object.Signature{
						Name:  b.GlobalConfig.Defaults.GitName,
						Email: b.GlobalConfig.Defaults.GitEmail,
						When:  time.Now(),
					},
				})
			}
		}

		if madeChanges {
			// If we made at least one change, push to the remote
			gitRepo.Push(&git.PushOptions{})
		}

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
