// Setup GitHub client
package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/avast/retry-go/v4"
	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v63/github"
	"github.com/sirupsen/logrus"
	"github.com/thejokersthief/banshee/v2/pkg/configs"
	"github.com/thejokersthief/banshee/v2/pkg/gitcli"
	"golang.org/x/oauth2"
)

type GithubClient struct {
	Client       *github.Client
	GlobalConfig *configs.GlobalConfig

	git             gitcli.Git
	log             *logrus.Entry
	tokenRefreshItr *ghinstallation.Transport
	accessToken     string
	ctx             context.Context
}

var (
	errAppConfigMissing   = errors.New("config missing for AppID, InstallationID or PrivateKeyPath")
	errTokenConfigMissing = errors.New("config missing for GitHub Token")
)

// Build a new GitHub client using the global config
func NewGithubClient(globalConf configs.GlobalConfig, ctx context.Context, log *logrus.Entry) (*GithubClient, error) {
	ghClient := &GithubClient{
		GlobalConfig:    &globalConf,
		ctx:             ctx,
		log:             log.WithField("client", "GithubClient"),
		tokenRefreshItr: nil,
		git:             gitcli.NewExecGit(globalConf.Options.ShowGitOutput, log),
	}

	if globalConf.Github.UseGithubApp {
		return newGithubAppClient(globalConf, ghClient, ctx)
	}

	return newGithubTokenClient(globalConf, ghClient)
}

// Create a GitHub client that uses token authentication
func newGithubTokenClient(globalConf configs.GlobalConfig, ghClient *GithubClient) (*GithubClient, error) {
	configMissing := (globalConf.Github.Token == "")
	if configMissing {
		return nil, errTokenConfigMissing
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: globalConf.Github.Token},
	)
	tc := oauth2.NewClient(ctx, ts)

	ghClient.Client = github.NewClient(tc)
	ghClient.accessToken = globalConf.Github.Token

	return ghClient, nil
}

// Create a GitHub client that uses App authentication
func newGithubAppClient(globalConf configs.GlobalConfig, ghClient *GithubClient, ctx context.Context) (*GithubClient, error) {
	configMissing := (globalConf.Github.AppID == 0 ||
		globalConf.Github.AppInstallationID == 0 ||
		globalConf.Github.AppPrivateKeyPath == "")
	if configMissing {
		return nil, errAppConfigMissing
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
	ghClient.tokenRefreshItr = itr
	if err != nil {
		return nil, err
	}

	return ghClient, nil
}

// freshTokenURL returns a token-embedded HTTPS URL, refreshing the token for GitHub Apps.
func (gc *GithubClient) freshTokenURL(org, repoName string) (string, error) {
	if gc.tokenRefreshItr != nil {
		token, err := gc.tokenRefreshItr.Token(gc.ctx)
		if err != nil {
			return "", fmt.Errorf("refreshing GitHub App token: %w", err)
		}
		gc.accessToken = token
	}
	return fmt.Sprintf("https://x-access-token:%s@github.com/%s/%s.git", gc.accessToken, org, repoName), nil
}

// ShallowClone clones (or opens + pulls) a repo and checks out the migration branch.
// It returns the default branch name so callers do not need a separate GetDefaultBranch call.
func (gc *GithubClient) ShallowClone(org, repoName, dir, migrationBranchName string) (string, error) {
	defaultBranch, err := gc.GetDefaultBranch(org, repoName)
	if err != nil {
		return "", err
	}

	tokenURL, err := gc.freshTokenURL(org, repoName)
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(dir + "/.git"); errors.Is(err, os.ErrNotExist) {
		gc.log.Info("Cloning ", repoName, " [", defaultBranch, "]...")
		if err := gc.git.Clone(tokenURL, dir, defaultBranch, 1); err != nil {
			return "", err
		}
	} else {
		gc.log.Info("Opening ", dir, " [", defaultBranch, "]...")
		if err := gc.git.Checkout(dir, defaultBranch, false); err != nil {
			return "", err
		}
		if err := gc.git.Pull(dir, tokenURL, defaultBranch); err != nil {
			return "", err
		}
	}

	if err := gc.git.Checkout(dir, migrationBranchName, true); err != nil {
		return "", err
	}

	// Refresh token before migration-branch network ops.
	migURL, err := gc.freshTokenURL(org, repoName)
	if err != nil {
		return "", err
	}
	if err := gc.git.Fetch(dir, migURL, migrationBranchName); err != nil {
		return "", err
	}
	if err := gc.git.Pull(dir, migURL, migrationBranchName); err != nil {
		return "", err
	}

	return defaultBranch, nil
}

// Get the default branch set for the repo on GitHub
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
