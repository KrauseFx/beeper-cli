package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

type dbInfo struct {
	Path      string   `json:"path"`
	HasFTS    bool     `json:"hasFts"`
	ReadOnly  bool     `json:"readOnly"`
	BridgeDBs []string `json:"bridgeDbs,omitempty"`
}

func newDBCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Database helpers",
	}

	cmd.AddCommand(newDBInfoCmd(app))
	return cmd
}

func newDBInfoCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show resolved DB path and capabilities",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx := context.Background()
			store, path, err := app.openStore()
			if err != nil {
				return err
			}
			defer func() {
				_ = store.Close()
			}()

			hasFTS, err := store.HasFTS(ctx)
			if err != nil {
				return err
			}

			info := dbInfo{Path: path, HasFTS: hasFTS, ReadOnly: true}
			if bridges := store.BridgeDBs(); len(bridges) > 0 {
				info.BridgeDBs = bridges
			}
			if app.JSON {
				return writeJSON(info)
			}

			fmt.Printf("Path: %s\n", info.Path)
			fmt.Printf("FTS: %t\n", info.HasFTS)
			fmt.Printf("Read-only: %t\n", info.ReadOnly)
			if len(info.BridgeDBs) > 0 {
				fmt.Printf("Bridge DBs: %d\n", len(info.BridgeDBs))
			}
			return nil
		},
	}

	return cmd
}
