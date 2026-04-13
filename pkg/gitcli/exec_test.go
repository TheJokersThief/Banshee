package gitcli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func newGit(t *testing.T) *ExecGit {
	t.Helper()
	return NewExecGit(context.Background(), false, logrus.NewEntry(logrus.New()))
}

// runCmd runs an arbitrary command and fails the test on error.
func runCmd(t *testing.T, name string, args ...string) string {
	t.Helper()
	out, err := exec.Command(name, args...).CombinedOutput()
	require.NoError(t, err, "command %s %v: %s", name, args, out)
	return strings.TrimSpace(string(out))
}

// headBranch returns the current branch name of the repo at dir.
func headBranch(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "symbolic-ref", "--short", "HEAD").CombinedOutput()
	require.NoError(t, err)
	return strings.TrimSpace(string(out))
}

// initRepo creates a git repo at dir with a single empty initial commit.
// Returns the branch name created by git init.
func initRepo(t *testing.T, dir string) string {
	t.Helper()
	runCmd(t, "git", "init", dir)
	runCmd(t, "git", "-C", dir, "config", "user.email", "test@example.com")
	runCmd(t, "git", "-C", dir, "config", "user.name", "Test User")
	runCmd(t, "git", "-C", dir, "commit", "--allow-empty", "-m", "initial commit")
	return headBranch(t, dir)
}

// initBareWithContent creates a bare repo and populates it with an initial
// commit via a temporary clone. Returns the bare dir path and the branch name.
func initBareWithContent(t *testing.T) (bareDir, branch string) {
	t.Helper()
	bareDir = t.TempDir()
	runCmd(t, "git", "init", "--bare", bareDir)

	src := t.TempDir()
	runCmd(t, "git", "clone", bareDir, src)
	runCmd(t, "git", "-C", src, "config", "user.email", "test@example.com")
	runCmd(t, "git", "-C", src, "config", "user.name", "Test User")
	runCmd(t, "git", "-C", src, "commit", "--allow-empty", "-m", "initial commit")
	runCmd(t, "git", "-C", src, "push", "origin", "HEAD")

	branch = headBranch(t, src)
	return
}

// ── Clone ────────────────────────────────────────────────────────────────────

func TestClone(t *testing.T) {
	src := t.TempDir()
	branch := initRepo(t, src)

	dst := filepath.Join(t.TempDir(), "clone")
	// For a regular (non-shallow) clone a local path works fine without file://.
	require.NoError(t, newGit(t).Clone(src, dst, branch, 0))
	assert.DirExists(t, filepath.Join(dst, ".git"))
}

func TestCloneShallow(t *testing.T) {
	src := t.TempDir()
	branch := initRepo(t, src)
	// Add a second commit so we can verify shallow truncation.
	runCmd(t, "git", "-C", src, "commit", "--allow-empty", "-m", "second commit")

	dst := filepath.Join(t.TempDir(), "shallow")
	// Use file:// so git uses the network transport, which honours --depth.
	require.NoError(t, newGit(t).Clone("file://"+src, dst, branch, 1))

	out, err := exec.Command("git", "-C", dst, "log", "--oneline").Output()
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	assert.Len(t, lines, 1, "shallow clone should have exactly 1 commit")
}

// ── Checkout ─────────────────────────────────────────────────────────────────

func TestCheckoutCreate(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)

	require.NoError(t, newGit(t).Checkout(dir, "new-branch", true))
	assert.Equal(t, "new-branch", headBranch(t, dir))
}

func TestCheckoutCreateAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	branch := initRepo(t, dir)

	// Create the branch first.
	runCmd(t, "git", "-C", dir, "branch", "existing-branch")

	// Checkout back to default, then try to create "existing-branch" again.
	runCmd(t, "git", "-C", dir, "checkout", branch)
	err := newGit(t).Checkout(dir, "existing-branch", true)
	require.NoError(t, err, "create=true on existing branch should not error")
	assert.Equal(t, "existing-branch", headBranch(t, dir))
}

func TestCheckoutNoCreate(t *testing.T) {
	dir := t.TempDir()
	branch := initRepo(t, dir)

	// Pre-create a branch to switch to.
	runCmd(t, "git", "-C", dir, "branch", "other-branch")

	require.NoError(t, newGit(t).Checkout(dir, "other-branch", false))
	assert.Equal(t, "other-branch", headBranch(t, dir))

	// Switch back to the original branch.
	require.NoError(t, newGit(t).Checkout(dir, branch, false))
	assert.Equal(t, branch, headBranch(t, dir))
}

