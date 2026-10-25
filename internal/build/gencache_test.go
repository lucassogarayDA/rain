package build

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// La caché del Java generado es la que evita repetir javac (varios segundos
// con decenas de R.java) en cada build en el que solo cambia el código.
func TestCompileGeneratedJavaReutilizaClases(t *testing.T) {
	if _, err := exec.LookPath("javac"); err != nil {
		t.Skip("javac no está disponible")
	}

	buildDir := t.TempDir()
	genDir := filepath.Join(buildDir, "gen", "demo")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := filepath.Join(genDir, "R.java")
	writeSource := func(value string) {
		t.Helper()
		body := "package demo;\npublic final class R { public static final int x = " + value + "; }\n"
		if err := os.WriteFile(src, []byte(body), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	writeSource("1")

	cfg := Config{BuildDir: buildDir}
	objDir := filepath.Join(buildDir, "obj")
	if err := os.MkdirAll(objDir, 0o755); err != nil {
		t.Fatalf("mkdir obj: %v", err)
	}

	if err := compileGeneratedJava(cfg, objDir, []string{src}); err != nil {
		t.Fatalf("primera compilación: %v", err)
	}
	classPath := filepath.Join(objDir, "demo", "R.class")
	if _, err := os.Stat(classPath); err != nil {
		t.Fatalf("no se copió la clase a objDir: %v", err)
	}

	// Sin javac en el PATH, la segunda llamada solo puede funcionar si
	// reutiliza la caché en vez de volver a compilar.
	t.Setenv("PATH", "")
	if err := compileGeneratedJava(cfg, objDir, []string{src}); err != nil {
		t.Fatalf("se esperaba reutilizar la caché sin javac: %v", err)
	}

	// Cambiar el contenido del Java generado debe invalidar la caché.
	writeSource("2")
	if err := compileGeneratedJava(cfg, objDir, []string{src}); err == nil {
		t.Fatal("un cambio en el Java generado debe forzar una recompilación")
	}
}
