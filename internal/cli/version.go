package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/xabinapal/patrol/internal/version"
)

// newVersionCmd creates the version command.
func (cli *CLI) newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print Patrol version information",
		Run: func(cmd *cobra.Command, args []string) {
			info := version.Get()
			fmt.Println(info.String())
		},
	}
}
