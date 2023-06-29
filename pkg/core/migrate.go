package core

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/google/go-github/v52/github"
	"github.com/sirupsen/logrus"
	"github.com/thejokersthief/banshee/pkg/actions"
)

// Perform a migration
func (b *Banshee) Migrate() error {

	b.log = logrus.WithField("command", "migrate")

	if validationErr := b.validateMigrateCommand(); validationErr != nil {
		return validationErr
	}

	if cacheErr := b.CreateCacheRepoIfEnabled(); cacheErr != nil {
		return cacheErr
	}

	org, repos, optionsErr := b.migrationOptions()
	if optionsErr != nil {
		return optionsErr
	}

	if b.Progress != nil {
		repos = b.Progress.GetReposNotMigrated()
		if (b.GlobalConfig.Options.SaveProgress.Batch) > 0 {
			repos = repos[:b.GlobalConfig.Options.SaveProgress.Batch]
		}
	}

	for _, repo := range repos {
		// Check if repo is of the form <org>/<repo>
		if !strings.Contains(repo, "/") {
			repo = fmt.Sprintf("%s/%s", org, repo)
		}

		_, repoErr := b.handleRepo(b.log.WithField("repo", repo), org, repo)
		if repoErr != nil {
			return repoErr
		}

		b.log.Println("")
	}
	return nil
}

// Validate migration command options
func (b *Banshee) validateMigrateCommand() error {
	if oneChoiceErr := b.OnlyOneRepoChoice(); oneChoiceErr != nil {
		return oneChoiceErr
	}

	return nil
}

// Handle setting defaults for the migration options
func (b *Banshee) migrationOptions() (string, []string, error) {
	org := b.getOrgName()

	if b.Progress != nil && len(b.Progress.Config.Repos) > 0 {
		return org, b.Progress.GetRepos(), nil
	}

	if len(b.MigrationConfig.ListOfRepos) > 0 {
		return org, b.MigrationConfig.ListOfRepos, nil
	}

	if b.MigrationConfig.SearchQuery != "" {
		query := b.MigrationConfig.SearchQuery
		if !strings.Contains(b.MigrationConfig.SearchQuery, "org:") {
			query = fmt.Sprintf("org:%s %s", org, b.MigrationConfig.SearchQuery)
		}

		repos, searchQueryErr := b.GithubClient.GetMatchingRepos(query)
		return org, b.saveRepos(repos), searchQueryErr
	}

	if b.MigrationConfig.AllReposInOrg {
		allRepos, allReposErr := b.GithubClient.GetAllRepos(org)
		return org, b.saveRepos(allRepos), allReposErr
	}

	return org, []string{}, nil
}

func (b *Banshee) saveRepos(repos []string) []string {
	if b.Progress != nil {
		b.Progress.AddRepos(repos)
	}
	return repos
}

func (b *Banshee) getCacheRepoPath(org, repo string) string {
	return fmt.Sprintf("%s/%s-%s", b.GlobalConfig.Options.CacheRepos.Directory, org, repo)
}

