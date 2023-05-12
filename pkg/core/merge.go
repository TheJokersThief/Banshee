package core

import (
	"github.com/sirupsen/logrus"
)

const (
	openState = "open"
)

// Perform a migration
func (b *Banshee) MergeApproved() error {

	b.log = logrus.WithField("command", "migrate")

	if validationErr := b.validateMigrateCommand(); validationErr != nil {
		return validationErr
	}

	query := b.formatPRQuery(openState)
	b.log.Debug("Getting list of PRs matching: \"", query, "\"")
	prList, prListErr := b.GithubClient.GetMatchingPRs(query)
	if prListErr != nil {
		return prListErr
	}

	for _, pr := range prList {
		if *pr.MergeableState == "mergeable" {
			b.GithubClient.MergePullRequest(pr)
		}
	}

	return nil
}
