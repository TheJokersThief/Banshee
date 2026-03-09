package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	gogithub "github.com/google/go-github/v63/github"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thejokersthief/banshee/v2/pkg/configs"
	"github.com/thejokersthief/banshee/v2/pkg/gitcli"
)

func TestMigrationOptions(t *testing.T) {
	var want_org, got_org string
	var want_repos, got_repos []string
	var optionsErr error

	mainConf := configs.MigrationConfig{
		SearchQuery:   "",
		ListOfRepos:   []string{"repo_name"},
		AllReposInOrg: false,
	}
	globalConf := configs.GlobalConfig{
		Options:  configs.OptionsConfig{LogLevel: "info"},
		Github:   configs.GithubConfig{Token: "testtoken"},
		Defaults: configs.DefaultsConfig{Organisation: "testorg"},
	}
	b, err := NewBanshee(context.Background(), globalConf, mainConf)
	assert.NoError(t, err)

	want_org = "testorg"
	want_repos = []string{"repo_name"}
	got_org, got_repos, optionsErr = b.migrationOptions()
	assert.NoError(t, optionsErr)
	assert.Equal(t, want_org, got_org)
	assert.Equal(t, want_repos, got_repos)
}

// setupBareRepo creates a bare git repo with a single initial commit and
// returns its path and the default branch name.
func setupBareRepo(t *testing.T) (bareDir, branch string) {
	t.Helper()
	bareDir = t.TempDir()
	run := func(name string, args ...string) {
		t.Helper()
		out, err := exec.Command(name, args...).CombinedOutput()
		require.NoError(t, err, "%s %v: %s", name, args, out)
	}
	run("git", "init", "--bare", bareDir)
	src := t.TempDir()
	run("git", "clone", bareDir, src)
	run("git", "-C", src, "config", "user.email", "test@example.com")
	run("git", "-C", src, "config", "user.name", "Test User")
	run("git", "-C", src, "commit", "--allow-empty", "-m", "initial commit")
	run("git", "-C", src, "push", "origin", "HEAD")
	out, err := exec.Command("git", "-C", src, "symbolic-ref", "--short", "HEAD").Output()
	require.NoError(t, err)
	branch = strings.TrimSpace(string(out))
	return
}

// ── fakeGithubClient ─────────────────────────────────────────────────────────

// fakeGithubClient implements githubClient for testing. Git operations
// are delegated to a real ExecGit pointed at a local bare repo; GitHub API
// calls return canned responses.
type fakeGithubClient struct {
	git           *gitcli.ExecGit
	bareRepoURL   string
	defaultBranch string
	createdPRURL  string
	pushCallCount atomic.Int32
}

func (f *fakeGithubClient) ShallowClone(org, repoName, dir, migrationBranchName string) (string, error) {
	if _, err := os.Stat(dir + "/.git"); os.IsNotExist(err) {
		if err := f.git.Clone(f.bareRepoURL, dir, f.defaultBranch, 0); err != nil {
			return "", err
		}
	} else {
		if err := f.git.Checkout(dir, f.defaultBranch, false); err != nil {
			return "", err
		}
	}
	return f.defaultBranch, f.git.Checkout(dir, migrationBranchName, true)
}