// ── Fetch ────────────────────────────────────────────────────────────────────

func TestFetch(t *testing.T) {
	bareDir, branch := initBareWithContent(t)

	// Push a feature branch to the bare repo.
	src := t.TempDir()
	runCmd(t, "git", "clone", bareDir, src)
	runCmd(t, "git", "-C", src, "config", "user.email", "test@example.com")
	runCmd(t, "git", "-C", src, "config", "user.name", "Test User")
	runCmd(t, "git", "-C", src, "checkout", "-b", "feature-branch")
	runCmd(t, "git", "-C", src, "commit", "--allow-empty", "-m", "feature commit")
	runCmd(t, "git", "-C", src, "push", "origin", "feature-branch")

	// Clone the bare repo into a fresh local (only the default branch).
	local := filepath.Join(t.TempDir(), "local")
	require.NoError(t, newGit(t).Clone(bareDir, local, branch, 0))

	// Fetch the feature branch into the local repo.
	found, err := newGit(t).Fetch(local, bareDir, "feature-branch")
	require.NoError(t, err)
	assert.True(t, found, "branch exists on remote, should return true")

	// Verify the local branch was created.
	out, err := exec.Command("git", "-C", local, "branch").Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "feature-branch")

	// Verify the feature commit is reachable in local.
	out2, err2 := exec.Command("git", "-C", local, "log", "--oneline", "feature-branch").Output()
	require.NoError(t, err2)
	assert.Contains(t, string(out2), "feature commit")
}

