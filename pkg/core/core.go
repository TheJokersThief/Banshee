package core

import (
	"context"
	"errors"
	"os"

	gogithub "github.com/google/go-github/v63/github"
	"github.com/sirupsen/logrus"
	"github.com/thejokersthief/banshee/v2/pkg/configs"
	localGH "github.com/thejokersthief/banshee/v2/pkg/github"
	"github.com/thejokersthief/banshee/v2/pkg/progress"
)

// githubClient is the subset of *localGH.GithubClient used by Banshee.
// Defined as an interface to allow injection of test doubles.
type githubClient interface {
	ShallowClone(org, repoName, dir, migrationBranchName string) (string, error)
	ShallowCloneWorktree(org, repoName, cacheDir, worktreeDir, migrationBranchName string) (string, error)
	GitWorktreeRemove(repoDir, worktreeDir string) error
	GitWorktreePrune(repoDir string) error
	GetDefaultBranch(owner, repo string) (string, error)
	GitIsClean(dir string) (bool, error)
	GitAddAll(dir string) error
	GitCommit(dir, message, name, email string) error
	Push(branch, dir, org, repoName string) error
	FindPullRequest(org, repo, baseBranch, headBranch string) (*gogithub.PullRequest, error)
	CreatePullRequest(org, repo, title, body, baseBranch, mergeBranch string, asDraft bool) (string, error)
	UpdatePullRequest(pr *gogithub.PullRequest, body string) error
	MergePullRequest(pr *gogithub.PullRequest) error
	GetAllRepos(owner string) ([]string, error)
	GetMatchingRepos(query string) ([]string, error)
	GetMatchingPRs(query string) ([]*gogithub.PullRequest, error)
}

type Banshee struct {
	GlobalConfig    *configs.GlobalConfig
	MigrationConfig *configs.MigrationConfig
	GithubClient    githubClient
	Progress        *progress.Progress
	DryRun          bool

	log *logrus.Entry
	ctx context.Context
}

func NewBanshee(ctx context.Context, config configs.GlobalConfig, migConfig configs.MigrationConfig) (*Banshee, error) {

	lvl, lvlErr := logrus.ParseLevel(config.Options.LogLevel)
	if lvlErr != nil {
		return nil, lvlErr
	}
	logrus.SetLevel(lvl)

	logger := logrus.New()
	log := logger.WithField("command", "unset")
	log.Logger.SetLevel(lvl)

	client, err := localGH.NewGithubClient(config, ctx, log)
	if err != nil {
		return nil, err
	}

	b := Banshee{
		GlobalConfig:    &config,
		MigrationConfig: &migConfig,
		GithubClient:    client,
		Progress:        nil,

		log: log,
		ctx: ctx,
	}

	if b.GlobalConfig.Options.SaveProgress.Enabled {
		progressID := progress.GenerateProgressID(b.getOrgName(), b.MigrationConfig.BranchName)
		createErr := b.createCacheRepo(b.log, config.Options.SaveProgress.Directory)
		if createErr != nil {
			return nil, createErr
		}

		progress, progressErr := progress.NewProgress(log, config.Options.SaveProgress.Directory, progressID)
		if progressErr != nil {
			return nil, progressErr
		}

		b.Progress = progress
	}
	return &b, nil
}

func (b *Banshee) getOrgName() string {
	org := b.MigrationConfig.Organisation
	if b.MigrationConfig.Organisation == "" {
		org = b.GlobalConfig.Defaults.Organisation
		b.log.Debug("No organisation chosen, using ", org)
	}

	return org
}

func (b *Banshee) createCacheRepo(log *logrus.Entry, path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		log.Debug("Creating cache directory ", path)
		err := os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return err
		}
	}

	return nil
}
