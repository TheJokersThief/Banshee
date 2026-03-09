// Setup GitHub client
package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/avast/retry-go/v4"
	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v63/github"
	"github.com/sirupsen/logrus"
	"github.com/thejokersthief/banshee/v2/pkg/configs"
	"github.com/thejokersthief/banshee/v2/pkg/gitcli"
	"golang.org/x/oauth2"
	"golang.org/x/time/rate"
)

type GithubClient struct {
	Client       *github.Client
	GlobalConfig *configs.GlobalConfig

	git             gitcli.Git
	log             *logrus.Entry
	tokenRefreshItr *ghinstallation.Transport
	accessToken     string
	ctx             context.Context
	tokenMu         sync.Mutex
	rateLimiter     *rate.Limiter
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
		git:             gitcli.NewExecGit(ctx, globalConf.Options.ShowGitOutput, log),
		rateLimiter:     rate.NewLimiter(rate.Limit(25), 5),
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
		return nil, fmt.Errorf("loading GitHub App private key: %w", err)
	}

	// Use installation transport with client.
	ghClient.Client = github.NewClient(&http.Client{Transport: itr})
	ghClient.tokenRefreshItr = itr
	ghClient.accessToken, err = itr.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching initial GitHub App token: %w", err)
	}

	return ghClient, nil
}

// freshTokenURL returns a token-embedded HTTPS URL, refreshing the token for GitHub Apps.
func (gc *GithubClient) freshTokenURL(org, repoName string) (string, error) {
	gc.tokenMu.Lock()
	defer gc.tokenMu.Unlock()

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

// ensureCacheClone clones the repo into dir (shallow, default branch) or updates
// an existing clone by checking out the default branch and pulling.
func (gc *GithubClient) ensureCacheClone(tokenURL, dir, repoName, defaultBranch string) error {
	if _, err := os.Stat(dir + "/.git"); errors.Is(err, os.ErrNotExist) {
		gc.log.Info("Cloning ", repoName, " [", defaultBranch, "] into cache...")
		return gc.git.Clone(tokenURL, dir, defaultBranch, 1)
	}
	gc.log.Info("Updating cache ", dir, " [", defaultBranch, "]...")
	if err := gc.git.Checkout(dir, defaultBranch, false); err != nil {
		return err
	}
	return gc.git.Pull(dir, tokenURL, defaultBranch)
}

// addWorktreeForBranch creates a worktree, trying an existing branch first
// and falling back to creating a new one if the reference doesn't exist.
func (gc *GithubClient) addWorktreeForBranch(repoDir, worktreeDir, branch string) error {
	firstErr := gc.git.WorktreeAdd(repoDir, worktreeDir, branch, false)
	if firstErr == nil {
		return nil
	}
	var ge *gitcli.GitError
	if !errors.As(firstErr, &ge) || !strings.Contains(ge.Stderr, "invalid reference") {
		return fmt.Errorf("worktree add: %w", firstErr)
	}
	if createErr := gc.git.WorktreeAdd(repoDir, worktreeDir, branch, true); createErr != nil {
		return fmt.Errorf("worktree add: %w", createErr)
	}
	return nil
}

// ShallowCloneWorktree clones (or updates) the cached repo on the default branch,
// then creates a git worktree for the migration branch. Returns the default branch name.
func (gc *GithubClient) ShallowCloneWorktree(org, repoName, cacheDir, worktreeDir, migrationBranchName string) (string, error) {
	defaultBranch, err := gc.GetDefaultBranch(org, repoName)
	if err != nil {
		return "", err
	}

	tokenURL, err := gc.freshTokenURL(org, repoName)
	if err != nil {
		return "", err
	}
	if err := gc.ensureCacheClone(tokenURL, cacheDir, repoName, defaultBranch); err != nil {
		return "", err
	}

	// Fetch the migration branch from remote (swallow "not found").
	fetchURL, err := gc.freshTokenURL(org, repoName)
	if err != nil {
		return "", err
	}
	if err := gc.git.Fetch(cacheDir, fetchURL, migrationBranchName); err != nil {
		return "", err
	}

	if err := gc.addWorktreeForBranch(cacheDir, worktreeDir, migrationBranchName); err != nil {
		return "", err
	}

	// If the branch existed remotely, pull latest into the worktree.
	pullURL, err := gc.freshTokenURL(org, repoName)
	if err != nil {
		return "", err
	}
	if pullErr := gc.git.Pull(worktreeDir, pullURL, migrationBranchName); pullErr != nil {
		// ErrReferenceNotFound means the branch is new (local only) — safe to ignore.
		if !errors.Is(pullErr, gitcli.ErrReferenceNotFound) {
			return "", pullErr
		}
		gc.log.Debug("Migration branch is new, skipping pull in worktree")
	}

	return defaultBranch, nil
}

// Get the default branch set for the repo on GitHub
func (gc *GithubClient) GetDefaultBranch(owner, repo string) (string, error) {
	if err := gc.waitRateLimit(); err != nil {
		return "", err
	}

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

	return ghRepo.GetDefaultBranch(), nil
}

// waitRateLimit blocks until the shared rate limiter allows the next request.
func (gc *GithubClient) waitRateLimit() error {
	return gc.rateLimiter.Wait(gc.ctx)
}