// Handle the migration for a repo
func (b *Banshee) handleRepo(log *logrus.Entry, org, repo string) (string, error) {
	repoNameOnly := strings.ReplaceAll(repo, org+"/", "")

	log.Info("Processing ", repo)
	dir, gitRepo, defaultBranch, cloneErr := b.cloneRepo(log, org, repo)
	if cloneErr != nil {
		return "", cloneErr
	}

	if !b.GlobalConfig.Options.CacheRepos.Enabled {
		// If we're not caching repos, delete the repo directory when this function returns
		defer os.RemoveAll(dir)
	}

	changelog := []string{}
	for _, action := range b.MigrationConfig.Actions {
		actionErr := actions.RunAction(log, b.GlobalConfig, action.Action, dir, action.Description, action.Input)
		if actionErr != nil {
			return "", actionErr
		}

		tree, _ := gitRepo.Worktree()
		state, _ := tree.Status()
		// check if git dirty
		if !state.IsClean() {
			changelog = append(changelog, "* "+action.Description)
			log.Debug("Is dirty, committing changes: ", action.Description)
			// if dirty, commit with action.Description as message
			addErr := tree.AddGlob("./")
			if addErr != nil {
				return "", errors.New("adding error: " + addErr.Error())
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

	htmlURL, err := b.pushChanges(changelog, gitRepo, org, repoNameOnly, defaultBranch)
	if err != nil {
		return "", err
	}

	if b.Progress != nil {
		saveErr := b.Progress.MarkMigrated(repo)
		if saveErr != nil {
			b.log.Error(saveErr)
		}
	}

	if htmlURL != "" {
		log.Info("PR for ", repo, ": \033[32m", htmlURL, "\033[0m")
	}
	return htmlURL, nil
}

// Push changes to aGitHub nd create a Pull Request
func (b *Banshee) pushChanges(changelog []string, gitRepo *git.Repository, org, repoName, defaultBranch string) (string, error) {
	pushError := b.GithubClient.Push(b.MigrationConfig.BranchName, gitRepo)
	if pushError != nil {
		return "", fmt.Errorf("push error: %w", pushError)
	}

	log := b.log.WithField("repo", org+"/"+repoName)

	log.Debug("Searching for pull requests")
	pr, prErr := b.GithubClient.FindPullRequest(org, repoName, defaultBranch, b.MigrationConfig.BranchName)
	if prErr != nil {
		return "", prErr
	}
	log.Debug("Got PR result: ", pr != nil)

	prBody, bodyErr := b.formatChangelog(pr, changelog)
	if bodyErr != nil {
		return "", bodyErr
	}

	if pr != nil {
		log.Debug("Updating pull request with new changelog")
		editErr := b.GithubClient.UpdatePullRequest(pr, prBody)
		if editErr != nil {
			return "", editErr
		}
		return pr.GetHTMLURL(), nil
	}

	log.Debug("Creating pull request")
	htmlURL, prErr := b.GithubClient.CreatePullRequest(
		org, repoName, b.MigrationConfig.PRTitle, prBody, defaultBranch,
		b.MigrationConfig.BranchName, b.MigrationConfig.PRDrafts)
	if prErr != nil {
		return "", prErr
	}

	return htmlURL, nil
}

func (b *Banshee) formatChangelog(pr *github.PullRequest, changelog []string) (string, error) {
	bodyText, prFileErr := os.ReadFile(b.MigrationConfig.PRBodyFile)
	if prFileErr != nil {
		return "", prFileErr
	}

	if pr != nil {
		bodyText = []byte(*pr.Body)
	}

	changelogText := strings.Join(changelog, "\n")
	prBody := strings.ReplaceAll(
		string(bodyText),
		"<!-- changelog -->",
		fmt.Sprintf("<!-- changelog -->\n%s", changelogText),
	)

	return prBody, nil
}

// Clone a new repo, and fetch info about its default branch
func (b *Banshee) cloneRepo(log *logrus.Entry, org, repo string) (string, *git.Repository, string, error) {
	repoNameOnly := strings.ReplaceAll(repo, org+"/", "")

	var dir string
	var mkDirErr error
	if b.GlobalConfig.Options.CacheRepos.Enabled {
		dir = b.getCacheRepoPath(org, repoNameOnly)
		mkDirErr = b.createCacheRepo(log, dir)
	} else {
		dir, mkDirErr = os.MkdirTemp(os.TempDir(), strings.ReplaceAll(repo, "/", "-"))
	}
	if mkDirErr != nil {
		return "", nil, "", mkDirErr
	}

	logrus.Debug("Using ", dir)

	gitRepo, cloneErr := b.GithubClient.ShallowClone(org, repoNameOnly, dir, b.MigrationConfig.BranchName)
	if cloneErr != nil {
		return "", nil, "", fmt.Errorf("clone error: %w", cloneErr)
	}

	defaultBranch, defaultBranchErr := b.GithubClient.GetDefaultBranch(org, repoNameOnly)
	if defaultBranchErr != nil {
		return "", nil, "", defaultBranchErr
	}

	return dir, gitRepo, defaultBranch, nil
}
