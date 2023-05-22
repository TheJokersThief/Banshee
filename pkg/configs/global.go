// Config for the CLI globally
package configs

type Configs interface {
	GlobalConfig | MigrationConfig
}

type GlobalConfig struct {
	Github GithubConfig `fig:"github"`

	Options OptionsConfig `fig:"options"`

	Defaults DefaultsConfig `fig:"defaults"`
}

type GithubConfig struct {
	UseGithubApp bool `fig:"use_github_app"`

	Token string `fig:"token"`

	AppID             int64  `fig:"app_id"`
	AppInstallationID int64  `fig:"app_installation_id"`
	AppPrivateKeyPath string `fig:"app_private_key_filepath"`
}

type OptionsConfig struct {
	LogLevel                         string   `fig:"log_level" default:"info"`
	AssignCodeReviewerIfNoneAssigned bool     `fig:"assign_code_reviewer_if_none_assigned"`
	ShowGitOutput                    bool     `fig:"show_git_output"`
	IgnoreDirectories                []string `fig:"ignore_directories"`

	CacheRepos struct {
		Enabled   bool   `fig:"enabled"`
		Directory string `fig:"directory"`
	} `fig:"cache_repos"`

	SaveProgress struct {
		Enabled   bool   `fig:"enabled"`
		Directory string `fig:"directory"`
		Batch     int64  `fig:"batch"`
	} `fig:"save_progress"`

	Merges Merges `fig:"merging"`
}

type DefaultsConfig struct {
	GitEmail string `fig:"git_email"`
	GitName  string `fig:"git_name"`

	Organisation string `fig:"organisation"`

	CodeReviewer string `fig:"code_reviewer"`
}

type Merges struct {
	Strategy    string `fig:"strategy" default:"merge"`
	AppendTitle string `fig:"append_title" default:""`
}
