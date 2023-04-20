// Config for each code change
package configs

import (
	"github.com/thejokersthief/banshee/pkg/actions"
)

type MigrationConfig struct {
	Organisation string              `fig:"organisation"`
	SearchQuery  string              `fig:"search_query"`
	Actions      []actions.Action    `fig:"actions"`
	Condition    []actions.Condition `fig:"condition"`
	PostCheckout []string            `fig:"post_checkout_commands"`

	PRTitle    string `fig:"pr_title"`
	PRBodyFile string `fig:"pr_body_file"`
}
