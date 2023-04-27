package core

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/sirupsen/logrus"
	"github.com/thejokersthief/banshee/pkg/actions"
)

func (b *Banshee) Migrate() error {

	b.log = logrus.WithField("command", "migrate")

	org := b.MigrationConfig.Organisation
	if b.MigrationConfig.Organisation == "" {
		org = b.GlobalConfig.Defaults.Organisation
	}

	if (len(b.MigrationConfig.ListOfRepos) > 0) == (b.MigrationConfig.SearchQuery != "") {
		return fmt.Errorf("You may only use one of search_query or repos")
	}

	if b.GlobalConfig.Options.CacheRepos.Enabled {
		cacheErr := b.createCacheRepo(b.log, b.GlobalConfig.Options.CacheRepos.Directory)
		if cacheErr != nil {
			return cacheErr
		}
	}

	var repos []string
	if len(b.MigrationConfig.ListOfRepos) > 0 {
		repos = b.MigrationConfig.ListOfRepos
	}

	if b.MigrationConfig.SearchQuery != "" {
		var searchQueryErr error
		var query string

		if !strings.Contains(b.MigrationConfig.SearchQuery, "org:") {
			query = fmt.Sprintf("org:%s %s", org, b.MigrationConfig.SearchQuery)
		}
		repos, searchQueryErr = b.GithubClient.GetMatchingRepos(query)
		if searchQueryErr != nil {
			return searchQueryErr
		}
	}

	for _, repo := range repos {
		if !strings.Contains(repo, "/") {
			repo = fmt.Sprintf("%s/%s", org, repo)
		}

		_, repoErr := b.handleRepo(b.log.WithField("repo", repo), org, repo)
		if repoErr != nil {
			return repoErr
		}
	}
	return nil
}

func (b *Banshee) createCacheRepo(log *logrus.Entry, path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *Banshee) getCacheRepoPath(org, repo string) string {
	return fmt.Sprintf("%s/%s-%s", b.GlobalConfig.Options.CacheRepos.Directory, org, repo)
}

// Handle the migration for a repo
func (b *Banshee) handleRepo(log *logrus.Entry, org, repo string) (string, error) {
	madeChanges := false
	repoNameOnly := strings.Replace(repo, org+"/", "", -1)

	log.Info("Processing ", repo)

	dir, gitRepo, defaultBranch, cloneErr := b.cloneRepo(log, org, repo)
	if cloneErr != nil {
		return "", cloneErr
	}

	if !b.GlobalConfig.Options.CacheRepos.Enabled {
		// If we're not caching repos, delete the repo directory when this function returns
		defer os.RemoveAll(dir)
	}

	for _, action := range b.MigrationConfig.Actions {
		actionErr := actions.RunAction(log, action.Action, dir, action.Description, action.Input)
		if actionErr != nil {
			return "", actionErr
		}

		tree, _ := gitRepo.Worktree()
		state, _ := tree.Status()
		log.Debug("Checking if dirty...")
		// check if git dirty
		if !state.IsClean() {
			log.Debug("Is dirty, committing changes: ", action.Description)
			// if dirty, commit with action.Description as message
			madeChanges = true
			addErr := tree.AddGlob("./")
			if addErr != nil {
				return "", addErr
			}

			_, commitErr := tree.Commit(action.Description, &git.CommitOptions{
				Author: &object.Signature{
					Name:  b.GlobalConfig.Defaults.GitName,
					Email: b.GlobalConfig.Defaults.GitEmail,
					When:  time.Now(),
				},
			})

			if commitErr != nil {
				return "", commitErr
			}
		}
	}

	if madeChanges {
		// If we made at least one change, push to the remote
		htmlURL, err := b.pushChanges(gitRepo, org, repoNameOnly, defaultBranch)
		if err != nil {
			return "", err
		}

		if htmlURL == "" {
			log.Info("PR already exists, not creating one")
			return "", nil
		}

		log.Info("Created PR for ", repo, ": ", htmlURL)
		return htmlURL, nil
	}

	return "", nil
}

// Push changes to aGitHub nd create a Pull Request
func (b *Banshee) pushChanges(gitRepo *git.Repository, org, repoName, defaultBranch string) (string, error) {
	pushError := b.GithubClient.Push(b.MigrationConfig.BranchName, gitRepo)
	if pushError != nil {
		return "", fmt.Errorf("push error: %s", pushError)
	}

	bodyText, prFileErr := os.ReadFile(b.MigrationConfig.PRBodyFile)
	if prFileErr != nil {
		return "", prFileErr
	}

	htmlURL, prErr := b.GithubClient.CreatePullRequest(
		org, repoName, b.MigrationConfig.PRTitle, string(bodyText), defaultBranch, b.MigrationConfig.BranchName)
	if prErr != nil {
		return "", prErr
	}

	return htmlURL, nil
}

// Clone a new repo, and fetch info about its default branch
func (b *Banshee) cloneRepo(log *logrus.Entry, org, repo string) (string, *git.Repository, string, error) {
	repoNameOnly := strings.Replace(repo, org+"/", "", -1)

	var dir string
	var mkDirErr error
	if b.GlobalConfig.Options.CacheRepos.Enabled {
		dir = b.getCacheRepoPath(org, repoNameOnly)
		mkDirErr = b.createCacheRepo(log, dir)
	} else {
		dir, mkDirErr = os.MkdirTemp(os.TempDir(), strings.Replace(repo, "/", "-", -1))
	}
	if mkDirErr != nil {
		return "", nil, "", mkDirErr
	}

	logrus.Debug("Using ", dir)

	gitRepo, cloneErr := b.GithubClient.ShallowClone(org, repoNameOnly, dir, b.MigrationConfig.BranchName)
	if cloneErr != nil {
		return "", nil, "", fmt.Errorf("clone error: %s", cloneErr)
	}

	defaultBranch, defaultBranchErr := b.GithubClient.GetDefaultBranch(org, repoNameOnly)
	if defaultBranchErr != nil {
		return "", nil, "", defaultBranchErr
	}

	return dir, gitRepo, defaultBranch, nil
}
