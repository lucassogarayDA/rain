package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseKtlint(t *testing.T) {
	output := `app/src/main/java/com/x/Main.kt:12:5: Missing trailing comma before ")" (standard:trailing-comma-on-call-site)
app/src/main/java/com/x/Main.kt:20:1: File must end with a newline (\n) (standard:final-newline)
`
	opts := Options{ProjectDir: "/proj"}
	findings := parseKtlint(output, opts)
	if len(findings) != 2 {
		t.Fatalf("hallazgos = %d, quiero 2: %+v", len(findings), findings)
	}

	first := findings[0]
	if first.Tool != "ktlint" {
		t.Errorf("herramienta = %q, quiero ktlint", first.Tool)
	}
	if first.File != "app/src/main/java/com/x/Main.kt" {
		t.Errorf("fichero = %q", first.File)
	}
	if first.Line != 12 || first.Column != 5 {
		t.Errorf("posición = %d:%d, quiero 12:5", first.Line, first.Column)
	}
	if first.Rule != "standard:trailing-comma-on-call-site" {
		t.Errorf("regla = %q", first.Rule)
	}
	if first.Severity != SeverityWarning {
		t.Errorf("severidad = %q, quiero warning", first.Severity)
	}
	if first.Message == "" {
		t.Error("mensaje vacío")
	}
}

func TestParseKtlintIgnoraLoQueNoEsUnHallazgo(t *testing.T) {
	output := "Procesando 3 ficheros\n\n"
	if findings := parseKtlint(output, Options{}); len(findings) != 0 {
		t.Errorf("hallazgos = %d, quiero 0: %+v", len(findings), findings)
	}
}

func TestParseDetekt(t *testing.T) {
	output := `app/src/main/java/com/x/Main.kt:29:47: This expression contains a magic number. [MagicNumber]
app/src/main/java/com/x/Other.kt:3:1: Too many functions. [ComplexMethod]
`
	findings := parseDetekt(output, Options{ProjectDir: "/proj"})
	if len(findings) != 2 {
		t.Fatalf("hallazgos = %d, quiero 2: %+v", len(findings), findings)
	}
	if findings[0].Tool != "detekt" {
		t.Errorf("herramienta = %q, quiero detekt", findings[0].Tool)
	}
	if findings[0].Rule != "MagicNumber" {
		t.Errorf("regla = %q, quiero MagicNumber", findings[0].Rule)
	}
	if findings[0].Line != 29 || findings[0].Column != 47 {
		t.Errorf("posición = %d:%d, quiero 29:47", findings[0].Line, findings[0].Column)
	}
	if findings[1].File != "app/src/main/java/com/x/Other.kt" {
		t.Errorf("fichero = %q", findings[1].File)
	}
}

