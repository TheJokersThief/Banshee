package core

func (b *Banshee) CreateCacheRepoIfEnabled() error {
	if b.GlobalConfig.Options.CacheRepos.Enabled {
		if cacheErr := b.createCacheRepo(b.log, b.GlobalConfig.Options.CacheRepos.Directory); cacheErr != nil {
			return cacheErr
		}
	}

	return nil
}
