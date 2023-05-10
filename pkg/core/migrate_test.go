package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thejokersthief/banshee/pkg/configs"
)

func TestMigrationOptions(t *testing.T) {
	var want_org, got_org string
	var want_repos, got_repos []string
	var optionsErr error

	mainConf := configs.MigrationConfig{
		SearchQuery:   "",
		ListOfRepos:   []string{"repo_name"},
		AllReposInOrg: false,
	}
	globalConf := configs.GlobalConfig{
		Github:   configs.GithubConfig{Token: "testtoken"},
		Defaults: configs.DefaultsConfig{Organisation: "testorg"},
	}
	b, err := NewBanshee(globalConf, mainConf)
	assert.NoError(t, err)

	want_org = "testorg"
	want_repos = []string{"repo_name"}
	got_org, got_repos, optionsErr = b.migrationOptions()
	assert.NoError(t, optionsErr)
	assert.Equal(t, want_org, got_org)
	assert.Equal(t, want_repos, got_repos)

}