func (f *fakeGithubClient) ShallowCloneWorktree(org, repoName, cacheDir, worktreeDir, migrationBranchName string) (string, error) {
	if _, err := os.Stat(cacheDir + "/.git"); os.IsNotExist(err) {
		if err := f.git.Clone(f.bareRepoURL, cacheDir, f.defaultBranch, 0); err != nil {
			return "", err
		}
	} else {
		if err := f.git.Checkout(cacheDir, f.defaultBranch, false); err != nil {
			return "", err
		}
	}
	if err := f.git.WorktreeAdd(cacheDir, worktreeDir, migrationBranchName, true); err != nil {
		return "", err
	}
	return f.defaultBranch, nil
}
func (f *fakeGithubClient) GitWorktreeRemove(repoDir, worktreeDir string) error {
	return f.git.WorktreeRemove(repoDir, worktreeDir)
}
func (f *fakeGithubClient) GitWorktreePrune(repoDir string) error {
	return f.git.WorktreePrune(repoDir)
}
func (f *fakeGithubClient) GetDefaultBranch(_, _ string) (string, error) {
	return f.defaultBranch, nil
}
func (f *fakeGithubClient) GitIsClean(dir string) (bool, error)  { return f.git.IsClean(dir) }
func (f *fakeGithubClient) GitAddAll(dir string) error            { return f.git.AddAll(dir) }
func (f *fakeGithubClient) GitCommit(dir, msg, name, email string) error {
	return f.git.Commit(dir, msg, name, email)
}
func (f *fakeGithubClient) Push(branch, dir, _, _ string) error {
	f.pushCallCount.Add(1)
	return f.git.Push(dir, f.bareRepoURL, branch)
}
func (f *fakeGithubClient) FindPullRequest(_, _, _, _ string) (*gogithub.PullRequest, error) {
	return nil, nil
}
func (f *fakeGithubClient) CreatePullRequest(_, _, _, _, _, _ string, _ bool) (string, error) {
	return f.createdPRURL, nil
}
func (f *fakeGithubClient) UpdatePullRequest(_ *gogithub.PullRequest, _ string) error { return nil }
func (f *fakeGithubClient) MergePullRequest(_ *gogithub.PullRequest) error             { return nil }
func (f *fakeGithubClient) GetAllRepos(_ string) ([]string, error)                     { return nil, nil }
func (f *fakeGithubClient) GetMatchingRepos(_ string) ([]string, error)                { return nil, nil }
func (f *fakeGithubClient) GetMatchingPRs(_ string) ([]*gogithub.PullRequest, error)   { return nil, nil }

// Verify fakeGithubClient satisfies the interface at compile time.
var _ githubClient = (*fakeGithubClient)(nil)

// ── TestHandleRepo ────────────────────────────────────────────────────────────

func TestHandleRepo(t *testing.T) {
	// ── set up local git remote ──────────────────────────────────────────────

	bareDir, branch := setupBareRepo(t)
	run := func(name string, args ...string) {
		t.Helper()
		out, err := exec.Command(name, args...).CombinedOutput()
		require.NoError(t, err, "%s %v: %s", name, args, out)
	}

	// ── PR body file ─────────────────────────────────────────────────────────

	prBodyFile := filepath.Join(t.TempDir(), "pr-body.md")
	require.NoError(t, os.WriteFile(prBodyFile, []byte("<!-- changelog -->"), 0644))

	// ── build Banshee with fakeGithubClient ──────────────────────────────────

	globalConf := configs.GlobalConfig{
		Options: configs.OptionsConfig{LogLevel: "info"},
		Github:  configs.GithubConfig{Token: "testtoken"},
		Defaults: configs.DefaultsConfig{
			Organisation: "testorg",
			GitName:      "Test User",
			GitEmail:     "test@example.com",
		},
	}
	migConf := configs.MigrationConfig{
		BranchName:  "migration-branch",
		PRBodyFile:  prBodyFile,
		PRTitle:     "Test migration",
		ListOfRepos: []string{"testrepo"},
		Actions: []configs.Action{
			{
				Action:      "run_command",
				Description: "Add a file",
				Input:       map[string]string{"command": "touch migrated.txt"},
			},
		},
	}

	b, err := NewBanshee(context.Background(), globalConf, migConf)
	require.NoError(t, err)

	g := gitcli.NewExecGit(context.Background(), false, logrus.NewEntry(logrus.New()))
	b.GithubClient = &fakeGithubClient{
		git:           g,
		bareRepoURL:   bareDir,
		defaultBranch: branch,
		createdPRURL:  "https://github.com/testorg/testrepo/pull/1",
	}

	// ── run handleRepo ───────────────────────────────────────────────────────

	htmlURL, err := b.handleRepo(b.log.WithField("repo", "testorg/testrepo"), "testorg", "testorg/testrepo")
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/testorg/testrepo/pull/1", htmlURL)
	assert.Equal(t, int32(1), b.GithubClient.(*fakeGithubClient).pushCallCount.Load(), "Push should be called exactly once")

	// ── verify the commit was pushed to the bare repo ────────────────────────

	verifyDir := filepath.Join(t.TempDir(), "verify")
	run("git", "clone", "--branch", "migration-branch", bareDir, verifyDir)
	assert.FileExists(t, filepath.Join(verifyDir, "migrated.txt"),
		"migrated.txt should have been committed and pushed to the migration branch")
}

