// Getting repos to perform the code changes in - probably choose between a GitHub search and graphql (?)
package github

import (
	"errors"
	"fmt"
	"math"

	"github.com/avast/retry-go/v4"
	"github.com/google/go-github/v52/github"
)

func (gc *GithubClient) GetMatchingPRs(query string) ([]*github.PullRequest, error) {
	prWorker := NewPRDataWorker(gc)
	pageOpts := github.ListOptions{PerPage: 100}
	opt := &github.SearchOptions{Sort: "created", Order: "asc", ListOptions: pageOpts}

	prWorker.spawnWorkers()
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
			prWorker.issues <- issue
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	close(prWorker.issues)

	return prWorker.processResults(), prWorker.processErrors()
}

const (
	prDataWorkers = 5
)

type PRDataWorker struct {
	client  *GithubClient
	issues  chan *github.Issue
	results chan *github.PullRequest
	errChan chan error
}

func NewPRDataWorker(client *GithubClient) *PRDataWorker {
	return &PRDataWorker{
		client:  client,
		issues:  make(chan *github.Issue, 64),
		results: make(chan *github.PullRequest, math.MaxInt32),
		errChan: make(chan error, math.MaxInt32),
	}
}

// Spawn workers to process PR data
func (w *PRDataWorker) spawnWorkers() {
	for workerIndex := 0; workerIndex < prDataWorkers; workerIndex++ {
		go w.prDataWorker()
	}
}

// Process issues and transform them into PRs
func (w *PRDataWorker) prDataWorker() {
	for issue := range w.issues {
		repoOwner, repoName := w.client.getRepoNameFromURL(*issue.HTMLURL)
		pullRequest, prErr := w.client.GetPR(repoOwner, repoName, *issue.Number)
		if prErr != nil {
			w.errChan <- prErr
			continue
		}

		w.results <- pullRequest
		continue
	}
}

// Assemble our pull requests for returning
func (w *PRDataWorker) processResults() []*github.PullRequest {
	pullRequests := []*github.PullRequest{}
	for resultIndex := 0; resultIndex < len(w.results); resultIndex++ {
		pr := <-w.results
		pullRequests = append(pullRequests, pr)
	}
	close(w.results)
	return pullRequests
}

// If there are any errors, join them into a single error and return the error
func (w *PRDataWorker) processErrors() error {
	if len(w.errChan) > 0 {
		finalError := fmt.Errorf("")
		for i := 0; i < len(w.errChan); i++ {
			prGetErr := <-w.errChan
			finalError = errors.Join(finalError, prGetErr)
		}
		return finalError
	}
	close(w.errChan)

	return nil
}
