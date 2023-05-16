// pull request interactions - open/close/nag code reviewers for approval
package github

import (
	"errors"
	"fmt"
	"strings"

	"github.com/avast/retry-go/v4"
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

func (gc *GithubClient) CreatePullRequest(org, repo, title, body, base_branch, merge_branch string, asDraft bool) (string, error) {
	newPR := &github.NewPullRequest{
		Title:               github.String(title),
		Head:                github.String(merge_branch),
		Base:                github.String(base_branch),
		Body:                github.String(body),
		MaintainerCanModify: github.Bool(true),
		Draft:               github.Bool(asDraft),
	}

	pr, _, err := gc.Client.PullRequests.Create(gc.ctx, org, repo, newPR)
	if err != nil {
		var errResponse *github.ErrorResponse
		ghErr := errors.As(err, &errResponse)
		if ghErr {
			if strings.Contains(errResponse.Errors[0].Message, "No commits between") {
				return "", nil
			}
			return "", err
		}
	}

	if gc.GlobalConfig.Options.AssignCodeReviewerIfNoneAssigned {
		assignmentErr := gc.AssignDefaultReviewer(pr)
		if assignmentErr != nil {
			return "", assignmentErr
		}
	}

	return pr.GetHTMLURL(), nil
}

func (gc *GithubClient) MergePullRequest(pr *github.PullRequest) error {
	owner, repo := gc.getRepoNameFromURL(*pr.HTMLURL)

	options := &github.PullRequestOptions{MergeMethod: gc.GlobalConfig.Options.Merges.Strategy}
	searchErr := retry.Do(
		func() error {
			var err error
			_, _, err = gc.Client.PullRequests.Merge(
				gc.ctx, owner, repo, *pr.Number, gc.GlobalConfig.Options.Merges.AppendTitle, options)
			return checkIfRecoverable(err)
		},
		defaultRetryOptions...,
	)

	if searchErr != nil {
		return searchErr
	}

	return nil
}

func (gc *GithubClient) UpdatePullRequest(pr *github.PullRequest, body string) error {
	owner, repo := gc.getRepoNameFromURL(*pr.HTMLURL)

	pr.Body = &body
	_, _, err := gc.Client.PullRequests.Edit(gc.ctx, owner, repo, *pr.Number, pr)
	if err != nil {
		return err
	}

	return nil
}

func (gc *GithubClient) AssignDefaultReviewer(pr *github.PullRequest) error {

	owner, repo := gc.getRepoNameFromURL(*pr.HTMLURL)

	listOpts := &github.ListOptions{}
	reviewers, _, err := gc.Client.PullRequests.ListReviewers(gc.ctx, owner, repo, *pr.Number, listOpts)
	if err != nil {
		return err
	}

	if len(reviewers.Teams) > 0 && len(reviewers.Users) > 0 {
		// If there are no reviewers, assign some
		reviewRequest := github.ReviewersRequest{
			TeamReviewers: []string{gc.GlobalConfig.Defaults.CodeReviewer},
		}

		_, _, reviewerErr := gc.Client.PullRequests.RequestReviewers(gc.ctx, owner, repo, *pr.Number, reviewRequest)
		if reviewerErr != nil {
			return reviewerErr
		}
	}

	return nil
}