// TestHandleRepoWithWorktree verifies the worktree-based cache flow:
// push happens, worktree is cleaned up, and the cache dir stays on the default branch.
func TestHandleRepoWithWorktree(t *testing.T) {
	bareDir, branch := setupBareRepo(t)
	run := func(name string, args ...string) {
		t.Helper()
		out, err := exec.Command(name, args...).CombinedOutput()
		require.NoError(t, err, "%s %v: %s", name, args, out)
	}

	prBodyFile := filepath.Join(t.TempDir(), "pr-body.md")
	require.NoError(t, os.WriteFile(prBodyFile, []byte("<!-- changelog -->"), 0644))

	cacheDir := t.TempDir()

	globalConf := configs.GlobalConfig{
		Options: configs.OptionsConfig{
			LogLevel: "info",
			CacheRepos: configs.CacheReposConfig{
				Enabled:   true,
				Directory: cacheDir,
			},
		},
		Github: configs.GithubConfig{Token: "testtoken"},
		Defaults: configs.DefaultsConfig{
			Organisation: "testorg",
			GitName:      "Test User",
			GitEmail:     "test@example.com",
		},
	}
	migConf := configs.MigrationConfig{
		BranchName:  "migration-branch",
		PRBodyFile:  prBodyFile,
		PRTitle:     "Test worktree migration",
		ListOfRepos: []string{"testrepo"},
		Actions: []configs.Action{
			{
				Action:      "run_command",
				Description: "Add a file",
				Input:       map[string]string{"command": "touch migrated.txt"},
			},
		},
	}

	b, err := NewBanshee(context.Background(), globalConf, migConf)
	require.NoError(t, err)

	g := gitcli.NewExecGit(context.Background(), false, logrus.NewEntry(logrus.New()))
	b.GithubClient = &fakeGithubClient{
		git:           g,
		bareRepoURL:   bareDir,
		defaultBranch: branch,
		createdPRURL:  "https://github.com/testorg/testrepo/pull/2",
	}

	htmlURL, err := b.handleRepo(b.log.WithField("repo", "testorg/testrepo"), "testorg", "testorg/testrepo")
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/testorg/testrepo/pull/2", htmlURL)
	assert.Equal(t, int32(1), b.GithubClient.(*fakeGithubClient).pushCallCount.Load())

	// Verify worktree directory was cleaned up.
	worktreeDir := b.getWorktreePath("testorg", "testrepo")
	assert.NoDirExists(t, worktreeDir, "worktree should be cleaned up after handleRepo")

	// Verify cache dir still exists with .git on the default branch.
	cachePath := b.getCacheRepoPath("testorg", "testrepo")
	assert.DirExists(t, filepath.Join(cachePath, ".git"), "cache dir should still have .git")
	out, err := exec.Command("git", "-C", cachePath, "symbolic-ref", "--short", "HEAD").Output()
	require.NoError(t, err)
	assert.Equal(t, branch, strings.TrimSpace(string(out)), "cache should remain on default branch")

	// Verify the commit was pushed to the bare repo.
	verifyDir := filepath.Join(t.TempDir(), "verify")
	run("git", "clone", "--branch", "migration-branch", bareDir, verifyDir)
	assert.FileExists(t, filepath.Join(verifyDir, "migrated.txt"))
}

