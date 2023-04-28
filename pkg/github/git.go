package github

import (
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	gitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

const defaultRemote = "origin"

// Checkout a local branch, switching the working tree
func (gc *GithubClient) Checkout(branch string, gitRepo *git.Repository, create bool) error {
	wt, wtErr := gitRepo.Worktree()
	if wtErr != nil {
		return wtErr
	}

	checkoutErr := wt.Checkout(
		&git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName(branch),
			Create: create,
			Keep:   true,
		},
	)

	if checkoutErr != nil && !strings.Contains(checkoutErr.Error(), "already exists") {
		return checkoutErr
	}

	return nil
}

// Fetch from remote branch
func (gc *GithubClient) Fetch(branch string, gitRepo *git.Repository) error {
	gc.log.Debug("Fetching references for ", plumbing.NewBranchReferenceName(branch))
	fetchErr := gitRepo.Fetch(&git.FetchOptions{
		Progress: gc.Writer,
		Auth: &gitHttp.BasicAuth{
			Username: "placeholderUsername", // anything except an empty string
			Password: gc.accessToken,
		},
		RefSpecs: []config.RefSpec{
			config.RefSpec(plumbing.NewBranchReferenceName(branch) + ":" + plumbing.NewRemoteReferenceName(defaultRemote, branch)),
		},
	})

	if fetchErr != nil && (fetchErr != git.NoErrAlreadyUpToDate && !strings.Contains(fetchErr.Error(), "couldn't find remote ref")) {
		return fetchErr
	}
	return nil
}

// Pull from remote branch
func (gc *GithubClient) Pull(branch string, gitRepo *git.Repository) error {
	wt, wtErr := gitRepo.Worktree()
	if wtErr != nil {
		return wtErr
	}

	gc.log.Debug("Pulling ", plumbing.NewBranchReferenceName(branch))
	pullErr := wt.Pull(&git.PullOptions{
		Progress:      gc.Writer,
		RemoteName:    "origin",
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		Auth: &gitHttp.BasicAuth{
			Username: "placeholderUsername", // anything except an empty string
			Password: gc.accessToken,
		},
		SingleBranch: true,
	})

	if pullErr != nil && (pullErr != git.NoErrAlreadyUpToDate && pullErr.Error() != "reference not found") {
		return pullErr
	}

	return nil
}

// Push to remote branch
func (gc *GithubClient) Push(branch string, gitRepo *git.Repository) error {
	gc.log.Debug("Pushing changes")

	pushErr := gitRepo.Push(
		&git.PushOptions{
			Progress:   gc.Writer,
			RemoteName: "origin",
			Auth: &gitHttp.BasicAuth{
				Username: "placeholderUsername", // anything except an empty string
				Password: gc.accessToken,
			},
			RefSpecs: []config.RefSpec{
				config.RefSpec(plumbing.NewBranchReferenceName(branch) + ":" + plumbing.NewRemoteReferenceName(defaultRemote, branch)),
			},
		},
	)

	if pushErr != nil && pushErr != git.NoErrAlreadyUpToDate {
		return pushErr
	}
	return nil
}
