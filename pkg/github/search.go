// Getting repos to perform the code changes in - probably choose between a GitHub search and graphql (?)
package github

import (
	"strings"

	"github.com/avast/retry-go/v4"
	"github.com/google/go-github/v52/github"
)

func (gc *GithubClient) GetAllRepos(owner string) ([]string, error) {
	repos := []string{}
	opt := &github.RepositoryListOptions{Type: "owner", Sort: "created", Direction: "asc"}

	for {

		var searchResult []*github.Repository
		var resp *github.Response
		searchErr := retry.Do(
			func() error {
				var err error
				searchResult, resp, err = gc.Client.Repositories.List(gc.ctx, owner, opt)
				return checkIfRecoverable(err)
			},
			defaultRetryOptions...,
		)

		if searchErr != nil {
			return nil, searchErr
		}

		for _, result := range searchResult {
			repos = append(repos, *result.FullName)
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return repos, nil
}

// Get all repos returned by a code search query
func (gc *GithubClient) GetMatchingRepos(query string) ([]string, error) {
	repos := []string{}

	pageOpts := github.ListOptions{PerPage: 100}
	opt := &github.SearchOptions{Sort: "created", Order: "asc", ListOptions: pageOpts}

	for {

		var searchResult *github.CodeSearchResult
		var resp *github.Response
		searchErr := retry.Do(
			func() error {
				var err error
				searchResult, resp, err = gc.Client.Search.Code(gc.ctx, query, opt)
				return checkIfRecoverable(err)
			},
			defaultRetryOptions...,
		)

		if searchErr != nil {
			return nil, searchErr
		}

		for _, result := range searchResult.CodeResults {
			repos = append(repos, *result.Repository.FullName)
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return removeDuplicateStr(repos), nil
}

func (gc *GithubClient) GetMatchingPRs(query string) ([]*github.PullRequest, error) {
	pullRequests := []*github.PullRequest{}

	pageOpts := github.ListOptions{PerPage: 100}
	opt := &github.SearchOptions{Sort: "created", Order: "asc", ListOptions: pageOpts}

	for {

		var searchResult *github.IssuesSearchResult
		var resp *github.Response
		searchErr := retry.Do(
			func() error {
				var err error
				searchResult, resp, err = gc.Client.Search.Issues(gc.ctx, query, opt)
				return checkIfRecoverable(err)
			},
			defaultRetryOptions...,
		)

		if searchErr != nil {
			return nil, searchErr
		}

		// Convert every issue into a pull request
		for _, issue := range searchResult.Issues {
			repoOwner, repoName := gc.getRepoNameFromURL(*issue.HTMLURL)
			pullRequest, prErr := gc.GetPR(repoOwner, repoName, *issue.Number)
			if prErr != nil {
				return nil, prErr
			}
			pullRequests = append(pullRequests, pullRequest)
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return pullRequests, nil
}

func (gc *GithubClient) getRepoNameFromURL(url string) (string, string) {
	// https://github.com/octocat/Hello-World/pull/1347
	url = strings.ReplaceAll(url, "https://github.com/", "")
	pieces := strings.Split(url, "/")
	return pieces[0], pieces[1]
}

func (gc *GithubClient) GetPR(owner, repo string, number int) (*github.PullRequest, error) {
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

func removeDuplicateStr(strSlice []string) []string {
	allKeys := make(map[string]bool)
	list := []string{}
	for _, item := range strSlice {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}
