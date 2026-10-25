package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

func newVersionCmd(info BuildInfo) *cobra.Command {
	devlog("newVersionCmd", fmt.Sprintf("info=%+v", info), "Crea el subcomando version para mostrar datos de versión")
	return &cobra.Command{
		Use:   "version",
		Short: "Muestra la versión de Rain",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "rain %s\n", info.Version)
			fmt.Fprintf(out, "commit:   %s\n", info.Commit)
			fmt.Fprintf(out, "built:    %s\n", info.Date)
			fmt.Fprintf(out, "go:       %s\n", runtime.Version())
			fmt.Fprintf(out, "platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
			return nil
		},
	}
}
