package build

import (
	"os"
	"path/filepath"
	"testing"
)

// writeHashFile escribe un fichero creando los directorios necesarios.
func writeHashFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creando %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("escribiendo %s: %v", path, err)
	}
}

func TestComputeResourceHashCambiaConElContenido(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "app")

	writeHashFile(t, filepath.Join(app, "src", "main", "AndroidManifest.xml"),
		`<manifest package="com.x"/>`)
	writeHashFile(t, filepath.Join(app, "src", "main", "res", "values", "strings.xml"),
		`<resources><string name="a">1</string></resources>`)

	first, err := computeResourceHash(app)
	if err != nil {
		t.Fatalf("computeResourceHash: %v", err)
	}
	if first == "" {
		t.Fatal("computeResourceHash devolvió un hash vacío")
	}

	// Sin cambios: el hash debe ser estable.
	second, err := computeResourceHash(app)
	if err != nil {
		t.Fatalf("computeResourceHash: %v", err)
	}
	if first != second {
		t.Errorf("hash inestable: %s != %s", first, second)
	}

	// Cambia un recurso.
	writeHashFile(t, filepath.Join(app, "src", "main", "res", "values", "strings.xml"),
		`<resources><string name="a">2</string></resources>`)
	changed, err := computeResourceHash(app)
	if err != nil {
		t.Fatalf("computeResourceHash: %v", err)
	}
	if changed == first {
		t.Error("el hash no cambió al modificar un recurso")
	}

	// Cambia el manifest.
	writeHashFile(t, filepath.Join(app, "src", "main", "AndroidManifest.xml"),
		`<manifest package="com.y"/>`)
	changedManifest, err := computeResourceHash(app)
	if err != nil {
		t.Fatalf("computeResourceHash: %v", err)
	}
	if changedManifest == changed {
		t.Error("el hash no cambió al modificar el manifest")
	}
}

func TestComputeResourceHashSinRecursos(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "app")
	writeHashFile(t, filepath.Join(app, "src", "main", "AndroidManifest.xml"),
		`<manifest package="com.x"/>`)

	// Un proyecto sin res/ es válido: no debe fallar.
	hash, err := computeResourceHash(app)
	if err != nil {
		t.Fatalf("computeResourceHash sin res/: %v", err)
	}
	if hash == "" {
		t.Error("hash vacío sin recursos")
	}
}

func TestComputeJavaHashCambiaConElContenido(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "app")
	libs := filepath.Join(dir, "libs")
	gen := filepath.Join(dir, "gen")

	writeHashFile(t, filepath.Join(app, "java", "com", "x", "Main.kt"), "package com.x\n")
	writeHashFile(t, filepath.Join(gen, "com", "x", "BuildConfig.java"), "package com.x;\n")
	writeHashFile(t, filepath.Join(libs, "dep.jar"), "contenido")

	first, err := computeJavaHash(app, libs, gen, false)
	if err != nil {
		t.Fatalf("computeJavaHash: %v", err)
	}

	second, err := computeJavaHash(app, libs, gen, false)
	if err != nil {
		t.Fatalf("computeJavaHash: %v", err)
	}
	if first != second {
		t.Errorf("hash inestable: %s != %s", first, second)
	}

	// Cambia una fuente Kotlin.
	writeHashFile(t, filepath.Join(app, "java", "com", "x", "Main.kt"), "package com.x\n// cambio\n")
	changed, err := computeJavaHash(app, libs, gen, false)
	if err != nil {
		t.Fatalf("computeJavaHash: %v", err)
	}
	if changed == first {
		t.Error("el hash no cambió al modificar una fuente Kotlin")
	}

	// Cambia una dependencia.
	writeHashFile(t, filepath.Join(libs, "dep.jar"), "otro contenido")
	changedLib, err := computeJavaHash(app, libs, gen, false)
	if err != nil {
		t.Fatalf("computeJavaHash: %v", err)
	}
	if changedLib == changed {
		t.Error("el hash no cambió al modificar una dependencia")
	}
}

func TestComputeJavaHashDistingueReleaseYDebug(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "app")
	writeHashFile(t, filepath.Join(app, "java", "com", "x", "Main.kt"), "package com.x\n")

	debug, err := computeJavaHash(app, filepath.Join(dir, "libs"), filepath.Join(dir, "gen"), false)
	if err != nil {
		t.Fatalf("computeJavaHash: %v", err)
	}
	release, err := computeJavaHash(app, filepath.Join(dir, "libs"), filepath.Join(dir, "gen"), true)
	if err != nil {
		t.Fatalf("computeJavaHash: %v", err)
	}
	if debug == release {
		t.Error("debug y release producen el mismo hash: no se regeneraría el DEX")
	}
}

func TestComputeJavaHashIgnoraFuentesNoJava(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "app")
	writeHashFile(t, filepath.Join(app, "java", "com", "x", "Main.kt"), "package com.x\n")

	first, err := computeJavaHash(app, filepath.Join(dir, "libs"), filepath.Join(dir, "gen"), false)
	if err != nil {
		t.Fatalf("computeJavaHash: %v", err)
	}

	// Un .txt no debe alterar el hash de compilación.
	writeHashFile(t, filepath.Join(app, "java", "com", "x", "notas.txt"), "sin efecto\n")
	second, err := computeJavaHash(app, filepath.Join(dir, "libs"), filepath.Join(dir, "gen"), false)
	if err != nil {
		t.Fatalf("computeJavaHash: %v", err)
	}
	if first != second {
		t.Error("un fichero que no es fuente alteró el hash")
	}
}