func TestFetchNonexistentBranch(t *testing.T) {
	bareDir, branch := initBareWithContent(t)
	local := filepath.Join(t.TempDir(), "local")
	require.NoError(t, newGit(t).Clone(bareDir, local, branch, 0))

	// Fetching a branch that doesn't exist should return false (not found).
	found, err := newGit(t).Fetch(local, bareDir, "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found, "branch does not exist on remote, should return false")
}

func TestFetchOnCurrentBranch(t *testing.T) {
	// Fetching the branch we're currently on triggers "refusing to fetch into
	// current branch" — Fetch should swallow this and return nil.
	bareDir, branch := initBareWithContent(t)
	local := filepath.Join(t.TempDir(), "local")
	require.NoError(t, newGit(t).Clone(bareDir, local, branch, 0))

	// We are on `branch`; fetching branch:branch should be swallowed (branch exists).
	found, err := newGit(t).Fetch(local, bareDir, branch)
	require.NoError(t, err)
	assert.True(t, found, "current branch exists on remote, should return true")
}

// ── Pull ─────────────────────────────────────────────────────────────────────

func TestPull(t *testing.T) {
	bareDir, branch := initBareWithContent(t)

	// Clone to local.
	local := filepath.Join(t.TempDir(), "local")
	require.NoError(t, newGit(t).Clone(bareDir, local, branch, 0))

	// Push a new commit from a second clone.
	src2 := t.TempDir()
	runCmd(t, "git", "clone", bareDir, src2)
	runCmd(t, "git", "-C", src2, "config", "user.email", "test@example.com")
	runCmd(t, "git", "-C", src2, "config", "user.name", "Test User")
	require.NoError(t, os.WriteFile(filepath.Join(src2, "newfile.txt"), []byte("hello"), 0644))
	runCmd(t, "git", "-C", src2, "add", ".")
	runCmd(t, "git", "-C", src2, "commit", "-m", "add newfile")
	runCmd(t, "git", "-C", src2, "push")

	// Pull into local.
	require.NoError(t, newGit(t).Pull(local, bareDir, branch))
	assert.FileExists(t, filepath.Join(local, "newfile.txt"))
}

func TestPullRemoteBranchGone(t *testing.T) {
	bareDir, branch := initBareWithContent(t)
	local := filepath.Join(t.TempDir(), "local")
	require.NoError(t, newGit(t).Clone(bareDir, local, branch, 0))

	// Pull a branch that doesn't exist on the remote — should return an error.
	err := newGit(t).Pull(local, bareDir, "nonexistent-branch")
	assert.Error(t, err, "Pull should surface an error when the remote branch does not exist")
}

// ── Push ─────────────────────────────────────────────────────────────────────

func TestPush(t *testing.T) {
	bareDir, branch := initBareWithContent(t)

	local := filepath.Join(t.TempDir(), "local")
	require.NoError(t, newGit(t).Clone(bareDir, local, branch, 0))
	runCmd(t, "git", "-C", local, "config", "user.email", "test@example.com")
	runCmd(t, "git", "-C", local, "config", "user.name", "Test User")

	require.NoError(t, os.WriteFile(filepath.Join(local, "pushed.txt"), []byte("pushed"), 0644))
	require.NoError(t, newGit(t).AddAll(local))
	require.NoError(t, newGit(t).Commit(local, "push test", "Test User", "test@example.com"))

	require.NoError(t, newGit(t).Push(local, bareDir, branch))

	// Verify: clone bare into verify dir and check file exists.
	verify := filepath.Join(t.TempDir(), "verify")
	runCmd(t, "git", "clone", bareDir, verify)
	assert.FileExists(t, filepath.Join(verify, "pushed.txt"))
}

func TestPushForceOverwritesDivergedHistory(t *testing.T) {
	bareDir, branch := initBareWithContent(t)

	// Clone to local and make a commit.
	local := filepath.Join(t.TempDir(), "local")
	require.NoError(t, newGit(t).Clone(bareDir, local, branch, 0))
	runCmd(t, "git", "-C", local, "config", "user.email", "test@example.com")
	runCmd(t, "git", "-C", local, "config", "user.name", "Test User")

	require.NoError(t, os.WriteFile(filepath.Join(local, "a.txt"), []byte("a"), 0644))
	require.NoError(t, newGit(t).AddAll(local))
	require.NoError(t, newGit(t).Commit(local, "local commit", "Test User", "test@example.com"))
	require.NoError(t, newGit(t).Push(local, bareDir, branch))

	// Reset local to before the commit, add a different commit — history diverges.
	require.NoError(t, newGit(t).ResetToRef(local, "HEAD~1"))
	require.NoError(t, os.WriteFile(filepath.Join(local, "b.txt"), []byte("b"), 0644))
	require.NoError(t, newGit(t).AddAll(local))
	require.NoError(t, newGit(t).Commit(local, "diverged commit", "Test User", "test@example.com"))

	// Push should succeed (force) even though history diverged.
	require.NoError(t, newGit(t).Push(local, bareDir, branch))

	// Verify the bare repo has b.txt (from the force-push) and not a.txt.
	verify := filepath.Join(t.TempDir(), "verify")
	runCmd(t, "git", "clone", bareDir, verify)
	assert.FileExists(t, filepath.Join(verify, "b.txt"))
	assert.NoFileExists(t, filepath.Join(verify, "a.txt"))
}

// ── ResetToRef ───────────────────────────────────────────────────────────────

func TestResetToRef(t *testing.T) {
	dir := t.TempDir()
	branch := initRepo(t, dir)
	g := newGit(t)

	// Create a file and commit it.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("v1"), 0644))
	require.NoError(t, g.AddAll(dir))
	require.NoError(t, g.Commit(dir, "add file", "Test", "test@example.com"))

	// Record the current HEAD.
	headAfterCommit := runCmd(t, "git", "-C", dir, "rev-parse", "HEAD")

	// Add another commit so HEAD moves forward.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("v2"), 0644))
	require.NoError(t, g.AddAll(dir))
	require.NoError(t, g.Commit(dir, "update file", "Test", "test@example.com"))

	// Reset back to the first commit.
	require.NoError(t, g.ResetToRef(dir, headAfterCommit))

	// HEAD should now match the earlier commit.
	headNow := runCmd(t, "git", "-C", dir, "rev-parse", "HEAD")
	assert.Equal(t, headAfterCommit, headNow)

	// File content should match the first commit.
	content, err := os.ReadFile(filepath.Join(dir, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "v1", string(content))

	_ = branch // silence unused
}

func TestResetToRefBranchName(t *testing.T) {
	dir := t.TempDir()
	branch := initRepo(t, dir)
	g := newGit(t)

	// Add a commit on a feature branch.
	require.NoError(t, g.Checkout(dir, "feature", true))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "feat.txt"), []byte("feature"), 0644))
	require.NoError(t, g.AddAll(dir))
	require.NoError(t, g.Commit(dir, "feature commit", "Test", "test@example.com"))

	// Reset the feature branch back to the default branch.
	require.NoError(t, g.ResetToRef(dir, branch))

	// feat.txt should no longer exist.
	assert.NoFileExists(t, filepath.Join(dir, "feat.txt"))
}

