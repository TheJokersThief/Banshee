// Config for each code change
package configs

type Action struct {
	Directory string

	Description string            `fig:"description"`
	Action      string            `fig:"action"`
	Input       map[string]string `fig:"input"`
}

type MigrationConfig struct {
	BranchName    string   `fig:"branch_name"`
	Organisation  string   `fig:"organisation"`
	SearchQuery   string   `fig:"search_query"`
	ListOfRepos   []string `fig:"repos"`
	AllReposInOrg bool     `fig:"all_repos_in_org"`

	Actions []Action `fig:"actions"`

	PRTitle    string `fig:"pr_title"`
	PRBodyFile string `fig:"pr_body_file"`
	PRDrafts   bool   `fig:"pr_as_drafts"`
}
