package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/itskreisler/rain/internal/build"
	"github.com/itskreisler/rain/internal/config"
	"github.com/itskreisler/rain/internal/doctor"
	"github.com/itskreisler/rain/internal/lint"
	"github.com/itskreisler/rain/internal/toolchain"
	"github.com/itskreisler/rain/internal/ui"
)

// newLintCmd crea "rain lint": ejecuta los analizadores estáticos opcionales
// (ktlint, detekt y pmd) sobre las fuentes del proyecto.
//
// Es el complemento de "rain check": el check compila y por tanto pilla los
// errores de tipos; el lint revisa estilo, complejidad y "code smells", que
// son otra categoría de problemas.
func newLintCmd() *cobra.Command {
	devlog("newLintCmd", "", "Crea el subcomando lint para ejecutar análisis estático")
	var (
		install   bool
		toolsFlag string
		failOn    string
		typeRes   bool
	)

	cmd := &cobra.Command{
		Use:   "lint [dir]",
		Short: "Ejecuta analizadores estáticos sobre el código",
		Long: "Ejecuta ktlint, detekt y pmd sobre las fuentes del proyecto y\n" +
			"muestra los avisos que encuentran.\n\n" +
			"Estas herramientas analizan el estilo, la complejidad y los \"code\n" +
			"smells\". NO detectan los errores de tipos (referencias sin resolver,\n" +
			"imports rotos): para eso está \"rain check\", que compila el código.\n\n" +
			"Solo se usan los analizadores que estén instalados: los que Rain\n" +
			"gestiona se descargan con \"rain lint --install\" (o\n" +
			"\"rain doctor --fix --lint\") y se guardan en ~/.rain/toolchain.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if install {
				return runLintInstall(cmd)
			}

			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			return runLint(cmd, dir, toolsFlag, failOn, typeRes)
		},
	}

	cmd.Flags().BoolVar(&install, "install", false, "descargar ktlint y detekt en ~/.rain/toolchain")
	cmd.Flags().StringVar(&toolsFlag, "tools", "", "analizadores a ejecutar (auto,ktlint,detekt,pmd)")
	cmd.Flags().StringVar(&failOn, "fail-on", "", "cuándo fallar: never|warning|error")
	cmd.Flags().BoolVar(&typeRes, "type-resolution", false, "resolver tipos (detekt): más avisos, más lento")

	return cmd
}

// runLintInstall descarga los analizadores estáticos que gestiona Rain.
func runLintInstall(cmd *cobra.Command) error {
	devlog("runLintInstall", fmt.Sprintf("cmd=%s", cmd.Name()), "Descarga e instala los analizadores estáticos ktlint y detekt")
	manager, err := toolchain.New()
	if err != nil {
		return err
	}
	manager.Out = cmd.OutOrStdout()

	ui.Info("Descargando analizadores estáticos (ktlint, detekt)...")
	if err := manager.EnsureLint(cmd.Context()); err != nil {
		return fmt.Errorf("descargando analizadores estáticos: %w", err)
	}
	ui.Success("Analizadores estáticos listos en %s", manager.ToolDir())
	return nil
}

// lintTarget es un módulo que se va a analizar.
type lintTarget struct {
	projectDir string
	sourceRoot string
	kotlin     []string
	java       []string
	classpath  []string
}

// runLint analiza el proyecto indicado.
func runLint(cmd *cobra.Command, dir, toolsFlag, failOn string, typeRes bool) error {
	devlog("runLint", fmt.Sprintf("cmd=%s, dir=%s, toolsFlag=%s, failOn=%s, typeRes=%t", cmd.Name(), dir, toolsFlag, failOn, typeRes), "Ejecuta los analizadores estáticos sobre los módulos del proyecto")
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolviendo %s: %w", dir, err)
	}
	if st, err := os.Stat(absDir); err != nil || !st.IsDir() {
		return usageErr("%s no es un directorio válido", dir)
	}

	// Configuración: la del rain.toml del proyecto (si existe) más los flags.
	cfg := config.Config{}
	if _, err := os.Stat(filepath.Join(absDir, "rain.toml")); err == nil {
		if c, err := config.Read(filepath.Join(absDir, "rain.toml")); err == nil {
			cfg = c
		} else {
			ui.VerboseLog("rain.toml ilegible: se usan los valores por defecto")
		}
	}

	lintCfg := lint.Config{
		Tools:          cfg.Lint.Tools,
		FailOn:         cfg.Lint.FailOn,
		TypeResolution: cfg.Lint.TypeResolution,
	}
	if toolsFlag != "" {
		lintCfg.Tools = splitList(toolsFlag)
	}
	if failOn != "" {
		lintCfg.FailOn = failOn
	}
	if typeRes {
		lintCfg.TypeResolution = true
	}

	tools, err := lint.Select(lintCfg)
	if err != nil {
		return usageErr("%v", err)
	}
	if len(tools) == 0 {
		ui.Warn("no hay analizadores instalados")
		fmt.Fprintln(cmd.OutOrStdout(), "Instálalos con: rain lint --install")
		return nil
	}

	targets, err := lintTargets(absDir, cfg)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		ui.Warn("no se encontraron fuentes que analizar")
		return nil
	}

	var findings []lint.Finding
	for _, target := range targets {
		for _, t := range tools {
			ui.Info("%s: %s", t.Name, target.projectDir)
			found, err := t.Run(cmd.Context(), t.Path, lint.Options{
				ProjectDir:     target.projectDir,
				SourceRoot:     target.sourceRoot,
				KotlinFiles:    target.kotlin,
				JavaFiles:      target.java,
				Classpath:      target.classpath,
				TypeResolution: lintCfg.TypeResolution,
			})
			if err != nil {
				return fmt.Errorf("%s: %w", t.Name, err)
			}
			findings = append(findings, found...)
		}
	}

	printFindings(cmd, findings)

	level := lintCfg.FailOnLevel()
	if level == "" || len(findings) == 0 {
		return nil
	}
	failing := 0
	for _, f := range findings {
		if severityRank(f.Severity) >= severityRank(level) {
			failing++
		}
	}
	if failing > 0 {
		return &exitError{
			code: 1,
			err:  fmt.Errorf("%d avisos de severidad %s o superior", failing, level),
		}
	}
	return nil
}

