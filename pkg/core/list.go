package core

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/sirupsen/logrus"
)

const (
	header = "state\tmergeable_state\thtml_url"
)

// Perform a migration
func (b *Banshee) List(state string, format string) error {

	b.log = logrus.WithField("command", "migrate")

	if validationErr := b.validateMigrateCommand(); validationErr != nil {
		return validationErr
	}

	prList, prListErr := b.GithubClient.GetMatchingPRs(b.formatPRQuery(state))
	if prListErr != nil {
		return prListErr
	}

	switch format {
	case "json":
		jsonOutput, jsonErr := json.Marshal(prList)
		if jsonErr != nil {
			return jsonErr
		}

		fmt.Println(string(jsonOutput))
	default:
		w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
		fmt.Fprintln(w, header)
		for _, pr := range prList {
			line := strings.Join([]string{*pr.State, *pr.MergeableState, *pr.HTMLURL}, "\t")
			fmt.Fprintln(w, line)
		}
		w.Flush()
	}

	return nil
}

func (b *Banshee) formatPRQuery(state string) string {
	stateQuery := ""
	if state != "all" {
		stateQuery = fmt.Sprintf("state:%s", state)
	}
	return fmt.Sprintf("is:pr org:%s head:%s %s", b.getOrgName(), b.MigrationConfig.BranchName, stateQuery)
}
