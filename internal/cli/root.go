package cli

import (
	"fmt"
	"os"

	"github.com/KrauseFx/beeper-cli/internal/beeper"
	"github.com/KrauseFx/beeper-cli/internal/config"
	"github.com/spf13/cobra"
)

// App holds shared CLI configuration.
type App struct {
	DBPath      string
	JSON        bool
	NoBridge    bool
	ShowVersion bool
}

// Execute runs the CLI entrypoint.
func Execute() {
	app := &App{}
	rootCmd := newRootCmd(app)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "beeper-cli",
		Short: "Read-only CLI for local Beeper chats",
		Long:  "Beeper CLI provides read-only access to local Beeper SQLite data, including threads, messages, and search.",
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if app.ShowVersion {
				fmt.Println(Version)
				os.Exit(0)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if app.ShowVersion {
				fmt.Println(Version)
				return nil
			}
			return cmd.Help()
		},
	}

	cmd.PersistentFlags().StringVar(&app.DBPath, "db", "", "path to Beeper index.db (or set BEEPER_DB)")
	cmd.PersistentFlags().BoolVar(&app.JSON, "json", false, "output JSON")
	cmd.PersistentFlags().BoolVar(&app.NoBridge, "no-bridge", false, "disable megabridge name lookups")
	cmd.PersistentFlags().BoolVar(&app.ShowVersion, "version", false, "print version")

	cmd.AddCommand(newThreadsCmd(app))
	cmd.AddCommand(newMessagesCmd(app))
	cmd.AddCommand(newSearchCmd(app))
	cmd.AddCommand(newDBCmd(app))
	cmd.AddCommand(newVersionCmd())

	return cmd
}

func (a *App) openStore() (*beeper.Store, string, error) {
	path, err := config.ResolveDBPath(a.DBPath)
	if err != nil {
		return nil, "", err
	}
	store, err := beeper.OpenWithOptions(path, beeper.StoreOptions{
		BridgeLookup: !a.NoBridge,
	})
	if err != nil {
		return nil, "", err
	}
	return store, path, nil
}
