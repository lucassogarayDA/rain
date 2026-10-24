// Package lint ejecuta analizadores estáticos opcionales (ktlint, detekt y
// pmd) sobre las fuentes de un proyecto.
//
// Se diferencia de "rain check" en qué detecta: el check compila el código y
// por tanto encuentra los errores de tipos (referencias sin resolver, imports
// rotos), que ningún linter de estilo ve. Estas herramientas analizan el
// estilo, la complejidad y los "code smells", que son otra categoría de
// problemas.
//
// Ninguna herramienta se descarga sola: Rain usa las que ya estén instaladas
// (en el PATH o en ~/.rain/toolchain) y avisa de las que falten.
package lint

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Severity clasifica un hallazgo.
type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// Finding es un problema reportado por un analizador.
type Finding struct {
	Tool     string
	File     string
	Line     int
	Column   int
	Rule     string
	Severity Severity
	Message  string
}

// String devuelve el hallazgo en formato "archivo:línea:columna: mensaje".
func (f Finding) String() string {
	loc := f.File
	if f.Line > 0 {
		loc = fmt.Sprintf("%s:%d:%d", f.File, f.Line, f.Column)
	}
	return fmt.Sprintf("%s: %s", loc, f.Message)
}

// Options describe el proyecto que se va a analizar.
type Options struct {
	// ProjectDir es la raíz del proyecto: las rutas se muestran relativas.
	ProjectDir string
	// SourceRoot es el directorio con las fuentes (app/src/main/java).
	SourceRoot string
	// KotlinFiles y JavaFiles son las fuentes a analizar.
	KotlinFiles []string
	JavaFiles   []string
	// Classpath se usa para la resolución de tipos (detekt).
	Classpath []string
	// TypeResolution activa la resolución de tipos en detekt. Es más lento y
	// puede fallar con algunas combinaciones de versión, así que va apagada.
	TypeResolution bool
}

// Config es la sección [lint] de rain.toml.
type Config struct {
	// Tools lista los analizadores a ejecutar. Vacío o con "auto" detecta los
	// que estén instalados.
	Tools []string `toml:"tools"`
	// FailOn decide si el comando falla: "never", "warning" o "error".
	FailOn string `toml:"fail_on"`
	// TypeResolution pasa el classpath del proyecto a detekt.
	TypeResolution bool `toml:"type_resolution"`
}

// FailOnLevel traduce el valor de configuración al nivel de severidad que
// hace fallar el comando.
func (c Config) FailOnLevel() Severity {
	switch strings.ToLower(strings.TrimSpace(c.FailOn)) {
	case "never", "none", "off":
		return ""
	case "error":
		return SeverityError
	default:
		return SeverityWarning
	}
}

// Tool es un analizador estático disponible en el entorno.
type Tool struct {
	// Name es el identificador del analizador ("ktlint", "detekt", "pmd").
	Name string
	// Path es la ruta al ejecutable o al jar.
	Path string
	// Run ejecuta el analizador sobre las fuentes.
	Run func(ctx context.Context, path string, opts Options) ([]Finding, error)
}

// toolNames son los analizadores que Rain sabe ejecutar, en el orden en que
// se ejecutan.
var toolNames = []string{"ktlint", "detekt", "pmd"}

// Available devuelve los analizadores instalados en el entorno, en el orden
// canónico.
func Available() []Tool {
	var out []Tool
	for _, name := range toolNames {
		if t, ok := newTool(name); ok {
			out = append(out, t)
		}
	}
	return out
}

// Select devuelve los analizadorespedidos en cfg. Con "auto" o sin lista
// devuelve los que estén instalados. Si se pide uno que no está instalado,
// devuelve un error con la instrucción de instalarlo.
func Select(cfg Config) ([]Tool, error) {
	available := Available()

	requested := cfg.Tools
	if len(requested) == 0 {
		requested = []string{"auto"}
	}

	var out []Tool
	for _, name := range requested {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}
		if name == "auto" {
			out = append(out, available...)
			continue
		}
		found := false
		for _, t := range available {
			if t.Name == name {
				out = append(out, t)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("el analizador %q no está instalado (%s)", name, installHint(name))
		}
	}
	return out, nil
}

func installHint(name string) string {
	switch name {
	case "ktlint", "detekt":
		return "instálalo con: rain lint --install"
	case "pmd":
		return "instálalo con tu gestor de paquetes (pmd debe estar en el PATH)"
	default:
		return " Rain solo conoce ktlint, detekt y pmd"
	}
}

