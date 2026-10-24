package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckCommandTieneElFlagFast(t *testing.T) {
	out, err := runCmd(t, "check", "--help")
	if err != nil {
		t.Fatalf("check --help: %v", err)
	}
	for _, want := range []string{"--fast", "--android-jar", "unresolved reference"} {
		if !strings.Contains(out, want) {
			t.Errorf("la ayuda de check no menciona %q:\n%s", want, out)
		}
	}
}

func TestCheckHelpAvisaDeQueNoDetectaErroresDeTipos(t *testing.T) {
	out, err := runCmd(t, "check", "--help")
	if err != nil {
		t.Fatalf("check --help: %v", err)
	}
	// El texto debe dejar claro que el check compila: es lo que detecta
	// los errores de tipos.
	if !strings.Contains(out, "compila") {
		t.Errorf("la ayuda de check debería explicar que compila:\n%s", out)
	}
}

func TestLintCommandAyuda(t *testing.T) {
	out, err := runCmd(t, "check", "--help")
	if err != nil {
		t.Fatalf("check --help: %v", err)
	}
	if !strings.Contains(out, "--fast") {
		t.Errorf("check --help debería mencionar --fast:\n%s", out)
	}

	out, err = runCmd(t, "lint", "--help")
	if err != nil {
		t.Fatalf("lint --help: %v", err)
	}
	for _, want := range []string{"ktlint", "detekt", "--install", "--fail-on", "--type-resolution"} {
		if !strings.Contains(out, want) {
			t.Errorf("la ayuda de lint no menciona %q:\n%s", want, out)
		}
	}
	// La ayuda debe avisar de que el lint NO detecta errores de tipos: es
	// el malentendido más habitual.
	if !strings.Contains(out, "NO detectan los errores de tipos") {
		t.Errorf("la ayuda de lint debería aclarar que no detecta errores de tipos:\n%s", out)
	}
}

func TestLintConUnAnalizadorQueNoExiste(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	_, err := runCmd(t, "lint", ".", "--tools", "noexiste")
	if err == nil {
		t.Fatal("lint con un analizador desconocido debería fallar")
	}
	if !strings.Contains(err.Error(), "noexiste") {
		t.Errorf("el error debería nombrar el analizador: %v", err)
	}
}

func TestLintEnUnDirectorioSinFuentes(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	out, err := runCmd(t, "lint", ".")
	if err != nil {
		t.Fatalf("lint en un directorio vacío: %v", err)
	}
	if !strings.Contains(out, "no se encontraron fuentes") && !strings.Contains(out, "no hay analizadores instalados") {
		t.Errorf("lint debería avisar de que no hay fuentes o no hay analizadores instalados:\n%s", out)
	}
}

func TestLintEnUnDirectorioInexistente(t *testing.T) {
	_, err := runCmd(t, "lint", filepath.Join(t.TempDir(), "no-existe"))
	if err == nil {
		t.Fatal("lint en un directorio inexistente debería fallar")
	}
}

// TestDoctorIncluyeLosAnalizadoresEstaticos comprueba que ktlint y detekt
// aparecen en el informe de doctor como comprobaciones opcionales: no
// bloquean el estado "Ready", pero el usuario los ve.
func TestDoctorIncluyeLosAnalizadoresEstaticos(t *testing.T) {
	out, err := runCmd(t, "doctor", "--json")
	if err != nil && ExitCode(err) != 3 {
		t.Fatalf("doctor --json: %v", err)
	}
	for _, want := range []string{`"name": "ktlint"`, `"name": "detekt"`} {
		if !strings.Contains(out, want) {
			t.Errorf("el informe de doctor no contiene %s:\n%s", want, out)
		}
	}
	// Son opcionales: required = false.
	for _, name := range []string{"ktlint", "detekt"} {
		idx := strings.Index(out, `"name": "`+name+`"`)
		if idx < 0 {
			continue
		}
		fragment := out[idx:min(idx+200, len(out))]
		if !strings.Contains(fragment, `"required": false`) {
			t.Errorf("%s debería ser una comprobación opcional:\n%s", name, fragment)
		}
	}
}

func TestDoctorFixTieneElFlagLint(t *testing.T) {
	out, err := runCmd(t, "doctor", "--help")
	if err != nil {
		t.Fatalf("doctor --help: %v", err)
	}
	if !strings.Contains(out, "--lint") {
		t.Errorf("doctor --help debería mencionar --lint:\n%s", out)
	}
}