// ── IsClean ───────────────────────────────────────────────────────────────────

func TestIsClean(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	g := newGit(t)

	// Fresh repo is clean.
	clean, err := g.IsClean(dir)
	require.NoError(t, err)
	assert.True(t, clean, "expected clean repo")

	// Untracked file makes it dirty.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("x"), 0644))
	clean, err = g.IsClean(dir)
	require.NoError(t, err)
	assert.False(t, clean, "expected dirty after untracked file")

	// Staged file is still dirty.
	require.NoError(t, g.AddAll(dir))
	clean, err = g.IsClean(dir)
	require.NoError(t, err)
	assert.False(t, clean, "expected dirty after staging")

	// After committing, clean again.
	require.NoError(t, g.Commit(dir, "add untracked", "Test", "test@example.com"))
	clean, err = g.IsClean(dir)
	require.NoError(t, err)
	assert.True(t, clean, "expected clean after commit")

	// Modifying a tracked file makes it dirty.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("modified"), 0644))
	clean, err = g.IsClean(dir)
	require.NoError(t, err)
	assert.False(t, clean, "expected dirty after modifying tracked file")
}

// ── AddAll ────────────────────────────────────────────────────────────────────

func TestAddAll(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	g := newGit(t)

	// Create several files.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644))

	require.NoError(t, g.AddAll(dir))

	// git status --porcelain should show both files as staged (A).
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "a.txt")
	assert.Contains(t, string(out), "b.txt")
	// All lines should start with a staged marker (first column non-space).
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		assert.NotEqual(t, ' ', rune(line[0]), "expected staged, got unstaged line: %s", line)
	}
}

// ── Commit ────────────────────────────────────────────────────────────────────

func TestCommit(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	g := newGit(t)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644))
	require.NoError(t, g.AddAll(dir))
	require.NoError(t, g.Commit(dir, "my commit message", "Alice", "alice@example.com"))

	// Verify message.
	msg := runCmd(t, "git", "-C", dir, "log", "-1", "--format=%s")
	assert.Equal(t, "my commit message", msg)

	// Verify author name and email.
	authorName := runCmd(t, "git", "-C", dir, "log", "-1", "--format=%an")
	assert.Equal(t, "Alice", authorName)
	authorEmail := runCmd(t, "git", "-C", dir, "log", "-1", "--format=%ae")
	assert.Equal(t, "alice@example.com", authorEmail)
}

// ── WorktreeAdd / WorktreeRemove ──────────────────────────────────────────────

func TestWorktreeAddAndRemove(t *testing.T) {
	bareDir, branch := initBareWithContent(t)

	// Clone the bare repo to use as the main repo dir.
	repoDir := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, newGit(t).Clone(bareDir, repoDir, branch, 0))

	g := newGit(t)

	// Add a worktree with a new branch.
	wtDir := filepath.Join(t.TempDir(), "worktree")
	require.NoError(t, g.WorktreeAdd(repoDir, wtDir, "wt-branch", true))

	// Verify the worktree exists and is on the expected branch.
	assert.DirExists(t, wtDir)
	assert.Equal(t, "wt-branch", headBranch(t, wtDir))

	// Remove the worktree.
	require.NoError(t, g.WorktreeRemove(repoDir, wtDir))
	assert.NoDirExists(t, wtDir)
}

func TestWorktreeAddExistingBranch(t *testing.T) {
	bareDir, branch := initBareWithContent(t)

	repoDir := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, newGit(t).Clone(bareDir, repoDir, branch, 0))

	g := newGit(t)

	// Create the branch first, then add worktree without -b.
	runCmd(t, "git", "-C", repoDir, "branch", "existing-wt-branch")

	wtDir := filepath.Join(t.TempDir(), "worktree")
	require.NoError(t, g.WorktreeAdd(repoDir, wtDir, "existing-wt-branch", false))

	assert.DirExists(t, wtDir)
	assert.Equal(t, "existing-wt-branch", headBranch(t, wtDir))

	require.NoError(t, g.WorktreeRemove(repoDir, wtDir))
}

