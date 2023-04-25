// Setup GitHub client
package github

import (
	"context"
	"fmt"
	"net/http"

	"github.com/avast/retry-go/v4"
	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	gitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v52/github"
	"github.com/sirupsen/logrus"
	"github.com/thejokersthief/banshee/pkg/configs"
	"golang.org/x/oauth2"
)

type GithubClient struct {
	Client *github.Client

	accessToken string
	ctx         context.Context
}

func NewGithubClient(globalConf configs.GlobalConfig, ctx context.Context) (*GithubClient, error) {
	ghClient := &GithubClient{ctx: ctx}
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
		ghClient.Client = github.NewClient(&http.Client{Transport: itr})
		ghClient.accessToken, err = itr.Token(ctx)
		if err != nil {
			return nil, err
		}
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

		ghClient.Client = github.NewClient(tc)
		ghClient.accessToken = globalConf.Github.Token
	}

	return ghClient, nil
}

func (gc *GithubClient) ShallowClone(repoFullName, dir, migrationBranchName string) (*git.Repository, error) {
	repo, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL: fmt.Sprintf("https://github.com/%s.git", repoFullName),
		Auth: &gitHttp.BasicAuth{
			Username: "placeholderUsername", // anything except an empty string
			Password: gc.accessToken,
		},
		// Depth: 1 // Unfortunately there's an issue in go-git that means using depth breaks the working tree
	})

	if err != nil {
		return nil, err
	}

	wt, wtErr := repo.Worktree()
	if wtErr != nil {
		return nil, wtErr
	}

	h, headErr := repo.Head()
	if headErr != nil {
		return nil, headErr
	}

	checkoutErr := wt.Checkout(
		&git.CheckoutOptions{
			Hash:   h.Hash(),
			Branch: plumbing.ReferenceName("refs/heads/" + migrationBranchName),
			Create: true,
			Keep:   true,
		},
	)
	if checkoutErr != nil {
		return nil, checkoutErr
	}

	logrus.Debug("Pulling ", plumbing.NewBranchReferenceName(migrationBranchName))
	pullErr := wt.Pull(&git.PullOptions{
		RemoteName:    "origin",
		ReferenceName: plumbing.NewBranchReferenceName(migrationBranchName),
		Auth: &gitHttp.BasicAuth{
			Username: "placeholderUsername", // anything except an empty string
			Password: gc.accessToken,
		},
	})
	if pullErr != nil && (pullErr != git.NoErrAlreadyUpToDate && pullErr.Error() != "reference not found") {
		return nil, pullErr
	}

	return repo, nil
}

func (gc *GithubClient) Push(branch string, gitRepo *git.Repository) error {
	logrus.Debug("Pushing changes")

	pushErr := gitRepo.Push(
		&git.PushOptions{
			RemoteName: "origin",
			Auth: &gitHttp.BasicAuth{
				Username: "placeholderUsername", // anything except an empty string
				Password: gc.accessToken,
			},
			RefSpecs: []config.RefSpec{
				config.RefSpec(plumbing.NewBranchReferenceName(branch) + ":" + plumbing.NewBranchReferenceName(branch)),
			},
		},
	)

	return pushErr
}

func (gc *GithubClient) GetDefaultBranch(owner, repo string) (string, error) {
	var ghRepo *github.Repository
	searchErr := retry.Do(
		func() error {
			var err error
			ghRepo, _, err = gc.Client.Repositories.Get(gc.ctx, owner, repo)
			return checkIfRecoverable(err)
		},
		defaultRetryOptions...,
	)

	if searchErr != nil {
		return "", searchErr
	}

	return *ghRepo.DefaultBranch, nil
}
