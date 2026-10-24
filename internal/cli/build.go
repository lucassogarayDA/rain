package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/itskreisler/rain/internal/aar"
	"github.com/itskreisler/rain/internal/build"
	"github.com/itskreisler/rain/internal/config"
	"github.com/itskreisler/rain/internal/doctor"
	"github.com/itskreisler/rain/internal/mavenrepo"
	"github.com/itskreisler/rain/internal/pom"
	"github.com/itskreisler/rain/internal/ui"
)

func newBuildCmd() *cobra.Command {
	devlog("newBuildCmd", "", "Crea el subcomando build para compilar y firmar un APK")
	var (
		androidJar        string
		release           bool
		allowDebugSigning bool
	)

	cmd := &cobra.Command{
		Use:   "build [dir]",
		Short: "Compila y firma un APK",
		Long: "Compila un APK. Sin argumentos usa rain.toml del directorio actual.\n" +
			"Con [dir] compila un proyecto completo o un layout plano estilo android2apk\n" +
			"(AndroidManifest.xml, java/ y res/ en la raíz).\n\n" +
			"Si se omite --android-jar, rain busca el android.jar instalado por\n" +
			"\"rain doctor --fix\" o en el SDK del sistema.",
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
				return runBuildDir(args[0], absJar, release, allowDebugSigning)
			}
			return runBuild(absJar, release, allowDebugSigning)
		},
	}

	cmd.Flags().StringVar(&androidJar, "android-jar", "", "ruta a android.jar (por defecto se autodetecta)")
	cmd.Flags().BoolVarP(&release, "release", "r", false, "compilar en modo release (R8)")
	cmd.Flags().BoolVar(&allowDebugSigning, "allow-debug-signing", false, "permitir firmar un release con la keystore de debug (con aviso)")

	return cmd
}

func runBuild(androidJar string, release, allowDebugSigning bool) error {
	devlog("runBuild", fmt.Sprintf("androidJar=%s, release=%t, allowDebugSigning=%t", androidJar, release, allowDebugSigning), "Ejecuta la compilación de un proyecto configurado con rain.toml")
	return runBuildOrCheck(androidJar, release, allowDebugSigning, false, false)
}

