package progress

import "github.com/thejokersthief/banshee/v2/pkg/configs"

// GetReposNotMigrated returns a list of repos that haven't been migrated yet.
func (p *Progress) GetReposNotMigrated() []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.reposWhere(func(rp *configs.RepoProgress) bool {
		return !rp.Migrated
	})
}

func (p *Progress) MarkMigrated(repo string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Config.Repos[repo].Migrated = true
	return p.writeProgress()
}
