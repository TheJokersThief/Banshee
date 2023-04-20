// Config for the CLI globally
package configs

type Configs interface {
	GlobalConfig | MigrationConfig
}

type GlobalConfig struct {
	Github struct {
		UseGithubApp bool `fig:"use_github_app"`

		Token string `fig:"token"`

		AppID             int64  `fig:"app_id"`
		AppInstallationID int64  `fig:"app_installation_id"`
		AppPrivateKeyPath string `fig:"app_private_key_filepath"`
	} `fig:"github"`

	Options struct {
		AssignCodeReviewerIfNoneAssigned bool `fig:"assign_code_reviewer_if_none_assigned"`
	} `fig:"options"`

	Defaults struct {
		GitEmail string `fig:"git_email"`
		GitName  string `fig:"git_name"`

		Organisation string `fig:"organisation"`

		CodeReviewer string `fig:"code_reviewer"`
	} `fig:"defaults"`
}