// runBuildOrCheck compila el proyecto. Con checkOnly solo valida los tipos
// (compile Kotlin y Java) y no genera DEX, APK ni firma. Con fast el chequeo
// es incremental: solo recompila lo que ha cambiado.
func runBuildOrCheck(androidJar string, release, allowDebugSigning, checkOnly, fast bool) error {
	devlog("runBuildOrCheck", fmt.Sprintf("androidJar=%s, release=%t, allowDebugSigning=%t, checkOnly=%t, fast=%t", androidJar, release, allowDebugSigning, checkOnly, fast), "Ejecuta el flujo de compilación o verificación de tipos para un proyecto rain")
	cfg, err := config.Read("rain.toml")
	if err != nil {
		return fmt.Errorf("no se pudo leer rain.toml: %w (usa rain init o rain build <dir>)", err)
	}

	mavenrepo.ExtraRepositories = cfg.Repositories.Extra

	var libs *aar.Set
	// Las dependencias por variante: en un release no entran las de debug
	// (debugImplementation) y sí las de release; en un debug, al revés. Es lo
	// que hace Gradle, y importa: una dependencia de depuración referencia
	// clases opcionales que R8 no encuentra y aborta la compilación.
	deps := cfg.Dependencies
	if release {
		deps = append(append([]string{}, deps...), cfg.Build.ReleaseDependencies...)
	} else {
		deps = append(append([]string{}, deps...), cfg.Build.DebugDependencies...)
	}
	if len(deps) > 0 {
		ui.Info("Resolviendo y descargando dependencias...")
		coords := deps
		if cfg.Mode == "completo" {
			coords, err = resolveTransitiveCoordinates(deps)
			if err != nil {
				return fmt.Errorf("resolviendo dependencias: %w", err)
			}
		}
		libs, err = prepareAARSet(coords, ".rain-deps")
		if err != nil {
			return fmt.Errorf("preparando dependencias: %w", err)
		}
	}

	modules := cfg.Modules
	if len(modules) == 0 {
		modules = []config.Module{{Name: "app", Path: "app"}}
	}

	// Los annotation processors (kapt) se resuelven aparte: no van en el
	// APK, solo en el classpath de la ronda de annotation processing.
	var kapt *build.KaptConfig
	if len(cfg.Kapt.Processors) > 0 {
		ui.Info("Resolviendo annotation processors (kapt)...")
		coords, err := resolveCoordinates(cfg.Kapt.Processors)
		if err != nil {
			return fmt.Errorf("resolviendo annotation processors: %w", err)
		}
		set, err := prepareAARSet(coords, ".rain-kapt")
		if err != nil {
			return fmt.Errorf("preparando annotation processors: %w", err)
		}
		kapt = &build.KaptConfig{
			ProcessorJars: set.ClassJars(),
			Options:       cfg.Kapt.Options,
		}
		ui.VerboseLog("processors de kapt: %d jars", len(kapt.ProcessorJars))
	}

	for _, mod := range modules {
		ui.Info("=== Compilando módulo: %s ===", mod.Name)
		bc := resolveModuleBuildConfig(cfg, mod.Path)
		bcfg := build.Config{
			AppDir:            mod.Path,
			BuildDir:          filepath.Join(mod.Path, "build"),
			LibsDir:           "libs",
			AndroidJar:        androidJar,
			Package:           bc.Package,
			Release:           release,
			Signing:           cfg.Signing,
			MinSdk:            bc.MinSdk,
			TargetSdk:         bc.TargetSdk,
			VersionCode:       bc.VersionCode,
			VersionName:       bc.VersionName,
			AppName:           cfg.Project.Name,
			JVMTarget:         bc.JavaTarget,
			Libs:              libs,
			AllowDebugSigning: allowDebugSigning,
			CheckOnly:         checkOnly,
			FastCheck:         fast,
			Kapt:              kapt,
			ProguardFiles:     cfg.Build.ProguardFiles,
		}
		if libs != nil {
			bcfg.DepsDir = libs.Dir
		}
		if err := build.Run(bcfg); err != nil {
			return fmt.Errorf("módulo %s: %w", mod.Name, err)
		}
	}
	return nil
}

// defaults de Rain cuando ni rain.toml ni app.properties definen un valor.
const (
	defaultMinSdk    = 21
	defaultTargetSdk = 36
	defaultVersion   = "1.0"
)

// moduleBuildSettings son los valores efectivos de build de un módulo, ya
// resueltos con la precedencia: rain.toml > app.properties del módulo > default.
type moduleBuildSettings struct {
	Package     string
	MinSdk      int
	TargetSdk   int
	VersionCode int
	VersionName string
	JavaTarget  int
}

// resolveModuleBuildConfig combina la sección [build] de rain.toml con un
// app.properties (estilo android2apk) situado en la raíz del módulo, para que
// los ejemplos planos se configuren sin rain.toml y los proyectos con rain.toml
// tengan la última palabra.
func resolveModuleBuildConfig(cfg config.Config, modulePath string) moduleBuildSettings {
	devlog("resolveModuleBuildConfig", fmt.Sprintf("cfg=%+v, modulePath=%s", cfg, modulePath), "Resuelve la configuración de compilación de un módulo según rain.toml y app.properties")
	props := readProperties(filepath.Join(modulePath, "app.properties"))
	s := moduleBuildSettings{
		Package:     cfg.Project.Package,
		MinSdk:      cfg.Build.MinSdk,
		TargetSdk:   cfg.Build.TargetSdk,
		VersionCode: cfg.Build.VersionCode,
		VersionName: cfg.Build.VersionName,
		JavaTarget:  cfg.Build.JavaTarget,
	}

	if s.Package == "" && cfg.Build.ApplicationID != "" {
		s.Package = cfg.Build.ApplicationID
	}
	if s.Package == "" {
		s.Package = props["package"]
	}

	// app.properties como fallback.
	if s.MinSdk == 0 {
		s.MinSdk = parseInt(props["minSdk"], 0)
	}
	if s.TargetSdk == 0 {
		s.TargetSdk = parseInt(props["targetSdk"], 0)
	}
	if s.VersionCode == 0 {
		s.VersionCode = parseInt(props["versionCode"], 0)
	}
	if s.VersionName == "" {
		s.VersionName = props["versionName"]
	}
	if s.JavaTarget == 0 {
		s.JavaTarget = parseInt(props["javaTarget"], 0)
	}

	// Defaults finales.
	if s.MinSdk <= 0 {
		s.MinSdk = defaultMinSdk
	}
	if s.TargetSdk <= 0 {
		s.TargetSdk = defaultTargetSdk
	}
	if s.TargetSdk < s.MinSdk {
		s.TargetSdk = s.MinSdk
	}
	if s.VersionCode <= 0 {
		s.VersionCode = 1
	}
	if s.VersionName == "" {
		s.VersionName = cfg.Project.Version
	}
	if s.VersionName == "" {
		s.VersionName = defaultVersion
	}
	return s
}

