package main

import (
	"fmt"

	"github.com/kkyr/fig"
	"github.com/thejokersthief/banshee/pkg/banshee/configs"
)

func main() {
	var err error
	var globalConfig configs.GlobalConfig
	err = fig.Load(&globalConfig)
	if err != nil {
		fmt.Println(err)
	}

	var migrationConfig configs.MigrationConfig
	err = fig.Load(&migrationConfig)
	if err != nil {
		fmt.Println(err)
	}
}
