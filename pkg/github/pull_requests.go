// pull request interactions - open/close/nag code reviewers for approval
package github

import (
	"github.com/google/go-github/v52/github"
)

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

func (gc *GithubClient) AssignDefaultReviewer(org, repo string, prNumber int) error {

	listOpts := &github.ListOptions{}
	prService := github.PullRequestsService{}
	reviewers, _, err := prService.ListReviewers(gc.ctx, org, repo, prNumber, listOpts)
	if err != nil {
		return err
	}

	if len(reviewers.Teams) > 0 && len(reviewers.Users) > 0 {
		// If there are no reviewers, assign some
		reviewRequest := github.ReviewersRequest{
			TeamReviewers: []string{gc.GlobalConfig.Defaults.CodeReviewer},
		}

		_, _, reviewerErr := prService.RequestReviewers(gc.ctx, org, repo, prNumber, reviewRequest)
		if reviewerErr != nil {
			return reviewerErr
		}
	}

	return nil
}
