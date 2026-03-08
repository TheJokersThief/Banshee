package core

import (
	"errors"
)

var (
	ErrOnlyUseOne = errors.New("may only use one of `search_query`, `all_repos_in_org` or `repos`")
	ErrMustUseOne = errors.New("must specify one of `search_query`, `all_repos_in_org` or `repos`")
)

func (b *Banshee) OnlyOneRepoChoice() error {

	usingRepolist := (len(b.MigrationConfig.ListOfRepos) > 0)
	queryNotEmpty := (b.MigrationConfig.SearchQuery != "")
	useAllRepos := b.MigrationConfig.AllReposInOrg

	alreadySet := false
	for _, option := range []bool{usingRepolist, queryNotEmpty, useAllRepos} {
		if option {
			if alreadySet {
				return ErrOnlyUseOne
			}
			alreadySet = true
		}
	}

	if !usingRepolist && !queryNotEmpty && !useAllRepos {
		return ErrMustUseOne
	}

	return nil
}
