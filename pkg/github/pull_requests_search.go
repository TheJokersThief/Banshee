// Getting repos to perform the code changes in - probably choose between a GitHub search and graphql (?)
package github

import (
	"errors"
	"sync"

	"github.com/avast/retry-go/v4"
	"github.com/google/go-github/v63/github"
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
	prWorker.shutdownWorkers()
	results := prWorker.processResults()

	return results, prWorker.processErrors()
}

const (
	prDataWorkers = 5
)

type PRDataWorker struct {
	client    *GithubClient
	waitGroup *sync.WaitGroup
	issues    chan *github.Issue
	mu        sync.Mutex
	results   []*github.PullRequest
	errs      []error
}

func NewPRDataWorker(client *GithubClient) *PRDataWorker {
	return &PRDataWorker{
		client:    client,
		waitGroup: &sync.WaitGroup{},
		issues:    make(chan *github.Issue, 64),
	}
}

// Spawn workers to process PR data
func (w *PRDataWorker) spawnWorkers() {
	for workerIndex := 0; workerIndex < prDataWorkers; workerIndex++ {
		w.waitGroup.Add(1)
		go func() {
			defer w.waitGroup.Done()
			w.prDataWorker()
		}()
	}
}

func (w *PRDataWorker) shutdownWorkers() {
	close(w.issues)
	w.waitGroup.Wait()
}

// Process issues and transform them into PRs
func (w *PRDataWorker) prDataWorker() {
	for issue := range w.issues {
		repoOwner, repoName := w.client.getRepoNameFromURL(*issue.HTMLURL)
		pullRequest, prErr := w.client.GetPR(repoOwner, repoName, *issue.Number)
		w.mu.Lock()
		if prErr != nil {
			w.errs = append(w.errs, prErr)
		} else {
			w.results = append(w.results, pullRequest)
		}
		w.mu.Unlock()
	}
}

// Assemble our pull requests for returning
func (w *PRDataWorker) processResults() []*github.PullRequest {
	return w.results
}

// If there are any errors, join them into a single error and return the error
func (w *PRDataWorker) processErrors() error {
	if len(w.errs) == 0 {
		return nil
	}
	return errors.Join(w.errs...)
}
