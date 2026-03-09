package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thejokersthief/banshee/v2/pkg/configs"
)

func TestOnlyOneRepoChoice(t *testing.T) {
	globalConf := configs.GlobalConfig{
		Options: configs.OptionsConfig{LogLevel: "info"},
		Github:  configs.GithubConfig{Token: "testtoken"},
	}

	tests := []struct {
		name       string
		migConf    configs.MigrationConfig
		wantErr    error
	}{
		{
			name:    "none set returns ErrMustUseOne",
			migConf: configs.MigrationConfig{},
			wantErr: ErrMustUseOne,
		},
		{
			name: "search_query and list_of_repos returns ErrOnlyUseOne",
			migConf: configs.MigrationConfig{
				SearchQuery: "Test query",
				ListOfRepos: []string{"repo_name"},
			},
			wantErr: ErrOnlyUseOne,
		},
		{
			name: "list_of_repos and all_repos_in_org returns ErrOnlyUseOne",
			migConf: configs.MigrationConfig{
				ListOfRepos:   []string{"repo_name"},
				AllReposInOrg: true,
			},
			wantErr: ErrOnlyUseOne,
		},
		{
			name: "search_query and all_repos_in_org returns ErrOnlyUseOne",
			migConf: configs.MigrationConfig{
				SearchQuery:   "Test query",
				AllReposInOrg: true,
			},
			wantErr: ErrOnlyUseOne,
		},
		{
			name: "only search_query returns no error",
			migConf: configs.MigrationConfig{
				SearchQuery: "Test query",
			},
			wantErr: nil,
		},
		{
			name: "only list_of_repos returns no error",
			migConf: configs.MigrationConfig{
				ListOfRepos: []string{"repo_name"},
			},
			wantErr: nil,
		},
		{
			name: "only all_repos_in_org returns no error",
			migConf: configs.MigrationConfig{
				AllReposInOrg: true,
			},
			wantErr: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, err := NewBanshee(context.Background(), globalConf, configs.MigrationConfig{
				// Need at least one repo option to pass NewBanshee validation;
				// we override MigrationConfig directly after construction.
				ListOfRepos: []string{"bootstrap"},
			})
			assert.NoError(t, err)

			b.MigrationConfig = &tc.migConf
			got := b.OnlyOneRepoChoice()

			if tc.wantErr != nil {
				assert.ErrorIs(t, got, tc.wantErr)
			} else {
				assert.NoError(t, got)
			}
		})
	}
}
