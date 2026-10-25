package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/itskreisler/rain/internal/compat"
	"github.com/itskreisler/rain/internal/doctor"
	"github.com/itskreisler/rain/internal/toolchain"
	"github.com/itskreisler/rain/internal/ui"
)

func newDoctorCmd() *cobra.Command {
	devlog("newDoctorCmd", "", "Crea el subcomando doctor para diagnosticar el entorno de desarrollo")
	var (
		profile      string
		jsonOut      bool
		printInstall bool
		fix          bool
		lintTools    bool
	)

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnostica el entorno de compilación Android",
		Long: "Comprueba Java, aapt2, d8, apksigner, android.jar y, en el perfil\n" +
			"complejo, Kotlin/Compose. Muestra los comandos necesarios para\n" +
			"completar la instalación.\n\n" +
			"Perfiles:\n" +
			"  simple   Java + recursos, sin Kotlin ni AndroidX\n" +
			"  complex  Kotlin + Compose + AndroidX (R8 y Kotlin modernos)\n\n" +
			"ktlint y detekt aparecen como opcionales: no hacen falta para\n" +
			"compilar, pero " + "\"rain lint\"" + " los usa si están instalados.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := doctor.ParseProfile(profile)
			if err != nil {
				return usageErr("%v", err)
			}

			if fix {
				if err := runDoctorFix(cmd, p); err != nil {
					return err
				}
				if lintTools {
					if err := runDoctorFixLint(cmd); err != nil {
						return err
					}
				}
			}

			report := doctor.Diagnose(p)

			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(report); err != nil {
					return fmt.Errorf("codificando JSON: %w", err)
				}
			} else {
				printDoctorReport(cmd, report, printInstall)
			}

			if !report.Ready {
				return &exitError{
					code: 3,
					err:  fmt.Errorf("faltan dependencias para el perfil %s: %s", p, strings.Join(report.Missing, ", ")),
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&profile, "profile", "simple", "perfil a comprobar: simple|complex")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "salida en JSON")
	cmd.Flags().BoolVar(&printInstall, "print-install", false, "mostrar solo los comandos de instalación")
	cmd.Flags().BoolVar(&fix, "fix", false, "descargar el toolchain propio en ~/.rain (sin root)")
	cmd.Flags().BoolVar(&lintTools, "lint", false, "con --fix, descargar también los analizadores estáticos (ktlint, detekt)")

	return cmd
}

// runDoctorFixLint descarga los analizadores estáticos opcionales.
func runDoctorFixLint(cmd *cobra.Command) error {
	devlog("runDoctorFixLint", fmt.Sprintf("cmd=%s", cmd.Name()), "Descarga los analizadores estáticos opcionales para doctor")
	manager, err := toolchain.New()
	if err != nil {
		return err
	}
	manager.Out = cmd.OutOrStdout()

	ui.Info("Descargando analizadores estáticos (ktlint, detekt)...")
	if err := manager.EnsureLint(cmd.Context()); err != nil {
		return fmt.Errorf("descargando analizadores estáticos: %w", err)
	}
	ui.Success("Analizadores estáticos listos")
	return nil
}

func runDoctorFix(cmd *cobra.Command, profile doctor.Profile) error {
	devlog("runDoctorFix", fmt.Sprintf("cmd=%s, profile=%s", cmd.Name(), profile), "Prepara y descarga el toolchain en ~/.rain")
	manager, err := toolchain.New()
	if err != nil {
		return err
	}
	manager.Out = cmd.OutOrStdout()

	ui.Info("Preparando toolchain en %s...", manager.Root)
	if err := manager.Ensure(cmd.Context(), profile == doctor.Complex); err != nil {
		return fmt.Errorf("preparando toolchain: %w", err)
	}
	ui.Success("Toolchain descargado")
	return nil
}

func printDoctorReport(cmd *cobra.Command, report doctor.Report, onlyInstall bool) {
	devlog("printDoctorReport", fmt.Sprintf("cmd=%s, report=%+v, onlyInstall=%t", cmd.Name(), report, onlyInstall), "Imprime en consola el reporte del diagnóstico de doctor")
	out := cmd.OutOrStdout()

	if !onlyInstall {
		fmt.Fprintf(out, "Rain doctor — perfil: %s\n", report.Profile)
		fmt.Fprintf(out, "Plataforma: %s (%s/%s)\n\n", report.Platform, report.OS, report.Arch)

		for _, check := range report.Checks {
			symbol := "✅"
			if !check.OK {
				if check.Required {
					symbol = "❌"
				} else {
					symbol = "⚠️ "
				}
			}

			detail := check.Version
			if detail == "" {
				detail = check.Path
			}
			if detail == "" {
				detail = check.Detail
			}
			if check.OK && check.Detail != "" {
				detail = check.Detail
			}

			fmt.Fprintf(out, "%s %-15s %s\n", symbol, check.Name, detail)
		}

		fmt.Fprintln(out)
		if len(report.Compat) > 0 {
			fmt.Fprintln(out, "Compatibilidad del toolchain:")
			for _, issue := range report.Compat {
				symbol := "ℹ️ "
				switch issue.Severity {
				case compat.Error:
					symbol = "❌"
				case compat.Warning:
					symbol = "⚠️ "
				}
				fmt.Fprintf(out, "%s %s\n", symbol, issue.Message)
			}
			fmt.Fprintln(out)
		}
		if report.Ready {
			ui.Success("Todo listo para el perfil %s", report.Profile)
			if report.Profile == doctor.Simple {
				fmt.Fprintln(out, "\nPara apps complejas (Kotlin/Compose): rain doctor --profile complex")
			}
		} else {
			fmt.Fprintf(out, "Faltan: %s\n", strings.Join(report.Missing, ", "))
		}
	}

	if len(report.Commands) > 0 && (onlyInstall || !report.Ready) {
		fmt.Fprintln(out, "\nComandos recomendados:")
		for _, command := range report.Commands {
			fmt.Fprintf(out, "  %s\n", command)
		}
	}
}
