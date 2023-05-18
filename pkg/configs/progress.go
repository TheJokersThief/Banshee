package configs

type ProgressConfig struct {
	LastUpdated string                   `json:"last_updated"`
	Repos       map[string]*RepoProgress `json:"repos"` // Map of repo name (form: org/repo) to progress
}

type RepoProgress struct {
	Cloned   bool `json:"cloned"`
	Migrated bool `json:"migrated"`
}