// TestHandleRepoNoChanges verifies that handleRepo returns an empty URL (no PR)
// when no action makes the repo dirty.
func TestHandleRepoNoChanges(t *testing.T) {
	bareDir, branch := setupBareRepo(t)

	prBodyFile := filepath.Join(t.TempDir(), "pr-body.md")
	require.NoError(t, os.WriteFile(prBodyFile, []byte("<!-- changelog -->"), 0644))

	globalConf := configs.GlobalConfig{
		Options:  configs.OptionsConfig{LogLevel: "info"},
		Github:   configs.GithubConfig{Token: "testtoken"},
		Defaults: configs.DefaultsConfig{Organisation: "testorg", GitName: "Test", GitEmail: "t@t.com"},
	}
	// Action that does nothing (no file created → repo stays clean).
	migConf := configs.MigrationConfig{
		BranchName:  "migration-branch",
		PRBodyFile:  prBodyFile,
		ListOfRepos: []string{"testrepo"},
		Actions: []configs.Action{
			{Action: "run_command", Description: "no-op", Input: map[string]string{"command": "true"}},
		},
	}

	b, err := NewBanshee(context.Background(), globalConf, migConf)
	require.NoError(t, err)
	g := gitcli.NewExecGit(context.Background(), false, logrus.NewEntry(logrus.New()))
	b.GithubClient = &fakeGithubClient{
		git: g, bareRepoURL: bareDir, defaultBranch: branch,
	}

	htmlURL, err := b.handleRepo(b.log.WithField("repo", "testorg/testrepo"), "testorg", "testorg/testrepo")
	require.NoError(t, err)
	assert.Empty(t, htmlURL, "no PR should be created when repo is clean")
	assert.Equal(t, int32(0), b.GithubClient.(*fakeGithubClient).pushCallCount.Load(), "Push should not be called when repo is clean")
}

// ── multiBareFakeClient ──────────────────────────────────────────────────────

// multiBareFakeClient wraps fakeGithubClient but creates a separate bare repo
// for each repo name, avoiding branch-name collisions when repos push in parallel.
type multiBareFakeClient struct {
	fakeGithubClient
	t     *testing.T
	bares map[string]string // repoName -> bareDir
	mu    sync.Mutex
}

func newMultiBareFake(t *testing.T, branch string) *multiBareFakeClient {
	t.Helper()
	return &multiBareFakeClient{
		fakeGithubClient: fakeGithubClient{
			git:           gitcli.NewExecGit(context.Background(), false, logrus.NewEntry(logrus.New())),
			defaultBranch: branch,
			createdPRURL:  "https://github.com/testorg/repo/pull/1",
		},
		t:     t,
		bares: make(map[string]string),
	}
}

func (m *multiBareFakeClient) bareFor(repoName string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if dir, ok := m.bares[repoName]; ok {
		return dir
	}
	dir, _ := setupBareRepo(m.t)
	m.bares[repoName] = dir
	return dir
}

func (m *multiBareFakeClient) ShallowClone(_, repoName, dir, migrationBranchName string) (string, error) {
	bareURL := m.bareFor(repoName)
	if _, err := os.Stat(dir + "/.git"); os.IsNotExist(err) {
		if err := m.git.Clone(bareURL, dir, m.defaultBranch, 0); err != nil {
			return "", err
		}
	} else {
		if err := m.git.Checkout(dir, m.defaultBranch, false); err != nil {
			return "", err
		}
	}
	return m.defaultBranch, m.git.Checkout(dir, migrationBranchName, true)
}

func (m *multiBareFakeClient) ShallowCloneWorktree(_, repoName, cacheDir, worktreeDir, migrationBranchName string) (string, error) {
	bareURL := m.bareFor(repoName)
	if _, err := os.Stat(cacheDir + "/.git"); os.IsNotExist(err) {
		if err := m.git.Clone(bareURL, cacheDir, m.defaultBranch, 0); err != nil {
			return "", err
		}
	} else {
		if err := m.git.Checkout(cacheDir, m.defaultBranch, false); err != nil {
			return "", err
		}
	}
	if err := m.git.WorktreeAdd(cacheDir, worktreeDir, migrationBranchName, true); err != nil {
		return "", err
	}
	return m.defaultBranch, nil
}

func (m *multiBareFakeClient) Push(branch, dir, _, repoName string) error {
	m.pushCallCount.Add(1)
	bareURL := m.bareFor(repoName)
	return m.git.Push(dir, bareURL, branch)
}

func (m *multiBareFakeClient) GitWorktreeRemove(repoDir, worktreeDir string) error {
	return m.git.WorktreeRemove(repoDir, worktreeDir)
}

func (m *multiBareFakeClient) GitWorktreePrune(repoDir string) error {
	return m.git.WorktreePrune(repoDir)
}

// Verify multiBareFakeClient satisfies the interface at compile time.
var _ githubClient = (*multiBareFakeClient)(nil)

// cancellingFake wraps multiBareFakeClient and cancels a context after N pushes.
type cancellingFake struct {
	*multiBareFakeClient
	cancel      context.CancelFunc
	cancelAfter int32
}

