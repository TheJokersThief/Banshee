// Config for each code change
package configs

import (
	"github.com/thejokersthief/banshee/pkg/banshee"
	"github.com/thejokersthief/banshee/pkg/banshee/actions"
)

type MigrationConfig struct {
	SearchQuery  string              `fig:"search_query"`
	Actions      []actions.Action    `fig:"actions"`
	Condition    []banshee.Condition `fig:"condition"`
	PostCheckout []string            `fig:"post_checkout_commands"`

	PRTitle    string `fig:"pr_title"`
	PRBodyFile string `fig:"pr_body_file"`
}
