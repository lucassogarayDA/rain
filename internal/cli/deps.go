package cli

import (
	"fmt"

	"github.com/itskreisler/rain/internal/config"
	"github.com/itskreisler/rain/internal/pom"
	"github.com/spf13/cobra"
)

func newDepsCmd() *cobra.Command {
	devlog("newDepsCmd", "", "Crea el subcomando deps para la gestión e inspección de dependencias")
	depsCmd := &cobra.Command{
		Use:   "deps",
		Short: "Comandos de gestión de dependencias",
	}

	treeCmd := &cobra.Command{
		Use:   "tree [dir]",
		Short: "Muestra el árbol de dependencias resueltas",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			cfgPath := "rain.toml"
			if dir != "." {
				cfgPath = dir + "/rain.toml"
			}

			cfg, err := config.Read(cfgPath)
			if err != nil {
				return fmt.Errorf("leyendo %s: %w", cfgPath, err)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Dependencias del proyecto %s (%s):\n", cfg.Project.Name, cfg.Project.Package)

			if len(cfg.Dependencies) == 0 {
				fmt.Fprintln(out, "  (sin dependencias declaradas)")
				return nil
			}

			for _, dep := range cfg.Dependencies {
				fmt.Fprintf(out, "├── %s\n", dep)
				resolved, err := pom.ResolveTransitive(dep, "cache")
				if err != nil {
					fmt.Fprintf(out, "│   └── [Error resolviendo transitivas: %v]\n", err)
					continue
				}
				for _, r := range resolved {
					if r.Coordinate() != dep {
						fmt.Fprintf(out, "│   └── %s\n", r.Coordinate())
					}
				}
			}

			return nil
		},
	}

	depsCmd.AddCommand(treeCmd)
	return depsCmd
}