// resolveTransitiveCoordinates resuelve el cierre transitivo de una lista de
// coordenadas y devuelve la lista final, sin duplicados y con la versión más
// alta cuando hay conflicto.
func resolveTransitiveCoordinates(deps []string) ([]string, error) {
	devlog("resolveTransitiveCoordinates", fmt.Sprintf("deps=%v", deps), "Resuelve transitivamente las coordenadas de dependencias")
	return resolveCoordinates(deps)
}

// resolveCoordinates resuelve el cierre transitivo de una lista de
// coordenadas y devuelve la lista final, sin duplicados y con la versión más
// alta cuando hay conflicto.
func resolveCoordinates(deps []string) ([]string, error) {
	devlog("resolveCoordinates", fmt.Sprintf("deps=%v", deps), "Resuelve y desduplica coordenadas de dependencias")
	var coords []string
	seen := make(map[string]bool)

	for _, dep := range deps {
		resolved, err := pom.ResolveTransitive(dep, "cache")
		if err != nil {
			return nil, fmt.Errorf("resolviendo %s: %w", dep, err)
		}
		for _, d := range resolved {
			coord := d.Coordinate()
			if !seen[coord] {
				seen[coord] = true
				coords = append(coords, coord)
			}
		}
		if !seen[dep] {
			seen[dep] = true
			coords = append(coords, dep)
		}
	}
	// "Highest wins" global: cada dependencia declarada se resolvió por
	// separado, así que la unión puede traer varias versiones del mismo
	// artefacto. Dejarlas todas en el classpath rompe kotlinc/javac.
	return pom.DedupeCoordinates(coords), nil
}

// prepareAARSet descarga y prepara cada coordenada (AAR o JAR) en dir.
func prepareAARSet(coords []string, dir string) (*aar.Set, error) {
	devlog("prepareAARSet", fmt.Sprintf("coords=%v, dir=%s", coords, dir), "Descarga y prepara el conjunto de AARs y JARs de dependencias")
	// Si las dependencias ya estaban preparadas y no han cambiado, se
	// reutilizan: extraer decenas de AAR de Compose cuesta varios segundos
	// en cada build y no aporta nada si el resultado es el mismo.
	if set, ok := aar.LoadCache(dir, coords); ok {
		ui.VerboseLog("dependencias sin cambios, reutilizando %s", dir)
		return set, nil
	}

	pb := ui.NewProgressBar(len(coords), "Preparando ")
	set := &aar.Set{Dir: dir}
	client := aar.NewClient()
	failures := 0
	for i, coord := range coords {
		pb.Current = i + 1
		pb.Render()
		s, err := client.Prepare([]string{coord}, dir)
		if err != nil {
			failures++
			ui.Warn("No se pudo preparar %s: %v", coord, err)
			continue
		}
		set.Libraries = append(set.Libraries, s.Libraries...)
	}
	pb.Finish()
	if len(set.Libraries) == 0 {
		return nil, fmt.Errorf("no se pudo preparar ninguna dependencia")
	}
	if failures > 0 {
		ui.Warn("%d dependencias no se pudieron preparar", failures)
	}
	if err := aar.SaveCache(dir, set, coords); err != nil {
		ui.VerboseLog("no se pudo guardar la caché de dependencias: %v", err)
	}
	return set, nil
}