func TestWorktreeAddStaleRecovery(t *testing.T) {
	bareDir, branch := initBareWithContent(t)

	repoDir := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, newGit(t).Clone(bareDir, repoDir, branch, 0))

	g := newGit(t)

	// Create an initial worktree, then delete the directory (simulating interrupted run).
	wtDir := filepath.Join(t.TempDir(), "worktree")
	require.NoError(t, g.WorktreeAdd(repoDir, wtDir, "stale-branch", true))
	require.NoError(t, os.RemoveAll(wtDir))

	// Adding a new worktree for the same branch should recover via prune+retry.
	wtDir2 := filepath.Join(t.TempDir(), "worktree2")
	require.NoError(t, g.WorktreeAdd(repoDir, wtDir2, "stale-branch", false))

	assert.DirExists(t, wtDir2)
	assert.Equal(t, "stale-branch", headBranch(t, wtDir2))

	// Cleanup.
	require.NoError(t, g.WorktreeRemove(repoDir, wtDir2))
}

func TestWorktreeAddStaleDirectoryRecovery(t *testing.T) {
	bareDir, branch := initBareWithContent(t)

	repoDir := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, newGit(t).Clone(bareDir, repoDir, branch, 0))

	g := newGit(t)

	// Create a worktree, then prune its metadata but leave the directory on disk.
	// This simulates the state left behind by a previous interrupted run where
	// git no longer tracks the worktree but the physical directory remains.
	wtDir := filepath.Join(t.TempDir(), "worktree")
	require.NoError(t, g.WorktreeAdd(repoDir, wtDir, "leftover-branch", true))
	require.NoError(t, g.WorktreeRemove(repoDir, wtDir))
	// Re-create the directory to simulate the leftover.
	require.NoError(t, os.MkdirAll(wtDir, 0755))

	// WorktreeAdd should recover by removing the stale directory and retrying.
	require.NoError(t, g.WorktreeAdd(repoDir, wtDir, "leftover-branch", false))

	assert.DirExists(t, wtDir)
	assert.Equal(t, "leftover-branch", headBranch(t, wtDir))

	// Cleanup.
	require.NoError(t, g.WorktreeRemove(repoDir, wtDir))
}

// ── parseGitError ─────────────────────────────────────────────────────────────

func TestParseGitError(t *testing.T) {
	tests := []struct {
		name     string
		stderr   string
		wantErr  error
		wantType string // "sentinel" or "GitError"
	}{
		{
			name:    "branch already exists",
			stderr:  "fatal: A branch named 'foo' already exists.",
			wantErr: ErrBranchAlreadyExists,
		},
		{
			name:    "already up to date",
			stderr:  "Already up to date.",
			wantErr: ErrAlreadyUpToDate,
		},
		{
			name:    "reference not found",
			stderr:  "error: reference not found",
			wantErr: ErrReferenceNotFound,
		},
		{
			name:    "couldn't find remote ref",
			stderr:  "fatal: couldn't find remote ref feature-branch",
			wantErr: ErrRemoteRefNotFound,
		},
		{
			name:    "branch already exists (mixed case)",
			stderr:  "fatal: A Branch Named 'foo' Already Exists.",
			wantErr: ErrBranchAlreadyExists,
		},
		{
			name:    "already up to date (mixed case)",
			stderr:  "Already Up To Date.",
			wantErr: ErrAlreadyUpToDate,
		},
		{
			name:    "reference not found (mixed case)",
			stderr:  "Error: Reference Not Found",
			wantErr: ErrReferenceNotFound,
		},
		{
			name:    "couldn't find remote ref (mixed case)",
			stderr:  "Fatal: Couldn't Find Remote Ref feature",
			wantErr: ErrRemoteRefNotFound,
		},
		{
			name:    "worktree already checked out",
			stderr:  "fatal: 'stale-branch' is already checked out at '/tmp/worktree'",
			wantErr: ErrWorktreeAlreadyExists,
		},
		{
			name:     "unknown error returns GitError",
			stderr:   "fatal: some unexpected git error",
			wantType: "GitError",
		},
		{
			name:     "empty stderr returns GitError",
			stderr:   "",
			wantType: "GitError",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := parseGitError(tc.stderr)
			require.Error(t, err)

			if tc.wantErr != nil {
				assert.Equal(t, tc.wantErr, err)
			} else {
				var ge *GitError
				assert.ErrorAs(t, err, &ge, "expected *GitError")
			}
		})
	}
}
