// Config for each code change
package configs

import (
	"github.com/thejokersthief/banshee/pkg/banshee"
	"github.com/thejokersthief/banshee/pkg/banshee/actions"
)

type MigrationConfig struct {
	SearchQuery  string
	Actions      []actions.Action
	Condition    []banshee.Condition
	PostCheckout []string

	PRTitle    string
	PRBodyFile string
}