// runBuildDir compila un directorio sin rain.toml usando el layout plano de
// android2apk, o entra en el directorio si allí hay un rain.toml.
func runBuildDir(dir, androidJar string, release, allowDebugSigning bool) error {
	devlog("runBuildDir", fmt.Sprintf("dir=%s, androidJar=%s, release=%t, allowDebugSigning=%t", dir, androidJar, release, allowDebugSigning), "Compila un proyecto desde un directorio o layout plano")
	return runBuildOrCheckDir(dir, androidJar, release, allowDebugSigning, false, false)
}

// runBuildOrCheckDir hace lo que runBuildDir, pero en modo check solo valida
// los tipos del código (y con fast, solo lo modificado).
func runBuildOrCheckDir(dir, androidJar string, release, allowDebugSigning, checkOnly, fast bool) error {
	devlog("runBuildOrCheckDir", fmt.Sprintf("dir=%s, androidJar=%s, release=%t, allowDebugSigning=%t, checkOnly=%t, fast=%t", dir, androidJar, release, allowDebugSigning, checkOnly, fast), "Ejecuta la compilación o verificación para un directorio específico")
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolviendo %s: %w", dir, err)
	}
	st, err := os.Stat(absDir)
	if err != nil || !st.IsDir() {
		return usageErr("%s no es un directorio válido", dir)
	}

	if _, err := os.Stat(filepath.Join(absDir, "rain.toml")); err == nil {
		previous, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := os.Chdir(absDir); err != nil {
			return fmt.Errorf("entrando en %s: %w", absDir, err)
		}
		defer func() { _ = os.Chdir(previous) }()
		return runBuildOrCheck(androidJar, release, allowDebugSigning, checkOnly, fast)
	}

	if _, err := os.Stat(filepath.Join(absDir, "src/main/AndroidManifest.xml")); err != nil {
		if _, err := os.Stat(filepath.Join(absDir, "AndroidManifest.xml")); err != nil {
			return usageErr("%s no contiene rain.toml ni AndroidManifest.xml", dir)
		}
	}

	props := readProperties(filepath.Join(absDir, "app.properties"))
	name := props["name"]
	if name == "" {
		name = filepath.Base(absDir)
	}
	// Repos extra (opción A) también en layout plano, si hay rain.toml.
	if cfg, err := config.Read(filepath.Join(absDir, "rain.toml")); err == nil {
		mavenrepo.ExtraRepositories = cfg.Repositories.Extra
	} else {
		mavenrepo.ExtraRepositories = nil
	}
	s := resolveModuleBuildConfig(config.Config{}, absDir)
	if s.Package == "" {
		s.Package = build.PackageFromManifest(absDir)
	}
	if s.Package == "" {
		return fmt.Errorf("no se pudo leer package=\"...\" de AndroidManifest.xml")
	}

	ui.Info("=== Compilando %s (layout plano) ===", name)
	bcfg := build.Config{
		AppDir:            absDir,
		BuildDir:          filepath.Join(absDir, "build"),
		LibsDir:           filepath.Join(absDir, "libs"),
		AndroidJar:        androidJar,
		Package:           s.Package,
		Release:           release,
		MinSdk:            s.MinSdk,
		TargetSdk:         s.TargetSdk,
		VersionCode:       s.VersionCode,
		VersionName:       s.VersionName,
		JVMTarget:         s.JavaTarget,
		AppName:           name,
		AllowDebugSigning: allowDebugSigning,
		CheckOnly:         checkOnly,
		FastCheck:         fast,
	}
	return build.Run(bcfg)
}

func readProperties(path string) map[string]string {
	devlog("readProperties", fmt.Sprintf("path=%s", path), "Lee un archivo de propiedades key=value")
	props := make(map[string]string)
	data, err := os.ReadFile(path)
	if err != nil {
		return props
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		props[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return props
}

func parseInt(value string, fallback int) int {
	devlog("parseInt", fmt.Sprintf("value=%s, fallback=%d", value, fallback), "Convierte una cadena a entero con un valor de respaldo")
	number, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return number
}