func TestParsePMD(t *testing.T) {
	output := `app/src/main/java/com/x/Main.java:12:	UnusedImports: Avoid unused imports such as 'java.util.List'
`
	findings := parsePMD(output, Options{ProjectDir: "/proj"})
	if len(findings) != 1 {
		t.Fatalf("hallazgos = %d, quiero 1: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.Tool != "pmd" {
		t.Errorf("herramienta = %q, quiero pmd", f.Tool)
	}
	if f.Rule != "UnusedImports" {
		t.Errorf("regla = %q, quiero UnusedImports", f.Rule)
	}
	if f.Line != 12 {
		t.Errorf("línea = %d, quiero 12", f.Line)
	}
	if f.Message == "" {
		t.Error("mensaje vacío")
	}
}

func TestFailOnLevel(t *testing.T) {
	cases := map[string]Severity{
		"":            SeverityWarning,
		"never":       "",
		"none":        "",
		"off":         "",
		"error":       SeverityError,
		"ERROR":       SeverityError,
		"warning":     SeverityWarning,
		"  warning  ": SeverityWarning,
	}
	for value, want := range cases {
		got := Config{FailOn: value}.FailOnLevel()
		if got != want {
			t.Errorf("FailOnLevel(%q) = %q, quiero %q", value, got, want)
		}
	}
}

func TestSelectAutoUsaLoInstalado(t *testing.T) {
	tools, err := Select(Config{})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	// "auto" no puede fallar: devuelve lo que haya instalado (puede ser
	// una lista vacía si el entorno no tiene ningún analizador).
	for _, tool := range tools {
		if tool.Name != "ktlint" && tool.Name != "detekt" && tool.Name != "pmd" {
			t.Errorf("analizador desconocido: %q", tool.Name)
		}
		if tool.Run == nil {
			t.Errorf("%s sin función de ejecución", tool.Name)
		}
	}
}

func TestSelectPideUnAnalizadorQueNoExiste(t *testing.T) {
	_, err := Select(Config{Tools: []string{"noexiste"}})
	if err == nil {
		t.Fatal("Select con un analizador desconocido debería fallar")
	}
	if !strings.Contains(err.Error(), "noexiste") {
		t.Errorf("el error debería nombrar el analizador: %v", err)
	}
}

func TestCollectSources(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Main.kt"), "fun main() {}")
	writeFile(t, filepath.Join(dir, "sub", "Other.kt"), "class Other")
	writeFile(t, filepath.Join(dir, "sub", "Legacy.java"), "class Legacy {}")
	writeFile(t, filepath.Join(dir, "notes.txt"), "ignorar")

	kotlin, java, err := CollectSources(dir)
	if err != nil {
		t.Fatalf("CollectSources: %v", err)
	}
	if len(kotlin) != 2 {
		t.Errorf("kotlin = %d, quiero 2: %v", len(kotlin), kotlin)
	}
	if len(java) != 1 {
		t.Errorf("java = %d, quiero 1: %v", len(java), java)
	}
}

func TestCollectSourcesConDirectorioInexistente(t *testing.T) {
	kotlin, java, err := CollectSources(filepath.Join(t.TempDir(), "nada"))
	if err != nil {
		t.Fatalf("CollectSources: %v", err)
	}
	if len(kotlin) != 0 || len(java) != 0 {
		t.Errorf("quiero 0 fuentes, tengo %d kotlin y %d java", len(kotlin), len(java))
	}
}

func TestSourcePatternsAgrupaPorDirectorio(t *testing.T) {
	opts := Options{
		KotlinFiles: []string{
			"/p/app/src/main/java/com/x/A.kt",
			"/p/app/src/main/java/com/x/B.kt",
			"/p/app/src/main/java/com/y/C.kt",
		},
	}
	patterns := sourcePatterns(opts)
	if len(patterns) != 2 {
		t.Fatalf("patrones = %d, quiero 2: %v", len(patterns), patterns)
	}
	if patterns[0] >= patterns[1] {
		t.Errorf("los patrones deberían ir ordenados: %v", patterns)
	}
}

func TestRelativePath(t *testing.T) {
	if got := relativePath("/proj/app/src/Main.kt", "/proj"); got != "app/src/Main.kt" {
		t.Errorf("relativePath = %q", got)
	}
	if got := relativePath("/otro/Main.kt", "/proj"); got != "/otro/Main.kt" {
		t.Errorf("relativePath fuera del proyecto = %q", got)
	}
	if got := relativePath("/proj/Main.kt", ""); got != "/proj/Main.kt" {
		t.Errorf("relativePath sin proyecto = %q", got)
	}
}

func TestAtoiSafe(t *testing.T) {
	if got := atoiSafe("42"); got != 42 {
		t.Errorf("atoiSafe(42) = %d", got)
	}
	if got := atoiSafe("x"); got != 0 {
		t.Errorf("atoiSafe(x) = %d, quiero 0", got)
	}
	if got := atoiSafe(""); got != 0 {
		t.Errorf("atoiSafe(\"\") = %d, quiero 0", got)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creando %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("escribiendo %s: %v", path, err)
	}
}
