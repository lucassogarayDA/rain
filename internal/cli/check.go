package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/itskreisler/rain/internal/doctor"
	"github.com/itskreisler/rain/internal/ui"
)

// newCheckCmd crea el subcomando "rain check": valida los tipos del proyecto
// sin generar DEX, sin empaquetar el APK y sin firmarlo.
//
// El objetivo es la respuesta rápida: "unresolved reference 'setContentTitle'"
// y compañía son errores del compilador de Kotlin, no de un linter de estilo,
// así que la única forma de detectarlos sin empaquetar es compilar. El check
// hace exactamente eso y se salta el resto del pipeline.
func newCheckCmd() *cobra.Command {
	devlog("newCheckCmd", "", "Crea el subcomando check para verificar tipos sin generar APK")
	var (
		androidJar string
		release    bool
		fast       bool
	)

	cmd := &cobra.Command{
		Use:   "check [dir]",
		Short: "Comprueba los tipos del código sin generar APK",
		Long: "Valida el código del proyecto sin generar DEX, sin empaquetar y sin\n" +
			"firmar el APK. Detecta los errores que solo vería el compilador\n" +
			"(referencias sin resolver, imports rotos, tipos incompatibles) mucho\n" +
			"antes que un build completo.\n\n" +
			"Pilla, por ejemplo, un \"unresolved reference 'setContentTitle'\": son\n" +
			"errores del compilador de Kotlin, no de un linter de estilo, así que la\n" +
			"única forma de verlos sin empaquetar es compilar.\n\n" +
			"Usa el mismo android.jar, el mismo classpath y el mismo plugin de\n" +
			"Compose que \"rain build\": si el check pasa, el código compila.\n\n" +
			"Si las fuentes no han cambiado desde la última compilación correcta no\n" +
			"se repite el trabajo.\n\n" +
			"Con --fast el chequeo es incremental: solo se recompilan los ficheros\n" +
			"modificados (y los que dependen de ellos) contra las clases ya\n" +
			"compiladas. Es mucho más rápido, pero menos exhaustivo: un error en\n" +
			"un fichero que no se recompila puede no aparecer.\n\n" +
			"Sin rain.toml también acepta el layout plano: si el directorio actual\n" +
			"tiene un AndroidManifest.xml (en la raíz o en src/main/), lo compila\n" +
			"igual que \"rain check <dir>\".",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jar := androidJar
			if jar == "" {
				if found, ok := doctor.FindAndroidJar(); ok {
					jar = found
					ui.VerboseLog("android.jar: %s", found)
				}
			}
			if jar == "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "Sugerencia:")
				fmt.Fprintln(cmd.ErrOrStderr(), "  rain doctor --fix    # descarga d8, apksigner y android.jar en ~/.rain")
				fmt.Fprintln(cmd.ErrOrStderr(), "  o usa --android-jar <ruta>")
				return usageErr("falta android.jar")
			}

			absJar, err := filepath.Abs(jar)
			if err != nil {
				return fmt.Errorf("resolviendo android.jar: %w", err)
			}

			if len(args) == 1 {
				return runBuildOrCheckDir(args[0], absJar, release, false, true, fast)
			}
			return runBuildOrCheck(absJar, release, false, true, fast)
		},
	}

	cmd.Flags().StringVar(&androidJar, "android-jar", "", "ruta a android.jar (por defecto se autodetecta)")
	cmd.Flags().BoolVarP(&release, "release", "r", false, "comprobar con la configuración de release")
	cmd.Flags().BoolVar(&fast, "fast", false, "chequeo incremental: solo recompila lo modificado (menos exhaustivo)")

	return cmd
}