// newTool construye el analizador indicado si está instalado.
func newTool(name string) (Tool, bool) {
	switch name {
	case "ktlint":
		if path, ok := findKtlint(); ok {
			return Tool{Name: "ktlint", Path: path, Run: runKtlint}, true
		}
	case "detekt":
		if path, ok := findDetekt(); ok {
			return Tool{Name: "detekt", Path: path, Run: runDetekt}, true
		}
	case "pmd":
		if path, err := exec.LookPath("pmd"); err == nil {
			return Tool{Name: "pmd", Path: path, Run: runPMD}, true
		}
	}
	return Tool{}, false
}

// toolchainDir es el directorio del toolchain de Rain (~/.rain/toolchain).
func toolchainDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".rain", "toolchain")
}

func findKtlint() (string, bool) {
	if dir := toolchainDir(); dir != "" {
		jar := filepath.Join(dir, "ktlint-cli.jar")
		if fileExists(jar) {
			return jar, true
		}
	}
	if path, err := exec.LookPath("ktlint"); err == nil {
		return path, true
	}
	return "", false
}

func findDetekt() (string, bool) {
	if dir := toolchainDir(); dir != "" {
		jar := filepath.Join(dir, "detekt-cli.jar")
		if fileExists(jar) {
			return jar, true
		}
	}
	if path, err := exec.LookPath("detekt"); err == nil {
		return path, true
	}
	return "", false
}

// ktlintLine captura el formato del reporter "plain":
//
//	ruta/fichero.kt:12:5: mensaje (regla)
var ktlintLine = regexp.MustCompile(`^(.+?):(\d+):(\d+):\s+(.*?)\s*(?:\(([a-z0-9:.\-]+)\))?$`)

// runKtlint ejecuta ktlint sobre las fuentes Kotlin.
func runKtlint(ctx context.Context, path string, opts Options) ([]Finding, error) {
	if len(opts.KotlinFiles) == 0 {
		return nil, nil
	}

	toolArgs := []string{"--relative", "**/*.kt"}

	// ktlint acepta patrones como argumentos posicionales; se le pasan las
	// fuentes una a una agrupadas por directorio para no agotar la línea de
	// comandos en proyectos grandes.
	patterns := sourcePatterns(opts)
	toolArgs = append(toolArgs, patterns...)

	cmd := toolCommand(ctx, path, toolArgs)
	cmd.Dir = opts.ProjectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		// ktlint sale con 1 cuando encuentra problemas: no es un error.
		if exitCode(err) != 1 {
			return nil, fmt.Errorf("ktlint falló: %w\n%s", err, strings.TrimSpace(string(out)))
		}
	}

	return parseKtlint(string(out), opts), nil
}

// sourcePatterns agrupa las fuentes Kotlin por directorio y devuelve los
// patrones que ktlint entiende.
func sourcePatterns(opts Options) []string {
	dirs := map[string]bool{}
	for _, f := range opts.KotlinFiles {
		dirs[filepath.Dir(f)] = true
	}
	names := make([]string, 0, len(dirs))
	for d := range dirs {
		names = append(names, d)
	}
	sort.Strings(names)

	out := make([]string, 0, len(names))
	for _, d := range names {
		out = append(out, filepath.Join(d, "**", "*.kt"))
	}
	return out
}

// parseKtlint convierte la salida de ktlint en hallazgos.
func parseKtlint(output string, opts Options) []Finding {
	var findings []Finding
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		m := ktlintLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		findings = append(findings, Finding{
			Tool:     "ktlint",
			File:     relativePath(m[1], opts.ProjectDir),
			Line:     atoiSafe(m[2]),
			Column:   atoiSafe(m[3]),
			Message:  strings.TrimSpace(m[4]),
			Rule:     m[5],
			Severity: SeverityWarning,
		})
	}
	return findings
}

// detektLine captura el formato de la salida por consola de detekt:
//
//	ruta/fichero.kt:12:5: mensaje [Regla]
var detektLine = regexp.MustCompile(`^(.+?):(\d+):(\d+):\s+(.*?)\s*\[([A-Za-z0-9]+)\]$`)

