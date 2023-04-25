// Setup GitHub client
package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	gitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/github"
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
		Depth: 1,
	})

	if err != nil {
		return nil, err
	}

	_, branchNotFound := repo.Branch(migrationBranchName)
	if branchNotFound != nil {
		branchErr := repo.CreateBranch(&config.Branch{Name: migrationBranchName})
		if branchErr != nil {
			return nil, branchErr
		}
	}

	wt, _ := repo.Worktree()
	checkoutErr := wt.Checkout(&git.CheckoutOptions{Branch: plumbing.ReferenceName(migrationBranchName)})
	if checkoutErr != nil {
		return nil, checkoutErr
	}

	return repo, nil
}

func (gc *GithubClient) GetDefaultBranch(repo *git.Repository) (string, error) {
	ref, err := repo.Reference("refs/remotes/origin/HEAD", true)
	if err != nil {
		return "", err
	}

	return strings.Replace(ref.String(), "refs/remotes/origin/", "", -1), nil
}
