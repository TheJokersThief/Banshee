package core

import (
	"errors"
)

var (
	OnlyUseOneErr = errors.New("You may only use one of `search_query`, `all_repos_in_org` or `repos`")
	MustUseOneErr = errors.New("You must specify one of `search_query`, `all_repos_in_org` or `repos`")
)

func (b *Banshee) OnlyOneRepoChoice() error {

	usingRepolist := (len(b.MigrationConfig.ListOfRepos) > 0)
	queryNotEmpty := (b.MigrationConfig.SearchQuery != "")
	useAllRepos := b.MigrationConfig.AllReposInOrg

	alreadySet := false
	for _, option := range []bool{usingRepolist, queryNotEmpty, useAllRepos} {
		if option {
			if alreadySet {
				return OnlyUseOneErr
			}
			alreadySet = true
		}
	}

	if !(usingRepolist || queryNotEmpty || useAllRepos) {
		return MustUseOneErr
	}

	return nil
}
