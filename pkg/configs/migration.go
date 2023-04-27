// Config for each code change
package configs

import (
	"github.com/thejokersthief/banshee/pkg/actions"
)

type MigrationConfig struct {
	BranchName   string   `fig:"branch_name"`
	Organisation string   `fig:"organisation"`
	SearchQuery  string   `fig:"search_query"`
	ListOfRepos  []string `fig:"repos"`

	Actions []actions.Action `fig:"actions"`

	PRTitle    string `fig:"pr_title"`
	PRBodyFile string `fig:"pr_body_file"`
}
