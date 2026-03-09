// pull request interactions - open/close/nag code reviewers for approval
package github

import (
	"errors"
	"fmt"
	"strings"

	"github.com/avast/retry-go/v4"
	"github.com/google/go-github/v63/github"
	"github.com/sirupsen/logrus"
)

func (gc *GithubClient) FindPullRequest(org, repo, baseBranch, headBranch string) (*github.PullRequest, error) {
	if err := gc.waitRateLimit(); err != nil {
		return nil, err
	}

	opts := &github.PullRequestListOptions{
		State: "open",
		Base:  baseBranch,
		Head:  fmt.Sprintf("%s:%s", org, headBranch),
	}

	var prs []*github.PullRequest
	searchErr := retry.Do(
		func() error {
			var err error
			prs, _, err = gc.Client.PullRequests.List(gc.ctx, org, repo, opts)
			return checkIfRecoverable(err)
		},
		defaultRetryOptions...,
	)
	if searchErr != nil {
		return nil, searchErr
	}

	if len(prs) > 0 {
		return prs[0], nil
	}

	return nil, nil
}

func (gc *GithubClient) CreatePullRequest(org, repo, title, body, base_branch, merge_branch string, asDraft bool) (string, error) {
	if err := gc.waitRateLimit(); err != nil {
		return "", err
	}

	newPR := &github.NewPullRequest{
		Title:               github.String(title),
		Head:                github.String(merge_branch),
		Base:                github.String(base_branch),
		Body:                github.String(body),
		MaintainerCanModify: github.Bool(true),
		Draft:               github.Bool(asDraft),
	}

	var pr *github.PullRequest
	createErr := retry.Do(
		func() error {
			var err error
			pr, _, err = gc.Client.PullRequests.Create(gc.ctx, org, repo, newPR)
			if err != nil {
				var errResponse *github.ErrorResponse
				if errors.As(err, &errResponse) {
					if len(errResponse.Errors) > 0 && strings.Contains(errResponse.Errors[0].Message, "No commits between") {
						return nil // not an error — no commits to PR
					}
				}
			}
			return checkIfRecoverable(err)
		},
		defaultRetryOptions...,
	)
	if createErr != nil {
		return "", createErr
	}
	if pr == nil {
		return "", nil // "No commits between" case
	}

	gc.log.WithField("AssignReviewers", gc.GlobalConfig.Options.AssignCodeReviewerIfNoneAssigned).Debug("Assigning reviewers, if enabled")
	if gc.GlobalConfig.Options.AssignCodeReviewerIfNoneAssigned {
		assignmentErr := gc.AssignDefaultReviewer(pr)
		if assignmentErr != nil {
			return "", assignmentErr
		}
	}

	return pr.GetHTMLURL(), nil
}

func (gc *GithubClient) MergePullRequest(pr *github.PullRequest) error {
	if err := gc.waitRateLimit(); err != nil {
		return err
	}

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
	if err := gc.waitRateLimit(); err != nil {
		return err
	}

	owner, repo := gc.getRepoNameFromURL(*pr.HTMLURL)
	pr.Body = &body

	return retry.Do(
		func() error {
			_, _, err := gc.Client.PullRequests.Edit(gc.ctx, owner, repo, *pr.Number, pr)
			return checkIfRecoverable(err)
		},
		defaultRetryOptions...,
	)
}

func (gc *GithubClient) AssignDefaultReviewer(pr *github.PullRequest) error {
	if err := gc.waitRateLimit(); err != nil {
		return err
	}

	owner, repo := gc.getRepoNameFromURL(*pr.HTMLURL)

	var reviewers *github.Reviewers
	listOpts := &github.ListOptions{}
	listErr := retry.Do(
		func() error {
			var err error
			reviewers, _, err = gc.Client.PullRequests.ListReviewers(gc.ctx, owner, repo, *pr.Number, listOpts)
			return checkIfRecoverable(err)
		},
		defaultRetryOptions...,
	)
	if listErr != nil {
		return listErr
	}

	gc.log.WithFields(logrus.Fields{"Teams": reviewers.Teams, "Users": reviewers.Users}).Debug("Checking for reviewers")
	if len(reviewers.Teams) == 0 && len(reviewers.Users) == 0 {
		reviewRequest := github.ReviewersRequest{
			TeamReviewers: []string{gc.GlobalConfig.Defaults.CodeReviewer},
		}

		gc.log.Info("Assigning ", gc.GlobalConfig.Defaults.CodeReviewer, " as reviewers")
		return retry.Do(
			func() error {
				_, _, err := gc.Client.PullRequests.RequestReviewers(gc.ctx, owner, repo, *pr.Number, reviewRequest)
				return checkIfRecoverable(err)
			},
			defaultRetryOptions...,
		)
	}

	return nil
}

func (gc *GithubClient) GetPR(owner, repo string, number int) (*github.PullRequest, error) {
	if err := gc.waitRateLimit(); err != nil {
		return nil, err
	}

	var pullRequest *github.PullRequest
	searchErr := retry.Do(
		func() error {
			var err error
			pullRequest, _, err = gc.Client.PullRequests.Get(gc.ctx, owner, repo, number)
			return checkIfRecoverable(err)
		},
		defaultRetryOptions...,
	)

	return pullRequest, searchErr
}