func (c *cancellingFake) Push(branch, dir, org, repoName string) error {
	err := c.multiBareFakeClient.Push(branch, dir, org, repoName)
	if c.pushCallCount.Load() >= c.cancelAfter {
		c.cancel()
	}
	return err
}

var _ githubClient = (*cancellingFake)(nil)

// TestMigrateParallel verifies that multiple repos are processed concurrently
// when Concurrency > 1 and cache_repos is enabled.
func TestMigrateParallel(t *testing.T) {
	// Each repo gets its own bare repo via multiBareFakeClient; we only need a branch name.
	branch := "main"

	prBodyFile := filepath.Join(t.TempDir(), "pr-body.md")
	require.NoError(t, os.WriteFile(prBodyFile, []byte("<!-- changelog -->"), 0644))

	cacheDir := t.TempDir()
	repos := []string{"repo1", "repo2", "repo3", "repo4", "repo5"}

	globalConf := configs.GlobalConfig{
		Options: configs.OptionsConfig{
			LogLevel:    "info",
			Concurrency: 3,
			CacheRepos: configs.CacheReposConfig{
				Enabled:   true,
				Directory: cacheDir,
			},
		},
		Github: configs.GithubConfig{Token: "testtoken"},
		Defaults: configs.DefaultsConfig{
			Organisation: "testorg",
			GitName:      "Test User",
			GitEmail:     "test@example.com",
		},
	}
	migConf := configs.MigrationConfig{
		BranchName:  "migration-branch",
		PRBodyFile:  prBodyFile,
		PRTitle:     "Test parallel migration",
		ListOfRepos: repos,
		Actions: []configs.Action{
			{
				Action:      "run_command",
				Description: "Add a file",
				Input:       map[string]string{"command": "touch migrated.txt"},
			},
		},
	}

	b, err := NewBanshee(context.Background(), globalConf, migConf)
	require.NoError(t, err)

	fake := newMultiBareFake(t, branch)
	b.GithubClient = fake

	err = b.Migrate()
	require.NoError(t, err)
	assert.Equal(t, int32(len(repos)), fake.pushCallCount.Load(),
		"Push should be called once per repo")
}

// TestMigrateDefaultConcurrency verifies that Concurrency=1 behaves
// identically to the original sequential loop.
func TestMigrateDefaultConcurrency(t *testing.T) {
	_, branch := setupBareRepo(t)

	prBodyFile := filepath.Join(t.TempDir(), "pr-body.md")
	require.NoError(t, os.WriteFile(prBodyFile, []byte("<!-- changelog -->"), 0644))

	globalConf := configs.GlobalConfig{
		Options: configs.OptionsConfig{
			LogLevel:    "info",
			Concurrency: 1,
		},
		Github: configs.GithubConfig{Token: "testtoken"},
		Defaults: configs.DefaultsConfig{
			Organisation: "testorg",
			GitName:      "Test User",
			GitEmail:     "test@example.com",
		},
	}
	migConf := configs.MigrationConfig{
		BranchName:  "migration-branch",
		PRBodyFile:  prBodyFile,
		PRTitle:     "Test sequential migration",
		ListOfRepos: []string{"repo1", "repo2"},
		Actions: []configs.Action{
			{
				Action:      "run_command",
				Description: "Add a file",
				Input:       map[string]string{"command": "touch migrated.txt"},
			},
		},
	}

	b, err := NewBanshee(context.Background(), globalConf, migConf)
	require.NoError(t, err)

	fake := newMultiBareFake(t, branch)
	b.GithubClient = fake

	err = b.Migrate()
	require.NoError(t, err)
	assert.Equal(t, int32(2), fake.pushCallCount.Load(),
		"Push should be called once per repo")
}

