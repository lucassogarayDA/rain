package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDeclarationsExtraeNombres(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Main.kt")
	content := `package com.x

class MiClase {
    fun miFuncion(param: String): String = param
}

object MiObjeto

interface MiInterfaz

typealias MiAlias = String

fun顶层(): Unit {}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("escribiendo %s: %v", path, err)
	}

	names := declarations(path)
	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	for _, want := range []string{"MiClase", "miFuncion", "MiObjeto", "MiInterfaz", "MiAlias"} {
		if !found[want] {
			t.Errorf("no se encontró la declaración %q: %v", want, names)
		}
	}
}

func TestDeclarationsDeUnFicheroVacio(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Vacio.kt")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("escribiendo %s: %v", path, err)
	}
	if names := declarations(path); len(names) != 0 {
		t.Errorf("declaraciones = %v, quiero ninguna", names)
	}
}

func TestMentionsIdentifierNoCasaConPrefijos(t *testing.T) {
	if mentionsIdentifier("val miFuncion = 1", "miFuncion") != true {
		t.Error("debería encontrar miFuncion")
	}
	if mentionsIdentifier("val miFuncionLarga = 1", "miFuncion") {
		t.Error("miFuncion no debería casar con miFuncionLarga")
	}
	if mentionsIdentifier("miFuncion", "miFuncion") != true {
		t.Error("debería encontrar el identificador completo")
	}
	if mentionsIdentifier("otra cosa", "miFuncion") {
		t.Error("no debería encontrar nada")
	}
	if mentionsIdentifier("obj.miFuncion()", "miFuncion") != true {
		t.Error("debería encontrar un identificador tras un punto")
	}
}

func TestSourcesStampRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := writeSourcesStamp(dir); err != nil {
		t.Fatalf("writeSourcesStamp: %v", err)
	}
	stamp, err := sourcesStamp(dir)
	if err != nil {
		t.Fatalf("sourcesStamp: %v", err)
	}
	if stamp.IsZero() {
		t.Error("la marca debería tener fecha")
	}
	if time.Since(stamp) > time.Minute {
		t.Errorf("la marca debería ser reciente: %v", stamp)
	}
}

func TestSourcesStampSinMarca(t *testing.T) {
	if _, err := sourcesStamp(t.TempDir()); err == nil {
		t.Error("sourcesStamp sin fichero debería fallar")
	}
}

func TestChangedSourcesDetectaLoModificado(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "app")
	src := filepath.Join(app, "src", "main", "java")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatalf("creando %s: %v", src, err)
	}

	old := filepath.Join(src, "Viejo.kt")
	fresh := filepath.Join(src, "Nuevo.kt")
	for path, content := range map[string]string{
		old:   "class Viejo\n",
		fresh: "class Nuevo\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("escribiendo %s: %v", path, err)
		}
	}

	cfg := Config{AppDir: app}
	stamp := time.Now().Add(-time.Hour)
	changed, dependents, err := changedSources(cfg, stamp)
	if err != nil {
		t.Fatalf("changedSources: %v", err)
	}
	if len(changed) != 2 {
		t.Errorf("modificados = %d, quiero 2: %v", len(changed), changed)
	}
	if len(dependents) != 0 {
		t.Errorf("dependientes = %d, quiero 0: %v", len(dependents), dependents)
	}
}

func TestChangedSourcesRespetaLaMarca(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "app")
	src := filepath.Join(app, "src", "main", "java")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatalf("creando %s: %v", src, err)
	}
	path := filepath.Join(src, "Main.kt")
	if err := os.WriteFile(path, []byte("class Main\n"), 0o644); err != nil {
		t.Fatalf("escribiendo %s: %v", path, err)
	}

	cfg := Config{AppDir: app}
	// Marca en el futuro: nada ha cambiado desde entonces.
	changed, _, err := changedSources(cfg, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("changedSources: %v", err)
	}
	if len(changed) != 0 {
		t.Errorf("modificados = %d, quiero 0: %v", len(changed), changed)
	}
}

func TestChangedSourcesIncluyeLosDependientes(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "app")
	src := filepath.Join(app, "src", "main", "java")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatalf("creando %s: %v", src, err)
	}

	changed := filepath.Join(src, "Cambiado.kt")
	dependent := filepath.Join(src, "Dependiente.kt")
	unrelated := filepath.Join(src, "Ajeno.kt")

	if err := os.WriteFile(changed, []byte("fun miFuncion(): Int = 1\n"), 0o644); err != nil {
		t.Fatalf("escribiendo %s: %v", changed, err)
	}
	if err := os.WriteFile(dependent, []byte("val x = miFuncion()\n"), 0o644); err != nil {
		t.Fatalf("escribiendo %s: %v", dependent, err)
	}
	if err := os.WriteFile(unrelated, []byte("val y = 42\n"), 0o644); err != nil {
		t.Fatalf("escribiendo %s: %v", unrelated, err)
	}

	// Solo "Cambiado.kt" se ha modificado: los otros dos tienen fecha vieja.
	stamp := time.Now().Add(-time.Hour)
	old := time.Now().Add(-2 * time.Hour)
	for _, path := range []string{dependent, unrelated} {
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatalf("cambiando la fecha de %s: %v", path, err)
		}
	}

	cfg := Config{AppDir: app}
	changedFiles, dependents, err := changedSources(cfg, stamp)
	if err != nil {
		t.Fatalf("changedSources: %v", err)
	}
	if len(changedFiles) != 1 || !strings.HasSuffix(changedFiles[0], "Cambiado.kt") {
		t.Errorf("modificados = %v, quiero solo Cambiado.kt", changedFiles)
	}
	if len(dependents) != 1 {
		t.Fatalf("dependientes = %d, quiero 1: %v", len(dependents), dependents)
	}
	if !strings.HasSuffix(dependents[0], "Dependiente.kt") {
		t.Errorf("dependiente = %q, quiero Dependiente.kt", dependents[0])
	}
}

func TestHasClassFiles(t *testing.T) {
	dir := t.TempDir()
	if hasClassFiles(dir) {
		t.Error("un directorio vacío no tiene clases")
	}
	classDir := filepath.Join(dir, "com", "x")
	if err := os.MkdirAll(classDir, 0o755); err != nil {
		t.Fatalf("creando %s: %v", classDir, err)
	}
	if err := os.WriteFile(filepath.Join(classDir, "Main.class"), []byte("x"), 0o644); err != nil {
		t.Fatalf("escribiendo clase: %v", err)
	}
	if !hasClassFiles(dir) {
		t.Error("debería detectar la clase compilada")
	}
}

func TestDescribeFastCheck(t *testing.T) {
	if got := describeFastCheck(nil, nil); got != "0 fichero(s) modificado(s)" {
		t.Errorf("describeFastCheck = %q", got)
	}
	if got := describeFastCheck([]string{"a"}, []string{"b", "c"}); !strings.Contains(got, "2 dependiente(s)") {
		t.Errorf("describeFastCheck = %q", got)
	}
}
