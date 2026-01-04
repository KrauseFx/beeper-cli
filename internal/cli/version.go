package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is the current CLI version.
const Version = "0.1.0"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println(Version)
		},
	}
}
