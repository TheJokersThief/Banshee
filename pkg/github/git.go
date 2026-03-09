package github

// Push pushes the current HEAD to the remote.
func (gc *GithubClient) Push(branch, dir, org, repoName string) error {
	tokenURL, err := gc.freshTokenURL(org, repoName)
	if err != nil {
		return err
	}
	return gc.git.Push(dir, tokenURL, branch)
}

// GitIsClean reports whether the working tree has no uncommitted changes.
func (gc *GithubClient) GitIsClean(dir string) (bool, error) {
	return gc.git.IsClean(dir)
}

// GitAddAll stages all changes in dir.
func (gc *GithubClient) GitAddAll(dir string) error {
	return gc.git.AddAll(dir)
}

// GitCommit creates a commit with the given message and author identity.
func (gc *GithubClient) GitCommit(dir, message, name, email string) error {
	return gc.git.Commit(dir, message, name, email)
}

// GitWorktreeRemove removes a git worktree from the given repo directory.
func (gc *GithubClient) GitWorktreeRemove(repoDir, worktreeDir string) error {
	return gc.git.WorktreeRemove(repoDir, worktreeDir)
}

// GitWorktreePrune removes stale worktree metadata from the repo.
func (gc *GithubClient) GitWorktreePrune(repoDir string) error {
	return gc.git.WorktreePrune(repoDir)
}
