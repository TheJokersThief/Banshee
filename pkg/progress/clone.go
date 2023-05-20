package progress

// Returns a list of repos that haven't been cloned yet
func (p *Progress) GetReposNotCloned() []string {
	reposForCloning := []string{}
	for repo, progress := range p.Config.Repos {
		if !progress.Cloned {
			reposForCloning = append(reposForCloning, repo)
		}
	}

	return reposForCloning
}

func (p *Progress) MarkCloned(repo string) error {
	p.Config.Repos[repo].Cloned = true
	return p.writeProgress()
}