// lintTargets prepara los módulos a analizar: las fuentes, el directorio de
// proyecto y el classpath.
func lintTargets(absDir string, cfg config.Config) ([]lintTarget, error) {
	devlog("lintTargets", fmt.Sprintf("absDir=%s, cfg=%+v", absDir, cfg), "Prepara y obtiene la lista de módulos y fuentes a analizar")
	modules := cfg.Modules
	if len(modules) == 0 {
		modules = []config.Module{{Name: "app", Path: "app"}}
	}

	androidJar, _ := doctor.FindAndroidJar()

	var targets []lintTarget
	for _, mod := range modules {
		modDir := mod.Path
		if !filepath.IsAbs(modDir) {
			modDir = filepath.Join(absDir, modDir)
		}
		if st, err := os.Stat(modDir); err != nil || !st.IsDir() {
			// Layout plano: el propio directorio es el módulo.
			modDir = absDir
		}

		sourceRoot := build.SourceRoot(modDir)
		kotlin, java, err := lint.CollectSources(sourceRoot)
		if err != nil {
			return nil, fmt.Errorf("buscando fuentes en %s: %w", sourceRoot, err)
		}
		if len(kotlin) == 0 && len(java) == 0 {
			continue
		}

		targets = append(targets, lintTarget{
			projectDir: absDir,
			sourceRoot: sourceRoot,
			kotlin:     kotlin,
			java:       java,
			classpath:  lintClasspath(absDir, modDir, androidJar),
		})
	}
	return targets, nil
}

// lintClasspath construye el classpath para la resolución de tipos: el
// android.jar, los jars de dependencias ya preparados y las clases compiladas
// del propio módulo (si existen).
func lintClasspath(absDir, modDir, androidJar string) []string {
	devlog("lintClasspath", fmt.Sprintf("absDir=%s, modDir=%s, androidJar=%s", absDir, modDir, androidJar), "Construye el classpath para resolución de tipos en lint")
	var cp []string
	if androidJar != "" {
		cp = append(cp, androidJar)
	}

	// Las dependencias preparadas por Rain (no se descargan aquí: el lint no
	// necesita resolver dependencias, solo usarlas si ya están).
	depsDir := filepath.Join(absDir, ".rain-deps", "jars")
	if entries, err := os.ReadDir(depsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".jar") {
				cp = append(cp, filepath.Join(depsDir, e.Name()))
			}
		}
	}

	libsDir := filepath.Join(modDir, "libs")
	if entries, err := os.ReadDir(libsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".jar") {
				cp = append(cp, filepath.Join(libsDir, e.Name()))
			}
		}
	}

	objDir := filepath.Join(modDir, "build", "obj")
	if st, err := os.Stat(objDir); err == nil && st.IsDir() {
		cp = append(cp, objDir)
	}

	return cp
}

// printFindings muestra los avisos agrupados por fichero.
func printFindings(cmd *cobra.Command, findings []lint.Finding) {
	devlog("printFindings", fmt.Sprintf("cmd=%s, findingsCount=%d", cmd.Name(), len(findings)), "Muestra los hallazgos reportados por los analizadores estáticos")
	out := cmd.OutOrStdout()
	if len(findings) == 0 {
		ui.Success("sin avisos")
		return
	}

	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Column < findings[j].Column
	})

	current := ""
	for _, f := range findings {
		if f.File != current {
			current = f.File
			fmt.Fprintf(out, "\n%s\n", current)
		}
		rule := f.Rule
		if rule != "" {
			rule = " (" + rule + ")"
		}
		fmt.Fprintf(out, "  %d:%d  %s%s\n", f.Line, f.Column, f.Message, rule)
	}

	byTool := map[string]int{}
	for _, f := range findings {
		byTool[f.Tool]++
	}
	tools := make([]string, 0, len(byTool))
	for t := range byTool {
		tools = append(tools, t)
	}
	sort.Strings(tools)

	parts := make([]string, 0, len(tools))
	for _, t := range tools {
		parts = append(parts, fmt.Sprintf("%s: %d", t, byTool[t]))
	}
	fmt.Fprintf(out, "\n%d avisos (%s)\n", len(findings), strings.Join(parts, ", "))
}

// severityRank ordena las severidades para poder comparar.
func severityRank(s lint.Severity) int {
	devlog("severityRank", fmt.Sprintf("s=%v", s), "Devuelve la importancia numérica de un nivel de severidad")
	switch s {
	case lint.SeverityError:
		return 2
	case lint.SeverityWarning:
		return 1
	default:
		return 0
	}
}

// splitList parte una lista separada por comas.
func splitList(value string) []string {
	devlog("splitList", fmt.Sprintf("value=%s", value), "Divide una cadena separada por comas en una lista de elementos")
	var out []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
