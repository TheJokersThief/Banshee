// pull request interactions - open/close/nag code reviewers for approval
package github

import (
	"context"

	"github.com/google/go-github/github"
)

func (g *GithubClient) CreatePullRequest(org, repo, title, body, base_branch, merge_branch string) (string, error) {
	newPR := &github.NewPullRequest{
		Title:               github.String(title),
		Head:                github.String(merge_branch),
		Base:                github.String(base_branch),
		Body:                github.String(body),
		MaintainerCanModify: github.Bool(true),
	}

	ctx := context.Background()
	pr, _, err := g.Client.PullRequests.Create(ctx, org, repo, newPR)
	if err != nil {
		return "", err
	}

	return pr.GetHTMLURL(), nil
}
