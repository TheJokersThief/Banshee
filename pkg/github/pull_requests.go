// pull request interactions - open/close/nag code reviewers for approval
package github

import (
	"fmt"

	"github.com/google/go-github/v52/github"
)

func (gc *GithubClient) FindPullRequest(org, repo, baseBranch, headBranch string) (*github.PullRequest, error) {
	opts := &github.PullRequestListOptions{
		State: "open",
		Base:  baseBranch,
		Head:  fmt.Sprintf("%s:%s", org, headBranch),
	}
	prs, _, err := gc.Client.PullRequests.List(gc.ctx, org, repo, opts)
	if err != nil {
		return nil, err
	}

	if len(prs) > 0 {
		return prs[0], nil
	}

	return nil, nil
}

func (gc *GithubClient) CreatePullRequest(org, repo, title, body, base_branch, merge_branch string) (string, error) {
	asDraft := true
	newPR := &github.NewPullRequest{
		Title:               github.String(title),
		Head:                github.String(merge_branch),
		Base:                github.String(base_branch),
		Body:                github.String(body),
		MaintainerCanModify: github.Bool(true),
		Draft:               &asDraft,
	}

	pr, _, err := gc.Client.PullRequests.Create(gc.ctx, org, repo, newPR)

	if err != nil {
		ghErr := err.(*github.ErrorResponse)
		if ghErr.Message != "Validation Failed" {
			return "", err
		}
	}

	if gc.GlobalConfig.Options.AssignCodeReviewerIfNoneAssigned {
		gc.AssignDefaultReviewer(org, repo, *pr.Number)
	}

	return pr.GetHTMLURL(), nil
}

func (gc *GithubClient) UpdatePullRequest(pr *github.PullRequest, org, repo, body string) error {
	pr.Body = &body
	pr, _, err := gc.Client.PullRequests.Edit(gc.ctx, org, repo, *pr.Number, pr)
	if err != nil {
		return err
	}

	return nil
}

func (gc *GithubClient) AssignDefaultReviewer(org, repo string, prNumber int) error {

	listOpts := &github.ListOptions{}
	reviewers, _, err := gc.Client.PullRequests.ListReviewers(gc.ctx, org, repo, prNumber, listOpts)
	if err != nil {
		return err
	}

	if len(reviewers.Teams) > 0 && len(reviewers.Users) > 0 {
		// If there are no reviewers, assign some
		reviewRequest := github.ReviewersRequest{
			TeamReviewers: []string{gc.GlobalConfig.Defaults.CodeReviewer},
		}

		_, _, reviewerErr := gc.Client.PullRequests.RequestReviewers(gc.ctx, org, repo, prNumber, reviewRequest)
		if reviewerErr != nil {
			return reviewerErr
		}
	}

	return nil
}
