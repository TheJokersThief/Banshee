package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/google/go-github/v63/github"
	"github.com/sirupsen/logrus"
	"github.com/thejokersthief/banshee/v2/pkg/actions"
)

type repoStatus string

const (
	statusCompleted repoStatus = "completed"
	statusFailed    repoStatus = "failed"
	statusCancelled repoStatus = "cancelled"
)

type repoResult struct {
	Repo   string
	Status repoStatus
	Err    error
	PRURL  string
}

// deduplicateRepos returns repos with duplicates removed, preserving order.
func deduplicateRepos(repos []string) []string {
	seen := make(map[string]struct{}, len(repos))
	out := make([]string, 0, len(repos))
	for _, r := range repos {
		if _, dup := seen[r]; !dup {
			seen[r] = struct{}{}
			out = append(out, r)
		}
	}
	return out
}

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
		repos = b.applyBatchLimit(b.Progress.GetReposNotMigrated())
	}

	if len(repos) == 0 {
		if b.Progress != nil {
			return fmt.Errorf("found no repos for migration; check the progress file: %s", b.Progress.ProgressFile())
		}
		return fmt.Errorf("found no repos for migration")
	}

	repos = deduplicateRepos(repos)

	workers := max(b.GlobalConfig.Options.Concurrency, 1)
	results := b.dispatchRepos(org, repos, workers)

	b.printMigrationSummary(results)

	var migrationErrors []error
	for _, r := range results {
		if r.Err != nil {
			migrationErrors = append(migrationErrors, fmt.Errorf("%s: %w", r.Repo, r.Err))
		}
	}
	if len(migrationErrors) > 0 {
		return errors.Join(migrationErrors...)
	}
	return nil
}

// dispatchRepos fans out repo processing across the given number of workers,
// respecting context cancellation for graceful shutdown.
//
// NOTE: We use a manual semaphore+WaitGroup rather than errgroup.SetLimit because
// errgroup.Go blocks without a select, so we cannot check ctx.Done() during the
// wait for a semaphore slot. The manual pattern gives us cancellation-aware backpressure.
func (b *Banshee) dispatchRepos(org string, repos []string, workers int) []repoResult {
	total := len(repos)
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var done atomic.Int32
	var results []repoResult

	// Log when context is cancelled so the user knows shutdown is happening.
	go func() {
		<-b.ctx.Done()
		b.log.Warn("Interrupt received, waiting for in-flight repos to finish...")
	}()

	for i, repo := range repos {
		if !strings.Contains(repo, "/") {
			repo = fmt.Sprintf("%s/%s", org, repo)
		}

		// Check for context cancellation before acquiring a slot.
		select {
		case <-b.ctx.Done():
			mu.Lock()
			results = append(results, repoResult{Repo: repo, Status: statusCancelled})
			mu.Unlock()
			continue
		default:
		}

		// Acquire semaphore slot (or cancel).
		select {
		case <-b.ctx.Done():
			mu.Lock()
			results = append(results, repoResult{Repo: repo, Status: statusCancelled})
			mu.Unlock()
			continue
		case sem <- struct{}{}:
		}

		wg.Add(1)
		b.log.Infof("[start %d/%d] %s", i+1, total, repo)

		go func(repo string) {
			defer wg.Done()
			defer func() { <-sem }()

			// Recover panics so one repo cannot crash the entire migration.
			defer func() {
				if r := recover(); r != nil {
					n := done.Add(1)
					mu.Lock()
					defer mu.Unlock()
					err := fmt.Errorf("panic: %v", r)
					b.log.WithField("repo", repo).Errorf("[done %d/%d] Panicked %s: %v", n, total, repo, err)
					results = append(results, repoResult{Repo: repo, Status: statusFailed, Err: err})
				}
			}()

			prURL, repoErr := b.handleRepo(b.log.WithField("repo", repo), org, repo)
			n := done.Add(1)

			mu.Lock()
			defer mu.Unlock()
			switch {
			case repoErr != nil:
				b.log.WithField("repo", repo).Errorf("[done %d/%d] Failed %s: %v", n, total, repo, repoErr)
				results = append(results, repoResult{Repo: repo, Status: statusFailed, Err: repoErr})
			case prURL != "":
				b.log.Infof("[done %d/%d] Completed %s -> %s", n, total, repo, prURL)
				results = append(results, repoResult{Repo: repo, Status: statusCompleted, PRURL: prURL})
			default:
				b.log.Infof("[done %d/%d] Completed %s (no changes)", n, total, repo)
				results = append(results, repoResult{Repo: repo, Status: statusCompleted})
			}
		}(repo)
	}

	wg.Wait()
	return results
}