// TestMigrateCancellationPreCancelled verifies that a pre-cancelled context
// skips all repos without processing any.
func TestMigrateCancellationPreCancelled(t *testing.T) {
	branch := "main"

	prBodyFile := filepath.Join(t.TempDir(), "pr-body.md")
	require.NoError(t, os.WriteFile(prBodyFile, []byte("<!-- changelog -->"), 0644))

	cacheDir := t.TempDir()

	globalConf := configs.GlobalConfig{
		Options: configs.OptionsConfig{
			LogLevel:    "info",
			Concurrency: 1,
			CacheRepos: configs.CacheReposConfig{
				Enabled:   true,
				Directory: cacheDir,
			},
		},
		Github: configs.GithubConfig{Token: "testtoken"},
		Defaults: configs.DefaultsConfig{
			Organisation: "testorg",
			GitName:      "Test User",
			GitEmail:     "test@example.com",
		},
	}
	migConf := configs.MigrationConfig{
		BranchName:  "migration-branch",
		PRBodyFile:  prBodyFile,
		PRTitle:     "Test cancellation",
		ListOfRepos: []string{"repo1", "repo2", "repo3", "repo4", "repo5"},
		Actions: []configs.Action{
			{
				Action:      "run_command",
				Description: "Add a file",
				Input:       map[string]string{"command": "touch migrated.txt"},
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	b, err := NewBanshee(ctx, globalConf, migConf)
	require.NoError(t, err)

	fake := newMultiBareFake(t, branch)
	b.GithubClient = fake

	// Cancel immediately so no repos get dispatched
	cancel()

	err = b.Migrate()
	require.NoError(t, err)
	assert.Equal(t, int32(0), fake.pushCallCount.Load(),
		"No repos should be pushed when context is already cancelled")
}

// TestMigrateCancellationMidFlight verifies that cancelling the context
// mid-migration allows in-flight repos to complete while skipping remaining repos.
func TestMigrateCancellationMidFlight(t *testing.T) {
	branch := "main"

	prBodyFile := filepath.Join(t.TempDir(), "pr-body.md")
	require.NoError(t, os.WriteFile(prBodyFile, []byte("<!-- changelog -->"), 0644))

	cacheDir := t.TempDir()

	globalConf := configs.GlobalConfig{
		Options: configs.OptionsConfig{
			LogLevel:    "info",
			Concurrency: 1, // sequential so we can predict ordering
			CacheRepos: configs.CacheReposConfig{
				Enabled:   true,
				Directory: cacheDir,
			},
		},
		Github: configs.GithubConfig{Token: "testtoken"},
		Defaults: configs.DefaultsConfig{
			Organisation: "testorg",
			GitName:      "Test User",
			GitEmail:     "test@example.com",
		},
	}
	migConf := configs.MigrationConfig{
		BranchName:  "migration-branch",
		PRBodyFile:  prBodyFile,
		PRTitle:     "Test mid-flight cancellation",
		ListOfRepos: []string{"repo1", "repo2", "repo3", "repo4", "repo5"},
		Actions: []configs.Action{
			{
				Action:      "run_command",
				Description: "Add a file",
				Input:       map[string]string{"command": "touch migrated.txt"},
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	b, err := NewBanshee(ctx, globalConf, migConf)
	require.NoError(t, err)

	// Cancel after first repo completes by wrapping Push.
	fake := newMultiBareFake(t, branch)
	b.GithubClient = &cancellingFake{multiBareFakeClient: fake, cancel: cancel, cancelAfter: 1}

	_ = b.Migrate() // may or may not error depending on cancellation timing
	pushes := b.GithubClient.(*cancellingFake).pushCallCount.Load()
	assert.GreaterOrEqual(t, pushes, int32(1), "At least one repo should have completed before cancel")
	assert.Less(t, pushes, int32(5), "Not all repos should have completed")
}

// TestMigrateConcurrencyRequiresCacheRepos verifies that concurrency > 1
// without cache_repos returns a clear error.
func TestMigrateConcurrencyRequiresCacheRepos(t *testing.T) {
	globalConf := configs.GlobalConfig{
		Options: configs.OptionsConfig{
			LogLevel:    "info",
			Concurrency: 2,
			CacheRepos:  configs.CacheReposConfig{Enabled: false},
		},
		Github:   configs.GithubConfig{Token: "testtoken"},
		Defaults: configs.DefaultsConfig{Organisation: "testorg"},
	}
	migConf := configs.MigrationConfig{
		ListOfRepos: []string{"repo1"},
	}

	b, err := NewBanshee(context.Background(), globalConf, migConf)
	require.NoError(t, err)

	err = b.Migrate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parallel migration (concurrency > 1) requires repo caching")
}
