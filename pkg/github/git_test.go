package github

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thejokersthief/banshee/v2/pkg/gitcli"
	"golang.org/x/time/rate"
)

// ── fakeGit ───────────────────────────────────────────────────────────────────

type fakeGit struct {
	cloneArgs         *cloneCall
	checkoutArgs      *checkoutCall
	fetchArgs         *fetchCall
	pullArgs          *pullCall
	pushArgs          *pushCall
	isCleanArgs       *isCleanCall
	addAllArgs        *addAllCall
	commitArgs        *commitCall
	worktreeAddArgs   *worktreeAddCall
	worktreeRemoveArgs *worktreeRemoveCall

	isCleanResult bool
	err           error
}

type cloneCall   struct{ tokenURL, dir, branch string; depth int }
type checkoutCall struct{ dir, branch string; create bool }
type fetchCall   struct{ dir, tokenURL, branch string }
type pullCall    struct{ dir, tokenURL, branch string }
type pushCall    struct{ dir, tokenURL, branch string }
type isCleanCall struct{ dir string }
type addAllCall  struct{ dir string }
type commitCall        struct{ dir, message, name, email string }
type worktreeAddCall   struct{ repoDir, worktreeDir, branch string; create bool }
type worktreeRemoveCall struct{ repoDir, worktreeDir string }

func (f *fakeGit) Clone(tokenURL, dir, branch string, depth int) error {
	f.cloneArgs = &cloneCall{tokenURL, dir, branch, depth}
	return f.err
}
func (f *fakeGit) Checkout(dir, branch string, create bool) error {
	f.checkoutArgs = &checkoutCall{dir, branch, create}
	return f.err
}
func (f *fakeGit) Fetch(dir, tokenURL, branch string) error {
	f.fetchArgs = &fetchCall{dir, tokenURL, branch}
	return f.err
}
func (f *fakeGit) Pull(dir, tokenURL, branch string) error {
	f.pullArgs = &pullCall{dir, tokenURL, branch}
	return f.err
}
func (f *fakeGit) Push(dir, tokenURL, branch string) error {
	f.pushArgs = &pushCall{dir, tokenURL, branch}
	return f.err
}
func (f *fakeGit) IsClean(dir string) (bool, error) {
	f.isCleanArgs = &isCleanCall{dir}
	return f.isCleanResult, f.err
}
func (f *fakeGit) AddAll(dir string) error {
	f.addAllArgs = &addAllCall{dir}
	return f.err
}
func (f *fakeGit) Commit(dir, message, name, email string) error {
	f.commitArgs = &commitCall{dir, message, name, email}
	return f.err
}
func (f *fakeGit) WorktreeAdd(repoDir, worktreeDir, branch string, create bool) error {
	f.worktreeAddArgs = &worktreeAddCall{repoDir, worktreeDir, branch, create}
	return f.err
}
func (f *fakeGit) WorktreeRemove(repoDir, worktreeDir string) error {
	f.worktreeRemoveArgs = &worktreeRemoveCall{repoDir, worktreeDir}
	return f.err
}
func (f *fakeGit) WorktreePrune(_ string) error {
	return f.err
}

// Verify fakeGit satisfies the interface at compile time.
var _ gitcli.Git = (*fakeGit)(nil)

// ── helpers ───────────────────────────────────────────────────────────────────

func newTestClient(fake gitcli.Git, token string) *GithubClient {
	return &GithubClient{
		git:         fake,
		accessToken: token,
		log:         logrus.NewEntry(logrus.New()),
		ctx:         context.Background(),
		rateLimiter: rate.NewLimiter(rate.Inf, 0),
	}
}

// ── Push ─────────────────────────────────────────────────────────────────────

func TestPushForwardsToGit(t *testing.T) {
	fake := &fakeGit{}
	gc := newTestClient(fake, "mytoken")

	err := gc.Push("branch", "/repo/dir", "myorg", "myrepo")
	require.NoError(t, err)

	require.NotNil(t, fake.pushArgs)
	assert.Equal(t, "/repo/dir", fake.pushArgs.dir)
	assert.Contains(t, fake.pushArgs.tokenURL, "mytoken")
	assert.Contains(t, fake.pushArgs.tokenURL, "myorg/myrepo")
	assert.Equal(t, "branch", fake.pushArgs.branch)
}

