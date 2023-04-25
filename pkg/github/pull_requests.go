// pull request interactions - open/close/nag code reviewers for approval
package github

import (
	"context"

	"github.com/google/go-github/v52/github"
)

func (g *GithubClient) CreatePullRequest(org, repo, title, body, base_branch, merge_branch string) (string, error) {
	asDraft := true
	newPR := &github.NewPullRequest{
		Title:               github.String(title),
		Head:                github.String(merge_branch),
		Base:                github.String(base_branch),
		Body:                github.String(body),
		MaintainerCanModify: github.Bool(true),
		Draft:               &asDraft,
	}

	ctx := context.Background()
	pr, _, err := g.Client.PullRequests.Create(ctx, org, repo, newPR)
	if err != nil {
		return "", err
	}

	return pr.GetHTMLURL(), nil
}
