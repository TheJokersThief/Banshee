package core

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-github/v63/github"
	"github.com/sirupsen/logrus"
	"github.com/thejokersthief/banshee/v2/pkg/actions"
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
		if batch := b.GlobalConfig.Options.SaveProgress.Batch; batch < int64(len(repos)) {
			repos = repos[:batch]
		}
	}

	if len(repos) == 0 {
		if b.Progress != nil {
			return fmt.Errorf("Found no repos for migration. Maybe you need to check the progress file? %s", b.Progress.ProgressFile())
		}
		return fmt.Errorf("Found no repos for migration")
	}

	var migrationErrors []error
	for _, repo := range repos {
		// Check if repo is of the form <org>/<repo>
		if !strings.Contains(repo, "/") {
			repo = fmt.Sprintf("%s/%s", org, repo)
		}

		_, repoErr := b.handleRepo(b.log.WithField("repo", repo), org, repo)
		if repoErr != nil {
			b.log.WithField("repo", repo).Errorf("Migration failed: %v", repoErr)
			migrationErrors = append(migrationErrors, fmt.Errorf("%s: %w", repo, repoErr))
		}
		b.log.Println("")
	}
	if len(migrationErrors) > 0 {
		b.log.Errorf("%d repo(s) failed migration", len(migrationErrors))
		return errors.Join(migrationErrors...)
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
		rslt := b.saveRepos(b.MigrationConfig.ListOfRepos)
		return org, rslt, nil
	}

	if b.MigrationConfig.SearchQuery != "" {
		query := b.MigrationConfig.SearchQuery
		if !strings.Contains(b.MigrationConfig.SearchQuery, "org:") {
			query = fmt.Sprintf("org:%s %s", org, b.MigrationConfig.SearchQuery)
		}
		b.log.Debug("Searching code for repos with query: ", query)

		repos, searchQueryErr := b.GithubClient.GetMatchingRepos(query)
		if b.Progress != nil && len(b.Progress.Config.Repos) == 0 {
			b.Progress.AddRepos(repos)
		}
		rslt := b.saveRepos(repos)
		return org, rslt, searchQueryErr
	}

	if b.MigrationConfig.AllReposInOrg {
		allRepos, allReposErr := b.GithubClient.GetAllRepos(org)
		if b.Progress != nil && len(b.Progress.Config.Repos) == 0 {
			b.Progress.AddRepos(allRepos)
		}
		rslt := b.saveRepos(allRepos)
		return org, rslt, allReposErr
	}
	rslt := b.saveRepos(b.MigrationConfig.ListOfRepos)
	return org, rslt, nil
}

func (b *Banshee) saveRepos(repos []string) []string {
	if b.Progress != nil {
		b.log.Debugf("Adding %d repos", len(repos))
		b.Progress.AddRepos(repos)
	}
	return repos
}

func (b *Banshee) getCacheRepoPath(org, repo string) string {
	return fmt.Sprintf("%s/%s-%s", b.GlobalConfig.Options.CacheRepos.Directory, org, repo)
}

// Handle the migration for a repo
func (b *Banshee) handleRepo(log *logrus.Entry, org, repo string) (string, error) {
	repoNameOnly := strings.TrimPrefix(repo, org+"/")

	log.Info("Processing ", repo)
	dir, defaultBranch, cloneErr := b.cloneRepo(log, org, repo)
	if cloneErr != nil {
		return "", cloneErr
	}

	if !b.GlobalConfig.Options.CacheRepos.Enabled {
		// If we're not caching repos, delete the repo directory when this function returns
		defer os.RemoveAll(dir)
	}

	changelog := []string{}
	commitMade := false // Track whether any commits are made as actions run
	for _, action := range b.MigrationConfig.Actions {
		actionErr := actions.RunAction(log, b.GlobalConfig, action.Action, dir, action.Description, action.Input)
		if actionErr != nil {
			return "", actionErr
		}

		isClean, isCleanErr := b.GithubClient.GitIsClean(dir)
		if isCleanErr != nil {
			return "", isCleanErr
		}
		if !isClean {
			changelog = append(changelog, "* "+action.Description)
			log.Debug("Is dirty, committing changes: ", action.Description)
			if addErr := b.GithubClient.GitAddAll(dir); addErr != nil {
				return "", addErr
			}
			if commitErr := b.GithubClient.GitCommit(dir, action.Description,
				b.GlobalConfig.Defaults.GitName,
				b.GlobalConfig.Defaults.GitEmail); commitErr != nil {
				return "", commitErr
			}
			commitMade = true
		}
	}

	if !commitMade {
		log.Info("No changes made for ", repo)
		return "", nil
	}

	htmlURL, err := b.pushChanges(changelog, dir, org, repoNameOnly, defaultBranch)
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
		log.WithField("pr_url", htmlURL).Info("PR for ", repo)
	}
	return htmlURL, nil
}

// Push changes to GitHub and create a Pull Request
func (b *Banshee) pushChanges(changelog []string, dir, org, repoName, defaultBranch string) (string, error) {
	pushError := b.GithubClient.Push(b.MigrationConfig.BranchName, dir, org, repoName)
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
		bodyText = []byte(pr.GetBody())
	}

	changelogText := strings.Join(changelog, "\n")
	prBody := strings.ReplaceAll(
		string(bodyText),
		"<!-- changelog -->",
		fmt.Sprintf("<!-- changelog -->\n%s", changelogText),
	)

	return prBody, nil
}

// cloneRepo clones a repo and returns its dir and default branch.
func (b *Banshee) cloneRepo(log *logrus.Entry, org, repo string) (string, string, error) {
	repoNameOnly := strings.TrimPrefix(repo, org+"/")

	var dir string
	var mkDirErr error
	var cacheCreated bool
	if b.GlobalConfig.Options.CacheRepos.Enabled {
		dir = b.getCacheRepoPath(org, repoNameOnly)
		// Track whether we're creating a new dir so we can clean up on failure.
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			cacheCreated = true
		}
		mkDirErr = b.createCacheRepo(log, dir)
	} else {
		dir, mkDirErr = os.MkdirTemp(os.TempDir(), strings.ReplaceAll(repo, "/", "-"))
	}
	if mkDirErr != nil {
		return "", "", mkDirErr
	}

	log.Debug("Using ", dir)

	defaultBranch, cloneErr := b.GithubClient.ShallowClone(org, repoNameOnly, dir, b.MigrationConfig.BranchName)
	if cloneErr != nil {
		// CORE-I1: clean up a newly created cache dir on clone failure.
		if b.GlobalConfig.Options.CacheRepos.Enabled && cacheCreated {
			_ = os.RemoveAll(dir)
		}
		return "", "", fmt.Errorf("clone error: %w", cloneErr)
	}

	return dir, defaultBranch, nil
}
