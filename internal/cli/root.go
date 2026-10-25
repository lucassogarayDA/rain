package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/itskreisler/rain/internal/toolchain"
	"github.com/itskreisler/rain/internal/ui"
)

// BuildInfo contiene los datos de versión inyectados en tiempo de compilación.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

// exitError permite asociar un código de salida a un error.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

// ExitCode devuelve el código de salida asociado a err (1 por defecto).
func ExitCode(err error) int {
	devlog("ExitCode", fmt.Sprintf("err=%v", err), "Devuelve el código de salida asociado a un error")
	var e *exitError
	if errors.As(err, &e) {
		return e.code
	}
	return 1
}

func usageErr(format string, a ...any) error {
	devlog("usageErr", fmt.Sprintf("format=%s, a=%v", format, a), "Crea un error de uso con código de salida 2")
	return &exitError{code: 2, err: fmt.Errorf(format, a...)}
}

// Execute construye y ejecuta el CLI. Los errores se imprimen una sola vez y
// terminan el proceso con el código correspondiente.
func Execute(info BuildInfo) {
	devlog("Execute", fmt.Sprintf("info=%+v", info), "Ejecuta el CLI de Rain con la información de compilación proporcionada")
	root := NewRootCmd(info)
	root.SetArgs(normalizeLegacyArgs(os.Args[1:]))
	if err := root.Execute(); err != nil {
		ui.Error("%v", err)
		os.Exit(ExitCode(err))
	}
}

// NewRootCmd construye el árbol de comandos. Se exporta para poder testearlo
// con cmd.SetArgs(...) sin ejecutar os.Exit.
func NewRootCmd(info BuildInfo) *cobra.Command {
	devlog("NewRootCmd", fmt.Sprintf("info=%+v", info), "Construye el comando raíz y define las opciones globales y subcomandos")
	opts := &globalOptions{}

	root := &cobra.Command{
		Use:   "rain",
		Short: "Build system ligero para Android",
		Long: "Rain compila APKs Android sin Gradle ni Android Studio.\n" +
			"Pensado para Termux y contenedores Alpine, sin daemon y sin JVM persistente.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       info.Version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			ui.Verbose = opts.verbose
			ui.Quiet = opts.quiet
			ui.NoColor = opts.noColor || os.Getenv("NO_COLOR") != ""
			DevlogEnabled = opts.devlog
			DevlogOut = cmd.OutOrStdout()
			ui.SetOutput(cmd.OutOrStdout(), cmd.ErrOrStderr())
			addManagedBinToPath()
			refreshManagedWrappers()
			return nil
		},
	}
	root.SetVersionTemplate("rain {{.Version}}\n")

	root.PersistentFlags().BoolVarP(&opts.verbose, "verbose", "v", false, "mostrar detalles de ejecución")
	root.PersistentFlags().BoolVarP(&opts.quiet, "quiet", "q", false, "solo mostrar errores")
	root.PersistentFlags().BoolVar(&opts.noColor, "no-color", false, "desactivar colores ANSI")
	root.PersistentFlags().BoolVar(&opts.devlog, "devlog", false, "mostrar logs de desarrollo de funciones CLI")

	root.AddCommand(
		newInitCmd(),
		newBuildCmd(),
		newCheckCmd(),
		newLintCmd(),
		newDoctorCmd(),
		newCleanCmd(),
		newDepsCmd(),
		newGradleCmd(),
		newVersionCmd(info),
	)

	return root
}

type globalOptions struct {
	verbose bool
	quiet   bool
	noColor bool
	devlog  bool
}

// normalizeLegacyArgs acepta la forma antigua de un guion (-release,
// -android-jar) además de la forma POSIX (--release, --android-jar).
func normalizeLegacyArgs(args []string) []string {
	devlog("normalizeLegacyArgs", fmt.Sprintf("args=%v", args), "Normaliza los argumentos de la línea de comandos de un guion a dos guiones")
	known := map[string]bool{
		"android-jar":     true,
		"release":         true,
		"verbose":         true,
		"quiet":           true,
		"no-color":        true,
		"devlog":          true,
		"mode":            true,
		"name":            true,
		"package":         true,
		"version":         true,
		"non-interactive": true,
		"force":           true,
		"profile":         true,
		"json":            true,
		"print-install":   true,
		"fix":             true,
		"module":          true,
		"dry-run":         true,
		"offline":         true,
	}

	out := make([]string, len(args))
	for i, arg := range args {
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && len(arg) > 2 {
			name := strings.TrimPrefix(arg, "-")
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				name = name[:eq]
			}
			if known[name] {
				arg = "-" + arg
			}
		}
		out[i] = arg
	}
	return out
}

// addManagedBinToPath antepone ~/.rain/bin y SDKs de Android al PATH para que los
// wrappers y herramientas de compilación estén disponibles sin tocar el shell.
func addManagedBinToPath() {
	devlog("addManagedBinToPath", "", "Antepone ~/.rain/bin y directorios de SDK al PATH de ejecución")
	home, err := os.UserHomeDir()
	if err == nil {
		bin := filepath.Join(home, ".rain", "bin")
		inPath := false
		for _, part := range filepath.SplitList(os.Getenv("PATH")) {
			if part == bin {
				inPath = true
				break
			}
		}
		if !inPath {
			_ = os.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		}
	}

	if _, err := exec.LookPath("aapt2"); err != nil {
		sdkRoots := []string{
			os.Getenv("ANDROID_HOME"),
			os.Getenv("ANDROID_SDK_ROOT"),
			"/opt/android-sdk",
		}
		if home != "" {
			sdkRoots = append(sdkRoots, filepath.Join(home, "Android/Sdk"))
		}
		for _, root := range sdkRoots {
			if root == "" {
				continue
			}
			btDir := filepath.Join(root, "build-tools")
			entries, err := os.ReadDir(btDir)
			if err != nil {
				continue
			}
			var newest string
			for _, e := range entries {
				if e.IsDir() {
					if _, err := os.Stat(filepath.Join(btDir, e.Name(), "aapt2")); err == nil {
						newest = filepath.Join(btDir, e.Name())
					}
				}
			}
			if newest != "" {
				_ = os.Setenv("PATH", newest+string(os.PathListSeparator)+os.Getenv("PATH"))
				break
			}
		}
	}
}

// refreshManagedWrappers reescribe los lanzadores de ~/.rain/bin si su
// contenido cambió, para que una actualización de Rain aplique los nuevos
// ajustes (límite de heap de la JVM, variables de entorno) sin pedir
// "rain doctor --fix". Los errores se ignoran: son ficheros auxiliares y no
// deben impedir compilar.
func refreshManagedWrappers() {
	devlog("refreshManagedWrappers", "", "Actualiza los wrappers ejecutables administrados en ~/.rain/bin")
	manager, err := toolchain.New()
	if err != nil {
		return
	}
	_ = manager.RefreshWrappers()
}
