package gitcli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

// GitError holds the raw stderr output when git exits non-zero and no sentinel applies.
type GitError struct {
	Stderr string
}

func (e *GitError) Error() string {
	return fmt.Sprintf("git error: %s", e.Stderr)
}

// ExecGit runs git commands via the system git binary.
type ExecGit struct {
	showOutput bool
	log        *logrus.Entry
	ctx        context.Context
}

func NewExecGit(ctx context.Context, showOutput bool, log *logrus.Entry) *ExecGit {
	return &ExecGit{ctx: ctx, showOutput: showOutput, log: log}
}

// run executes git with the given args in dir, returning trimmed stdout.
// When dir is empty, the process working directory is used (acceptable for
// Clone, where the destination path is passed as an argument).
func (g *ExecGit) run(dir string, args ...string) (string, error) {
	g.log.WithField("args", args).Debug("git")
	cmd := exec.CommandContext(g.ctx, "git", args...)
	cmd.Dir = dir

	var stdoutBuf, stderrBuf strings.Builder
	if g.showOutput {
		mw := io.MultiWriter(&stdoutBuf, os.Stdout)
		cmd.Stdout = mw
		mw2 := io.MultiWriter(&stderrBuf, os.Stderr)
		cmd.Stderr = mw2
	} else {
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf
	}

	err := cmd.Run()
	stdout := strings.TrimSpace(stdoutBuf.String())
	stderr := strings.TrimSpace(stderrBuf.String())

	if err != nil {
		return stdout, parseGitError(stderr)
	}
	return stdout, nil
}

func parseGitError(stderr string) error {
	lower := strings.ToLower(stderr)
	switch {
	case strings.Contains(lower, "branch") && strings.Contains(lower, "already exists"):
		return ErrBranchAlreadyExists
	case strings.Contains(lower, "already up to date"):
		return ErrAlreadyUpToDate
	case strings.Contains(lower, "reference not found"):
		return ErrReferenceNotFound
	case strings.Contains(lower, "couldn't find remote ref"):
		return ErrRemoteRefNotFound
	case strings.Contains(lower, "already checked out") || strings.Contains(lower, "already used by worktree"):
		return ErrWorktreeAlreadyExists
	default:
		msg := stderr
		if msg == "" {
			msg = "(no stderr output; git may have exited due to a signal or early EOF)"
		}
		return &GitError{Stderr: msg}
	}
}

// Clone performs a single-branch clone, optionally shallow.
func (g *ExecGit) Clone(tokenURL, dir, branch string, depth int) error {
	args := []string{"clone", "--single-branch", "--branch", branch}
	if depth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", depth))
	}
	args = append(args, tokenURL, dir)
	_, err := g.run("", args...)
	return err
}

// Checkout switches to branch. When create=true it tries -b first, then falls
// back to a plain checkout if the branch already exists.
func (g *ExecGit) Checkout(dir, branch string, create bool) error {
	if create {
		_, err := g.run(dir, "checkout", "-b", branch)
		if errors.Is(err, ErrBranchAlreadyExists) {
			_, fallbackErr := g.run(dir, "checkout", branch)
			if fallbackErr != nil {
				return fmt.Errorf("checkout %s (fallback after branch-exists): %w", branch, fallbackErr)
			}
			return nil
		}
		return err
	}
	_, err := g.run(dir, "checkout", branch)
	return err
}

// Fetch fetches branch from the given URL. Returns true if the branch was
// found on the remote, false if it does not exist. ErrAlreadyUpToDate,
// "refusing to fetch into current branch", and "shallow update not allowed"
// are swallowed (branch is considered found in those cases).
func (g *ExecGit) Fetch(dir, tokenURL, branch string) (bool, error) {
	refspec := fmt.Sprintf("%s:%s", branch, branch)
	_, err := g.run(dir, "fetch", tokenURL, refspec)
	if errors.Is(err, ErrRemoteRefNotFound) {
		return false, nil
	}
	if errors.Is(err, ErrAlreadyUpToDate) {
		return true, nil
	}
	// When we are already on the target branch git refuses to update it via
	// refspec. Pull handles the actual sync in that case.
	// Shallow repos may also emit "shallow update not allowed" — both are safe to swallow.
	var ge *GitError
	if errors.As(err, &ge) {
		if strings.Contains(ge.Stderr, "refusing to fetch") ||
			strings.Contains(ge.Stderr, "shallow update not allowed") {
			return true, nil
		}
	}
	return err == nil, err
}

