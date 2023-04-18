// Config for the CLI globally
package configs

type GlobalConfig struct {
	GithubToken string `fig:"github_token"`

	UseGithubApp            bool   `fig:"use_github_app" default:"false"`
	GithubAppID             string `fig:"github_app_id"`
	GithubAppInstallationID string `fig:"github_app_installation_id"`
	GithubAppPrivateKeyPath string `fig:"github_app_private_key_filepath"`

	Options struct {
		AssignCodeReviewerIfNoneAssigned bool `fig:"assign_code_reviewer_if_none_assigned"`
	}

	Defaults struct {
		GitEmail string `fig:"git_email"`
		GitName  string `fig:"git_name"`

		Organisation string `fig:"organisation"`

		CodeReviewer string `fig:"code_reviewer"`
	}
}
