package main

import (
	"fmt"
	"os"
	"path"

	"github.com/alecthomas/kong"
	"github.com/charmbracelet/lipgloss"
	"github.com/kkyr/fig"
	"github.com/thejokersthief/banshee/pkg/configs"
	"github.com/thejokersthief/banshee/pkg/core"
)

var Version = "development"
var GitCommitSHA = "XXXXXX"

var FatalErrorStyling = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#CD4B41")).
	BorderStyle(lipgloss.DoubleBorder()).
	BorderForeground(lipgloss.Color("#CD4B41")).
	BorderTop(true).BorderBottom(true).
	PaddingTop(1).PaddingBottom(1).PaddingLeft(5).PaddingRight(5)

var CLI struct {
	Version struct{} `cmd:"" help:"Print banshee CLI version"`
	Migrate struct {
		MigrationFile string `arg:"" name:"path" help:"Path to migration file." type:"path"`
	} `cmd:"" help:"Run a migration"`

	List struct {
		MigrationFile string `arg:"" name:"path" help:"Path to migration file." type:"path"`
		State         string `name:"state" help:"State of PRs to show (open, closed, all)" default:"all"`
		Format        string `name:"format" help:"Format for output (json, summary)" default:"summary"`
	} `cmd:"" help:"List PRs associated with a migration"`

	ConfigFile string `name:"config" short:"c" help:"Path to global CLI config" type:"path" default:"./config.yaml"`
}

func main() {
	ctx := kong.Parse(&CLI)

	if ctx.Command() == "version" {
		fmt.Println("version:", Version, "| commit:", GitCommitSHA)
		os.Exit(0)
	}

	var globalConfig configs.GlobalConfig
	globalConfig = parseConfig(globalConfig, CLI.ConfigFile, "APP")

	switch ctx.Command() {
	case "migrate <path>":
		banshee := createBanshee(globalConfig, CLI.Migrate.MigrationFile)
		migrationErr := banshee.Migrate()
		handleErr(migrationErr)
	case "list <path>":
		banshee := createBanshee(globalConfig, CLI.List.MigrationFile)
		listErr := banshee.List(CLI.List.State, CLI.List.Format)
		handleErr(listErr)
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

func createBanshee(globalConfig configs.GlobalConfig, migrationConfigPath string) *core.Banshee {
	var migrationConfig configs.MigrationConfig
	migrationConfig = parseConfig(migrationConfig, migrationConfigPath, "APP")
	banshee, initErr := core.NewBanshee(globalConfig, migrationConfig)
	handleErr(initErr)

	return banshee
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

func handleErr(err error) {
	if err != nil {
		printFatalError(err)
	}
}
