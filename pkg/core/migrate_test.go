package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	b, err := NewBanshee(globalConf, mainConf)
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
	pushCallCount int
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

func (f *fakeGithubClient) GetDefaultBranch(_, _ string) (string, error) {
	return f.defaultBranch, nil
}
func (f *fakeGithubClient) GitIsClean(dir string) (bool, error)  { return f.git.IsClean(dir) }
func (f *fakeGithubClient) GitAddAll(dir string) error            { return f.git.AddAll(dir) }
func (f *fakeGithubClient) GitCommit(dir, msg, name, email string) error {
	return f.git.Commit(dir, msg, name, email)
}
func (f *fakeGithubClient) Push(branch, dir, _, _ string) error {
	f.pushCallCount++
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

	b, err := NewBanshee(globalConf, migConf)
	require.NoError(t, err)

	g := gitcli.NewExecGit(false, logrus.NewEntry(logrus.New()))
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
	assert.Equal(t, 1, b.GithubClient.(*fakeGithubClient).pushCallCount, "Push should be called exactly once")

	// ── verify the commit was pushed to the bare repo ────────────────────────

	verifyDir := filepath.Join(t.TempDir(), "verify")
	run("git", "clone", "--branch", "migration-branch", bareDir, verifyDir)
	assert.FileExists(t, filepath.Join(verifyDir, "migrated.txt"),
		"migrated.txt should have been committed and pushed to the migration branch")
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

	b, err := NewBanshee(globalConf, migConf)
	require.NoError(t, err)
	g := gitcli.NewExecGit(false, logrus.NewEntry(logrus.New()))
	b.GithubClient = &fakeGithubClient{
		git: g, bareRepoURL: bareDir, defaultBranch: branch,
	}

	htmlURL, err := b.handleRepo(b.log.WithField("repo", "testorg/testrepo"), "testorg", "testorg/testrepo")
	require.NoError(t, err)
	assert.Empty(t, htmlURL, "no PR should be created when repo is clean")
	assert.Equal(t, 0, b.GithubClient.(*fakeGithubClient).pushCallCount, "Push should not be called when repo is clean")
}
