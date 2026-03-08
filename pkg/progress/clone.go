package progress

import "github.com/thejokersthief/banshee/v2/pkg/configs"

// GetReposNotCloned returns a list of repos that haven't been cloned yet.
func (p *Progress) GetReposNotCloned() []string {
	return p.reposWhere(func(rp *configs.RepoProgress) bool {
		return !rp.Cloned
	})
}

func (p *Progress) MarkCloned(repo string) error {
	p.Config.Repos[repo].Cloned = true
	return p.writeProgress()
}