// Pull hard-resets HEAD then fast-forward pulls from the given URL.
// ErrAlreadyUpToDate is swallowed; all other errors (including ErrReferenceNotFound
// when the remote branch does not exist) are surfaced to the caller.
func (g *ExecGit) Pull(dir, tokenURL, branch string) error {
	if _, err := g.run(dir, "reset", "--hard", "HEAD"); err != nil {
		return err
	}
	_, err := g.run(dir, "pull", "--ff-only", tokenURL, branch)
	if errors.Is(err, ErrAlreadyUpToDate) {
		return nil
	}
	return err
}

// Push pushes the current HEAD to branch on the remote using an unambiguous
// refs/heads/ refspec. ErrAlreadyUpToDate is swallowed.
func (g *ExecGit) Push(dir, tokenURL, branch string) error {
	_, err := g.run(dir, "push", "--force", tokenURL, "HEAD:refs/heads/"+branch)
	if errors.Is(err, ErrAlreadyUpToDate) {
		return nil
	}
	return err
}

// ResetToRef hard-resets the current branch to the given ref (branch name,
// tag, or commit SHA). All uncommitted changes are discarded.
func (g *ExecGit) ResetToRef(dir, ref string) error {
	_, err := g.run(dir, "reset", "--hard", ref)
	return err
}

// IsClean returns true when the working tree has no changes.
func (g *ExecGit) IsClean(dir string) (bool, error) {
	out, err := g.run(dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out == "", nil
}

// AddAll stages all changes.
func (g *ExecGit) AddAll(dir string) error {
	_, err := g.run(dir, "add", "--all", ".")
	return err
}

// Commit creates a commit with the given message and author/committer identity.
// -c user.name/email sets the committer; --author sets the author explicitly.
func (g *ExecGit) Commit(dir, message, authorName, authorEmail string) error {
	author := fmt.Sprintf("%s <%s>", authorName, authorEmail)
	_, err := g.run(dir,
		"-c", "user.name="+authorName,
		"-c", "user.email="+authorEmail,
		"commit", "--author", author, "-m", message,
	)
	return err
}

// WorktreeAdd creates a git worktree at worktreeDir for the given branch.
// When create=true, a new branch is created (-b). If the worktree already
// exists (stale from an interrupted run), it is removed and retried.
func (g *ExecGit) WorktreeAdd(repoDir, worktreeDir, branch string, create bool) error {
	args := []string{"-C", repoDir, "worktree", "add"}
	if create {
		args = append(args, "-b", branch, worktreeDir)
	} else {
		args = append(args, worktreeDir, branch)
	}

	_, err := g.run("", args...)
	if isWorktreeExistsErr(err) {
		// Stale worktree from a previous interrupted run — force-remove,
		// prune metadata, delete the physical directory, then retry.
		// WorktreeRemove may fail if git no longer tracks this worktree; that's fine.
		_, _ = g.run("", "-C", repoDir, "worktree", "remove", "--force", worktreeDir)
		if _, pruneErr := g.run("", "-C", repoDir, "worktree", "prune"); pruneErr != nil {
			return fmt.Errorf("pruning stale worktrees before retry: %w", pruneErr)
		}
		// Remove the physical directory if it still exists (covers the case
		// where only the directory remains but git's worktree metadata is gone).
		_ = os.RemoveAll(worktreeDir)
		_, err = g.run("", args...)
	}
	return err
}

// isWorktreeExistsErr returns true when git reports the worktree (or its
// target path) already exists — either because the worktree is still tracked
// by git or because the directory remains from an interrupted run.
func isWorktreeExistsErr(err error) bool {
	if errors.Is(err, ErrWorktreeAlreadyExists) {
		return true
	}
	var ge *GitError
	return errors.As(err, &ge) && strings.Contains(ge.Stderr, "already exists")
}

// WorktreeRemove forcefully removes a git worktree.
func (g *ExecGit) WorktreeRemove(repoDir, worktreeDir string) error {
	_, err := g.run("", "-C", repoDir, "worktree", "remove", "--force", worktreeDir)
	return err
}

// WorktreePrune removes stale worktree metadata from the repo.
func (g *ExecGit) WorktreePrune(repoDir string) error {
	_, err := g.run("", "-C", repoDir, "worktree", "prune")
	return err
}
