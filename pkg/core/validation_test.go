package core

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thejokersthief/banshee/pkg/configs"
)

func TestOnlyOneRepoChoice(t *testing.T) {
	var want, got error

	mainConf := configs.MigrationConfig{
		SearchQuery:   "Test query",
		ListOfRepos:   []string{"repo_name"},
		AllReposInOrg: true,
	}
	globalConf := configs.GlobalConfig{Github: configs.GithubConfig{Token: "testtoken"}}
	b, err := NewBanshee(globalConf, mainConf)
	assert.NoError(t, err)

	want = MustUseOneErr
	b.MigrationConfig = &configs.MigrationConfig{}
	got = b.OnlyOneRepoChoice()
	assert.ErrorIs(t, got, want)

	want = OnlyUseOneErr
	b.MigrationConfig = &configs.MigrationConfig{SearchQuery: mainConf.SearchQuery, ListOfRepos: mainConf.ListOfRepos}
	got = b.OnlyOneRepoChoice()
	fmt.Println(got)
	assert.ErrorIs(t, got, want)

	want = OnlyUseOneErr
	b.MigrationConfig = &configs.MigrationConfig{ListOfRepos: mainConf.ListOfRepos, AllReposInOrg: mainConf.AllReposInOrg}
	got = b.OnlyOneRepoChoice()
	assert.ErrorIs(t, got, want)

	want = OnlyUseOneErr
	b.MigrationConfig = &configs.MigrationConfig{SearchQuery: mainConf.SearchQuery, AllReposInOrg: mainConf.AllReposInOrg}
	got = b.OnlyOneRepoChoice()
	assert.ErrorIs(t, got, want)
}
