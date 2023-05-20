package progress

// Returns a list of repos that haven't been migrated yet
func (p *Progress) GetReposNotMigrated() []string {
	reposForMigrating := []string{}
	for repo, progress := range p.Config.Repos {
		if !progress.Migrated {
			reposForMigrating = append(reposForMigrating, repo)
		}
	}

	return reposForMigrating
}

func (p *Progress) MarkMigrated(repo string) error {
	p.Config.Repos[repo].Migrated = true
	return p.writeProgress()
}
