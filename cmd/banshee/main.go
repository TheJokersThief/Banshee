package main

import (
	"fmt"
	"os"
	"path"

	"github.com/alecthomas/kong"
	"github.com/kkyr/fig"
	"github.com/thejokersthief/banshee/pkg/configs"
)

var CLI struct {
	Version struct{} `cmd:"" help:"Print banshee CLI version"`
	Migrate struct {
		MigrationFile string `arg:"" name:"path" help:"Path to migration file." type:"path"`
	} `cmd:"" help:"Run migration"`

	ConfigFile string `name:"config" short:"c" help:"Path to global CLI config" type:"path" default:"./config.yaml"`
}

func main() {
	ctx := kong.Parse(&CLI)

	var globalConfig configs.GlobalConfig
	globalConfig = parseConfig(globalConfig, CLI.ConfigFile, "APP")

	switch ctx.Command() {
	case "migrate <path>":
		fmt.Println("migrating?")
		var migrationConfig configs.MigrationConfig
		migrationConfig = parseConfig(migrationConfig, CLI.Migrate.MigrationFile, "APP")

	case "version":
		fmt.Println("versioning")
	default:
		printFatalError(fmt.Errorf(ctx.Command()))
	}

}

// Unmarshal a config into a datastructure we can reuse
func parseConfig[C configs.Configs](conf C, file string, envKey string) C {
	dir, base := getFilePieces(file)
	configParseError := fig.Load(&conf, fig.File(base), fig.Dirs(dir), fig.UseEnv(envKey))
	if configParseError != nil {
		printFatalError(configParseError)
	}
	return conf
}

// Break down the file pieces into the directory and filename
func getFilePieces(filepath string) (string, string) {
	return path.Dir(filepath), path.Base(filepath)
}

// Print a big red error message and exit
func printFatalError(err error) {
	fmt.Println(FatalErrorStyling.Render(err.Error()))
	os.Exit(1)
}
