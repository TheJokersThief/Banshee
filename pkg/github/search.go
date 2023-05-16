// Getting repos to perform the code changes in - probably choose between a GitHub search and graphql (?)
package github

import (
	"github.com/avast/retry-go/v4"
	"github.com/google/go-github/v52/github"
)

func (gc *GithubClient) GetAllRepos(owner string) ([]string, error) {
	repos := []string{}
	opt := &github.RepositoryListByOrgOptions{Type: "all", Sort: "created", Direction: "asc"}

	for {

		var searchResult []*github.Repository
		var resp *github.Response
		searchErr := retry.Do(
			func() error {
				var err error
				searchResult, resp, err = gc.Client.Repositories.ListByOrg(gc.ctx, owner, opt)
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
