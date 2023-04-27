package github

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	gitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

const defaultRemote = "origin"

// Checkout a local branch, switching the working tree
func (gc *GithubClient) Checkout(branch string, gitRepo *git.Repository) error {
	wt, wtErr := gitRepo.Worktree()
	if wtErr != nil {
		return wtErr
	}

	h, headErr := gitRepo.Head()
	if headErr != nil {
		return headErr
	}

	checkoutErr := wt.Checkout(
		&git.CheckoutOptions{
			Hash:   h.Hash(),
			Branch: plumbing.NewBranchReferenceName(branch),
			Create: true,
			Keep:   true,
		},
	)

	return checkoutErr
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
	return fetchErr
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

	return pullErr
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
				config.RefSpec(plumbing.NewBranchReferenceName(branch) + ":" + plumbing.NewBranchReferenceName(branch)),
			},
		},
	)

	return pushErr
}