// runDetekt ejecuta detekt sobre las fuentes Kotlin.
func runDetekt(ctx context.Context, path string, opts Options) ([]Finding, error) {
	if len(opts.KotlinFiles) == 0 {
		return nil, nil
	}

	toolArgs := []string{"--input", opts.SourceRoot}

	if opts.TypeResolution && len(opts.Classpath) > 0 {
		toolArgs = append(toolArgs, "--classpath", strings.Join(opts.Classpath, ":"))
	}

	cmd := toolCommand(ctx, path, toolArgs)
	cmd.Dir = opts.ProjectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		// detekt sale con 2 cuando encuentra problemas y con 1 si el
		// análisis falla de verdad.
		if exitCode(err) != 2 {
			return nil, fmt.Errorf("detekt falló: %w\n%s", err, strings.TrimSpace(string(out)))
		}
	}

	return parseDetekt(string(out), opts), nil
}

// parseDetekt convierte la salida de detekt en hallazgos.
func parseDetekt(output string, opts Options) []Finding {
	var findings []Finding
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		m := detektLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		findings = append(findings, Finding{
			Tool:     "detekt",
			File:     relativePath(m[1], opts.ProjectDir),
			Line:     atoiSafe(m[2]),
			Column:   atoiSafe(m[3]),
			Message:  strings.TrimSpace(m[4]),
			Rule:     m[5],
			Severity: SeverityWarning,
		})
	}
	return findings
}

// pmdLine captura el formato "text" de PMD:
//
//	ruta/fichero.java:12:	Regla: mensaje
var pmdLine = regexp.MustCompile(`^(.+?):(\d+):\s*(.+?):\s*(.*)$`)

// runPMD ejecuta PMD sobre las fuentes Java y Kotlin del proyecto.
func runPMD(ctx context.Context, path string, opts Options) ([]Finding, error) {
	if len(opts.JavaFiles) == 0 && len(opts.KotlinFiles) == 0 {
		return nil, nil
	}

	ruleset := "rulesets/java/quickstart.xml"
	if len(opts.JavaFiles) == 0 {
		ruleset = "rulesets/kotlin/quickstart.xml"
	}

	args := []string{"check", "-d", opts.SourceRoot, "-R", ruleset, "-f", "text", "--no-cache"}
	if len(opts.Classpath) > 0 {
		args = append(args, "--auxclasspath", strings.Join(opts.Classpath, ":"))
	}

	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = opts.ProjectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exitCode(err) != 4 {
			return nil, fmt.Errorf("pmd falló: %w\n%s", err, strings.TrimSpace(string(out)))
		}
	}

	return parsePMD(string(out), opts), nil
}

// parsePMD convierte la salida de PMD en hallazgos.
func parsePMD(output string, opts Options) []Finding {
	var findings []Finding
	scanner := bufio.NewScanner(strings.NewReader(output))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			continue
		}
		m := pmdLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		findings = append(findings, Finding{
			Tool:     "pmd",
			File:     relativePath(m[1], opts.ProjectDir),
			Line:     atoiSafe(m[2]),
			Rule:     strings.TrimSpace(m[3]),
			Message:  strings.TrimSpace(m[4]),
			Severity: SeverityWarning,
		})
	}
	return findings
}

// CollectSources devuelve las fuentes Kotlin y Java que hay bajo root.
func CollectSources(root string) (kotlin []string, java []string, err error) {
	if root == "" {
		return nil, nil, nil
	}
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		switch filepath.Ext(path) {
		case ".kt":
			kotlin = append(kotlin, path)
		case ".java":
			java = append(java, path)
		}
		return nil
	})
	if err != nil && os.IsNotExist(err) {
		return nil, nil, nil
	}
	return kotlin, java, err
}

// toolCommand construye el comando de un analizador. Los jars se ejecutan con
// la JVM del sistema; los binarios nativos se ejecutan directamente.
func toolCommand(ctx context.Context, path string, args []string) *exec.Cmd {
	if strings.HasSuffix(path, ".jar") {
		return exec.CommandContext(ctx, "java", append([]string{"-jar", path}, args...)...)
	}
	return exec.CommandContext(ctx, path, args...)
}

// relativePath muestra la ruta relativa a la raíz del proyecto para que los
// informes sean legibles y reproducibles.
func relativePath(path, projectDir string) string {
	if projectDir == "" {
		return path
	}
	rel, err := filepath.Rel(projectDir, path)
	if err != nil {
		return path
	}
	// Una ruta relativa que sale del proyecto ("../otro/fichero.kt") es peor
	// que la ruta absoluta: se muestra esta última.
	if strings.HasPrefix(rel, "..") {
		return path
	}
	return rel
}

// exitCode devuelve el código de salida del comando, o -1 si el error no
// fue un fallo del proceso (por ejemplo, si no se pudo ejecutarlo).
func exitCode(err error) int {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	return -1
}

func atoiSafe(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}
