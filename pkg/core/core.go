package banshee

import (
	"github.com/thejokersthief/banshee/pkg/configs"
)

type Banshee struct {
	GlobalConfig    *configs.GlobalConfig
	MigrationConfig *configs.MigrationConfig
}

func NewBanshee(config configs.GlobalConfig, migConfig configs.MigrationConfig) *Banshee {
	return &Banshee{
		GlobalConfig:    &config,
		MigrationConfig: &migConfig,
	}
}