func TestPushPropagatesError(t *testing.T) {
	fake := &fakeGit{err: &gitcli.GitError{Stderr: "push failed"}}
	gc := newTestClient(fake, "tok")

	err := gc.Push("branch", "/dir", "org", "repo")
	assert.Error(t, err)
}

// ── GitIsClean ────────────────────────────────────────────────────────────────

func TestGitIsCleanDelegates(t *testing.T) {
	fake := &fakeGit{isCleanResult: true}
	gc := newTestClient(fake, "tok")

	clean, err := gc.GitIsClean("/some/dir")
	require.NoError(t, err)
	assert.True(t, clean)
	require.NotNil(t, fake.isCleanArgs)
	assert.Equal(t, "/some/dir", fake.isCleanArgs.dir)
}

func TestGitIsCleanReturnsDirty(t *testing.T) {
	fake := &fakeGit{isCleanResult: false}
	gc := newTestClient(fake, "tok")

	clean, err := gc.GitIsClean("/dir")
	require.NoError(t, err)
	assert.False(t, clean)
}

// ── GitAddAll ─────────────────────────────────────────────────────────────────

func TestGitAddAllDelegates(t *testing.T) {
	fake := &fakeGit{}
	gc := newTestClient(fake, "tok")

	require.NoError(t, gc.GitAddAll("/my/dir"))
	require.NotNil(t, fake.addAllArgs)
	assert.Equal(t, "/my/dir", fake.addAllArgs.dir)
}

func TestGitAddAllPropagatesError(t *testing.T) {
	fake := &fakeGit{err: &gitcli.GitError{Stderr: "add failed"}}
	gc := newTestClient(fake, "tok")
	assert.Error(t, gc.GitAddAll("/dir"))
}

// ── GitCommit ─────────────────────────────────────────────────────────────────

func TestGitCommitDelegates(t *testing.T) {
	fake := &fakeGit{}
	gc := newTestClient(fake, "tok")

	require.NoError(t, gc.GitCommit("/repo", "fix: something", "Alice", "alice@example.com"))
	require.NotNil(t, fake.commitArgs)
	assert.Equal(t, "/repo", fake.commitArgs.dir)
	assert.Equal(t, "fix: something", fake.commitArgs.message)
	assert.Equal(t, "Alice", fake.commitArgs.name)
	assert.Equal(t, "alice@example.com", fake.commitArgs.email)
}

func TestGitCommitPropagatesError(t *testing.T) {
	fake := &fakeGit{err: &gitcli.GitError{Stderr: "commit failed"}}
	gc := newTestClient(fake, "tok")
	assert.Error(t, gc.GitCommit("/dir", "msg", "Name", "email"))
}

// ── freshTokenURL ─────────────────────────────────────────────────────────────

func TestFreshTokenURLEmbeddsToken(t *testing.T) {
	gc := newTestClient(&fakeGit{}, "secrettoken")
	url, err := gc.freshTokenURL("acme", "widget")
	require.NoError(t, err)
	assert.Equal(t, "https://x-access-token:secrettoken@github.com/acme/widget.git", url)
}

// TestFreshTokenURLRefreshesAppToken verifies that when tokenRefreshItr is nil
// (plain token auth), freshTokenURL always returns the static access token.
func TestFreshTokenURLStaticToken(t *testing.T) {
	gc := newTestClient(&fakeGit{}, "statictoken")
	url1, err := gc.freshTokenURL("org", "repo")
	require.NoError(t, err)
	url2, err := gc.freshTokenURL("org", "repo")
	require.NoError(t, err)
	assert.Equal(t, url1, url2, "static token URL should be stable")
	assert.Contains(t, url1, "statictoken")
}

// TestGitIsCleanPropagatesError verifies that an error from the underlying git
// implementation is surfaced rather than silently swallowed.
func TestGitIsCleanPropagatesError(t *testing.T) {
	fake := &fakeGit{err: &gitcli.GitError{Stderr: "not a git repo"}}
	gc := newTestClient(fake, "tok")
	_, err := gc.GitIsClean("/not/a/repo")
	assert.Error(t, err)
}