func (b *Banshee) printMigrationSummary(results []repoResult) {
	var completed, failed, cancelled int
	for _, r := range results {
		switch r.Status {
		case statusCompleted:
			completed++
		case statusFailed:
			failed++
		case statusCancelled:
			cancelled++
		}
	}

	label := "Migration summary"
	if b.DryRun {
		label = "Migration dry-run summary"
	}
	b.log.Infof("%s: %d completed, %d failed, %d cancelled (total: %d)",
		label, completed, failed, cancelled, len(results))

	for _, r := range results {
		switch r.Status {
		case statusCompleted:
			if r.PRURL != "" {
				b.log.Infof("  OK    %s -> %s", r.Repo, r.PRURL)
			} else {
				b.log.Infof("  OK    %s (no changes)", r.Repo)
			}
		case statusFailed:
			b.log.Infof("  FAIL  %s: %v", r.Repo, r.Err)
		case statusCancelled:
			b.log.Infof("  SKIP  %s (cancelled)", r.Repo)
		}
	}
}

// Validate migration command options
func (b *Banshee) validateMigrateCommand() error {
	if oneChoiceErr := b.OnlyOneRepoChoice(); oneChoiceErr != nil {
		return oneChoiceErr
	}

	if b.GlobalConfig.Options.Concurrency > 1 && !b.GlobalConfig.Options.CacheRepos.Enabled {
		return fmt.Errorf(
			"parallel migration (concurrency > 1) requires repo caching to be enabled\n" +
				"Set 'options.cache_repos.enabled: true' and 'options.cache_repos.directory: /some/path' in your global config.\n" +
				"This is needed so each worker gets an isolated git worktree")
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

// applyBatchLimit trims repos to the configured batch size when SaveProgress is
// enabled and a positive batch value has been set.
func (b *Banshee) applyBatchLimit(repos []string) []string {
	batch := int(b.GlobalConfig.Options.SaveProgress.Batch)
	if batch > 0 && batch < len(repos) {
		return repos[:batch]
	}
	return repos
}

// applyActionAndCommit runs an action and either commits the result (normal mode)
// or logs what would be committed (dry-run mode).
// It returns (true, nil) when a change was detected.
func (b *Banshee) applyActionAndCommit(log *logrus.Entry, dir, actionID, description string, input map[string]string) (bool, error) {
	if err := actions.RunAction(log, b.GlobalConfig, actionID, dir, description, input); err != nil {
		return false, err
	}

	if b.DryRun {
		isClean, err := b.GithubClient.GitIsClean(dir)
		if err != nil {
			return false, err
		}
		if !isClean {
			log.Info("[dry-run] Would commit: ", description)
			// Clean the working tree so uncommitted changes from this action
			// don't leak into the next action's diff check.
			if resetErr := b.GithubClient.GitResetToRef(dir, "HEAD"); resetErr != nil {
				return false, resetErr
			}
			return true, nil
		}
		return false, nil
	}

	return b.commitIfDirty(log, dir, description)
}

// commitIfDirty stages all changes and commits when the working tree is dirty.
// It returns (true, nil) when a commit was made and (false, nil) when clean.
func (b *Banshee) commitIfDirty(log *logrus.Entry, dir, message string) (bool, error) {
	isClean, isCleanErr := b.GithubClient.GitIsClean(dir)
	if isCleanErr != nil {
		return false, isCleanErr
	}
	if isClean {
		return false, nil
	}
	log.Debug("Is dirty, committing changes: ", message)
	if addErr := b.GithubClient.GitAddAll(dir); addErr != nil {
		return false, addErr
	}
	if commitErr := b.GithubClient.GitCommit(dir, message,
		b.GlobalConfig.Defaults.GitName,
		b.GlobalConfig.Defaults.GitEmail); commitErr != nil {
		return false, commitErr
	}
	return true, nil
}

func (b *Banshee) getCacheRepoPath(org, repo string) string {
	return fmt.Sprintf("%s/%s-%s", b.GlobalConfig.Options.CacheRepos.Directory, org, repo)
}

func (b *Banshee) getWorktreePath(org, repo string) string {
	safeBranch := strings.ReplaceAll(b.MigrationConfig.BranchName, "/", "-")
	return fmt.Sprintf("%s/%s-%s-wt/%s",
		b.GlobalConfig.Options.CacheRepos.Directory, org, repo, safeBranch)
}

// Handle the migration for a repo
func (b *Banshee) handleRepo(log *logrus.Entry, org, repo string) (string, error) {
	repoNameOnly := strings.TrimPrefix(repo, org+"/")

	log.Info("Processing ", repo)
	dir, defaultBranch, cloneErr := b.cloneRepo(log, org, repo)
	if cloneErr != nil {
		return "", cloneErr
	}

	if b.GlobalConfig.Options.CacheRepos.Enabled {
		cacheDir := b.getCacheRepoPath(org, repoNameOnly)
		defer func() {
			if rmErr := b.GithubClient.GitWorktreeRemove(cacheDir, dir); rmErr != nil {
				log.Warnf("worktree cleanup failed for %s: %v", dir, rmErr)
				_ = os.RemoveAll(dir) // fallback: just delete the directory
			}
		}()
	} else {
		defer func() { _ = os.RemoveAll(dir) }()
	}

	changelog := []string{}
	commitMade := false // Track whether any commits are made as actions run
	for _, action := range b.MigrationConfig.Actions {
		// Check for cancellation between actions for faster shutdown.
		select {
		case <-b.ctx.Done():
			return "", b.ctx.Err()
		default:
		}

		dirty, actionErr := b.applyActionAndCommit(log, dir, action.Action, action.Description, action.Input)
		if actionErr != nil {
			return "", actionErr
		}
		if dirty {
			changelog = append(changelog, "* "+action.Description)
			commitMade = true
		}
	}

	if !commitMade {
		log.Info("No changes made for ", repo)
		return "", nil
	}

	if b.DryRun {
		log.Info("[dry-run] Would push branch and open/update PR for ", repo)
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

// cloneRepo clones a repo and returns its working dir and default branch.
// When cache_repos is enabled, a worktree is created so the cached clone stays
// on the default branch and each migration gets an isolated directory.
func (b *Banshee) cloneRepo(log *logrus.Entry, org, repo string) (string, string, error) {
	repoNameOnly := strings.TrimPrefix(repo, org+"/")

	if b.GlobalConfig.Options.CacheRepos.Enabled {
		return b.cloneRepoWorktree(log, org, repoNameOnly)
	}

	dir, mkDirErr := os.MkdirTemp(os.TempDir(), strings.ReplaceAll(repo, "/", "-"))
	if mkDirErr != nil {
		return "", "", mkDirErr
	}

	log.Debug("Using ", dir)

	defaultBranch, cloneErr := b.GithubClient.ShallowClone(org, repoNameOnly, dir, b.MigrationConfig.BranchName)
	if cloneErr != nil {
		return "", "", fmt.Errorf("clone error: %w", cloneErr)
	}

	return dir, defaultBranch, nil
}

// cloneRepoWorktree handles the cached-repo + worktree path.
func (b *Banshee) cloneRepoWorktree(log *logrus.Entry, org, repoNameOnly string) (string, string, error) {
	cacheDir := b.getCacheRepoPath(org, repoNameOnly)
	worktreeDir := b.getWorktreePath(org, repoNameOnly)

	// Clean up any leftover worktree from an interrupted run.
	if _, err := os.Stat(worktreeDir); err == nil {
		log.Warn("Removing leftover worktree directory ", worktreeDir)
		_ = os.RemoveAll(worktreeDir)
		// Prune stale git worktree metadata if the cache repo exists.
		if _, statErr := os.Stat(cacheDir + "/.git"); statErr == nil {
			_ = b.GithubClient.GitWorktreePrune(cacheDir)
		}
	}

	// Ensure parent directories exist for both cache and worktree.
	if err := b.createCacheRepo(log, cacheDir); err != nil {
		return "", "", err
	}
	worktreeParent := filepath.Dir(worktreeDir)
	if err := b.createCacheRepo(log, worktreeParent); err != nil {
		return "", "", err
	}

	log.Info("Using cache ", cacheDir, ", worktree ", worktreeDir)

	defaultBranch, cloneErr := b.GithubClient.ShallowCloneWorktree(
		org, repoNameOnly, cacheDir, worktreeDir, b.MigrationConfig.BranchName,
	)
	if cloneErr != nil {
		return "", "", fmt.Errorf("clone error: %w", cloneErr)
	}

	return worktreeDir, defaultBranch, nil
}
