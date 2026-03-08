package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/alecthomas/kong"
	"github.com/charmbracelet/lipgloss"
	"github.com/kkyr/fig"
	"github.com/thejokersthief/banshee/v2/pkg/configs"
	"github.com/thejokersthief/banshee/v2/pkg/core"
)

var Version = "unset"
var GitCommitSHA = "XXXXXX"

var FatalErrorStyling = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#CD4B41")).
	BorderStyle(lipgloss.DoubleBorder()).
	BorderForeground(lipgloss.Color("#CD4B41")).
	BorderTop(true).BorderBottom(true).
	PaddingTop(1).PaddingBottom(1).PaddingLeft(5).PaddingRight(5)

var SuccessStyling = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#2ECC71"))

var CLI struct {
	Version struct{} `cmd:"" help:"Print the banshee version and git commit SHA."`
	Migrate struct {
		MigrationFile string `arg:"" name:"path" help:"Path to the migration config file (YAML). Defines the target repos, branch name, actions to run, and PR settings." type:"path"`
	} `cmd:"" help:"Run a migration: clone repos, apply actions, commit changes, and open pull requests."`

	List struct {
		MigrationFile string `arg:"" name:"path" help:"Path to the migration config file (YAML). Used to resolve the branch name and organisation for PR lookup." type:"path"`
		State         string `name:"state" short:"s" help:"Filter PRs by state. Valid values: open, closed, all." default:"all" enum:"open,closed,all"`
		Format        string `name:"format" short:"f" help:"Output format. Valid values: json, summary." default:"summary" enum:"json,summary"`
	} `cmd:"" help:"List all pull requests associated with a migration, with optional state filtering and output formatting."`

	Merge struct {
		MigrationFile string `arg:"" name:"path" help:"Path to the migration config file (YAML). Used to identify which PRs belong to this migration." type:"path"`
	} `cmd:"" help:"Merge all open pull requests for a migration that are not blocked by branch protections (mergeable state: clean)."`

	Clone struct {
		MigrationFile string `arg:"" name:"path" help:"Path to the migration config file (YAML). Used to determine which repos to pre-clone." type:"path"`
	} `cmd:"" help:"Pre-clone all repositories involved in a migration into the local cache directory. Requires options.cache_repos.enabled: true in the global config."`

	ConfigFile string `name:"config" short:"c" help:"Path to the global banshee config file (YAML). Controls GitHub authentication, logging, caching, and merge strategy. Defaults to ./config.yaml." type:"path" default:"./config.yaml"`
}

func main() {
	ctx := kong.Parse(
		&CLI,
		kong.Name("banshee"),
		kong.Description("Large-scale GitHub migration tool — clone repos, apply changes, and open pull requests across an entire organisation."),
		kong.UsageOnError(),
	)

	if ctx.Command() == "version" {
		fmt.Println(SuccessStyling.Render(fmt.Sprintf("banshee  version: %s  commit: %s", Version, GitCommitSHA)))
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
	case "merge <path>":
		banshee := createBanshee(globalConfig, CLI.Merge.MigrationFile)
		mergeErr := banshee.MergeApproved()
		handleErr(mergeErr)
	case "clone <path>":
		banshee := createBanshee(globalConfig, CLI.Clone.MigrationFile)
		cloneErr := banshee.Clone()
		handleErr(cloneErr)
	default:
		printFatalError(fmt.Errorf("unknown command: %q\nRun 'banshee --help' to see available commands", ctx.Command()))
	}
}

// Unmarshal a config into a datastructure we can reuse
func parseConfig[C configs.Configs](conf C, file string, envKey string) C {
	dir, base := getFilePieces(file)
	configParseError := fig.Load(&conf, fig.File(base), fig.Dirs(dir), fig.UseEnv(envKey))
	if configParseError != nil {
		printFatalError(fmt.Errorf("failed to load config file %q: %w\nCheck that the file exists and is valid YAML", file, configParseError))
	}
	return conf
}

func createBanshee(globalConfig configs.GlobalConfig, migrationConfigPath string) *core.Banshee {
	var migrationConfig configs.MigrationConfig
	migrationConfig = parseConfig(migrationConfig, migrationConfigPath, "APP")

	absPath, absErr := filepath.Abs(migrationConfigPath)
	if absErr != nil {
		printFatalError(fmt.Errorf("could not resolve migration file path %q: %w", migrationConfigPath, absErr))
	}

	globalConfig.MigrationDir = path.Dir(absPath)
	banshee, initErr := core.NewBanshee(globalConfig, migrationConfig)
	if initErr != nil {
		printFatalError(fmt.Errorf("failed to initialise banshee: %w\nCheck your global config for valid GitHub credentials and log level settings", initErr))
	}

	return banshee
}

// Break down the file pieces into the directory and filename
func getFilePieces(filepath string) (string, string) {
	return path.Dir(filepath), path.Base(filepath)
}

// Print a big red error message and exit with a non-zero status code.
func printFatalError(err error) {
	fmt.Fprintln(os.Stderr, FatalErrorStyling.Render("Error: "+err.Error()))
	os.Exit(1)
}

func handleErr(err error) {
	if err != nil {
		printFatalError(err)
	}
}
