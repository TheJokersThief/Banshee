// Config for the CLI globally
package configs

type GlobalConfig struct {
	GithubToken string

	UseGithubApp            bool
	GithubAppID             string
	GithubAppInstallationID string
	GithubAppPrivateKey     string

	AssignCodeReviewerIfNoneAssigned bool

	Defaults struct {
		GitEmail string
		GitName  string

		Organisation string

		CodeReviewer string
	}
}
