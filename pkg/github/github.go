// Setup GitHub client
package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/avast/retry-go/v4"
	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/sideband"
	"github.com/google/go-github/v63/github"
	"github.com/sirupsen/logrus"
	"github.com/thejokersthief/banshee/pkg/configs"
	"golang.org/x/oauth2"
)

type GithubClient struct {
	Client       *github.Client
	Writer       sideband.Progress
	GlobalConfig *configs.GlobalConfig

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

	var showOutput sideband.Progress
	if globalConf.Options.ShowGitOutput {
		showOutput = os.Stdout
	}

	ghClient := &GithubClient{
		GlobalConfig:    &globalConf,
		ctx:             ctx,
		log:             log.WithField("client", "GithubClient"),
		tokenRefreshItr: nil,
		Writer:          showOutput,
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

// Clone the smallest version of a repo we can
func (gc *GithubClient) ShallowClone(org, repoName, dir, migrationBranchName string) (*git.Repository, error) {
	defaultBranch, _ := gc.GetDefaultBranch(org, repoName)
	gitURL := fmt.Sprintf("https://github.com/%s/%s.git", org, repoName)

	var repo *git.Repository
	var plainOpenErr error
	if _, err := os.Stat(dir + "/.git"); errors.Is(err, os.ErrNotExist) {
		// If the directory doesn't exist, clone the repo into it
		gc.log.Info("Cloning ", gitURL, " [", defaultBranch, "]...")
		repo, plainOpenErr = git.PlainClone(dir, false, &git.CloneOptions{
			Progress:      gc.Writer,
			URL:           gitURL,
			Auth:          gc.auth(),
			ReferenceName: plumbing.NewBranchReferenceName(defaultBranch),
			SingleBranch:  true,
			// Depth:         1, // Unfortunately there's an issue in go-git that means using depth breaks the working tree
		})
	} else {
		gc.log.Info("Opening ", dir, " [", defaultBranch, "]...")
		repo, plainOpenErr = git.PlainOpen(dir)

		// Checkout the default branch
		checkoutErr := gc.Checkout(defaultBranch, repo, false)
		if checkoutErr != nil {
			return nil, checkoutErr
		}

		// Pull any changes to the default branch since we last cloned
		pullErr := gc.Pull(defaultBranch, repo)
		if pullErr != nil && (!errors.Is(pullErr, git.NoErrAlreadyUpToDate)) {
			if strings.Contains(pullErr.Error(), "worktree contains unstaged changes") {
				// go-git just straight up can't recover if the worktree has unstaged changes, so we scorch the earth instead
				gc.log.Error("Encountered unrecoverable worktree issue, deleting ", dir, " and recloning repo")
				os.RemoveAll(dir)
				return gc.ShallowClone(org, repoName, dir, migrationBranchName)
			}
			return nil, pullErr
		}
	}

	if plainOpenErr != nil {
		return nil, plainOpenErr
	}

	checkoutErr := gc.Checkout(migrationBranchName, repo, true)
	if checkoutErr != nil {
		return nil, checkoutErr
	}

	fetchErr := gc.Fetch(migrationBranchName, repo)
	if fetchErr != nil {
		return nil, fetchErr
	}

	pullErr := gc.Pull(migrationBranchName, repo)
	if pullErr != nil {
		return nil, pullErr
	}

	return repo, nil
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
