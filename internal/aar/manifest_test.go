package aar

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeManifest escribe un manifest en disco y devuelve su ruta.
func writeManifest(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const appManifest = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="com.ejemplo.app"
    android:versionCode="7">
    <uses-permission android:name="android.permission.INTERNET" />
    <application android:label="Ejemplo" android:theme="@style/AppTheme">
        <activity android:name=".MainActivity" android:exported="true" />
    </application>
</manifest>
`

const libManifest = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="com.ejemplo.lib">
    <uses-permission android:name="android.permission.INTERNET" />
    <uses-permission android:name="android.permission.CAMERA" />
    <application>
        <provider
            android:name="androidx.startup.InitializationProvider"
            android:authorities="${applicationId}.androidx-startup"
            android:exported="false" />
        <service android:name="com.ejemplo.lib.SyncService" android:exported="false" />
    </application>
</manifest>
`

func TestMergeManifestsConservaLosPrefijosDeNamespace(t *testing.T) {
	dir := t.TempDir()
	main := writeManifest(t, dir, "AndroidManifest.xml", appManifest)
	lib := writeManifest(t, dir, "lib.xml", libManifest)
	out := filepath.Join(dir, "merged.xml")

	if err := MergeManifests(main, []string{lib}, out); err != nil {
		t.Fatalf("MergeManifests: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)

	// El fallo clásico de esta fusión: al re-serializar el XML se perdía el
	// prefijo "android:" y aapt2 rechazaba el manifest.
	for _, want := range []string{
		`android:versionCode="7"`,
		`android:label="Ejemplo"`,
		`android:theme="@style/AppTheme"`,
		`android:name=".MainActivity"`,
		`android:exported="true"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("el manifest fusionado perdió %s:\n%s", want, text)
		}
	}
}

func TestMergeManifestsFusionaPermisosSinDuplicar(t *testing.T) {
	dir := t.TempDir()
	main := writeManifest(t, dir, "AndroidManifest.xml", appManifest)
	lib := writeManifest(t, dir, "lib.xml", libManifest)
	out := filepath.Join(dir, "merged.xml")

	if err := MergeManifests(main, []string{lib}, out); err != nil {
		t.Fatalf("MergeManifests: %v", err)
	}
	text := readFile(t, out)

	if n := strings.Count(text, "android.permission.INTERNET"); n != 1 {
		t.Errorf("INTERNET aparece %d veces, quiero 1:\n%s", n, text)
	}
	if !strings.Contains(text, "android.permission.CAMERA") {
		t.Errorf("falta el permiso CAMERA de la librería:\n%s", text)
	}
	// El permiso debe quedar fuera de <application>.
	appIdx := strings.Index(text, "<application")
	camIdx := strings.Index(text, "android.permission.CAMERA")
	if camIdx > appIdx {
		t.Errorf("el permiso quedó dentro de <application>:\n%s", text)
	}
}

func TestMergeManifestsFusionaComponentes(t *testing.T) {
	dir := t.TempDir()
	main := writeManifest(t, dir, "AndroidManifest.xml", appManifest)
	lib := writeManifest(t, dir, "lib.xml", libManifest)
	out := filepath.Join(dir, "merged.xml")

	if err := MergeManifests(main, []string{lib}, out); err != nil {
		t.Fatalf("MergeManifests: %v", err)
	}
	text := readFile(t, out)

	if !strings.Contains(text, "androidx.startup.InitializationProvider") {
		t.Errorf("falta el provider de la librería:\n%s", text)
	}
	if !strings.Contains(text, "com.ejemplo.lib.SyncService") {
		t.Errorf("falta el service de la librería:\n%s", text)
	}
	if !strings.Contains(text, ".MainActivity") {
		t.Errorf("se perdió la activity de la app:\n%s", text)
	}

	// Los componentes de la librería van dentro de <application>.
	closeApp := strings.Index(text, "</application>")
	provider := strings.Index(text, "InitializationProvider")
	if provider > closeApp {
		t.Errorf("el provider quedó fuera de <application>:\n%s", text)
	}
}

func TestMergeManifestsNoDuplicaComponentes(t *testing.T) {
	dir := t.TempDir()
	main := writeManifest(t, dir, "AndroidManifest.xml", appManifest)
	lib := writeManifest(t, dir, "lib.xml", libManifest)
	out := filepath.Join(dir, "merged.xml")

	if err := MergeManifests(main, []string{lib, lib}, out); err != nil {
		t.Fatalf("MergeManifests: %v", err)
	}
	text := readFile(t, out)

	if n := strings.Count(text, "InitializationProvider"); n != 1 {
		t.Errorf("el provider aparece %d veces, quiero 1:\n%s", n, text)
	}
}

func TestMergeManifestsNoRepiteLoQueYaTieneLaApp(t *testing.T) {
	dir := t.TempDir()
	main := writeManifest(t, dir, "AndroidManifest.xml", appManifest)
	lib := writeManifest(t, dir, "lib.xml", `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.ejemplo.lib">
    <application>
        <activity android:name=".MainActivity" android:exported="true" />
    </application>
</manifest>
`)
	out := filepath.Join(dir, "merged.xml")

	if err := MergeManifests(main, []string{lib}, out); err != nil {
		t.Fatalf("MergeManifests: %v", err)
	}
	text := readFile(t, out)

	if n := strings.Count(text, ".MainActivity"); n != 1 {
		t.Errorf("la activity aparece %d veces, quiero 1:\n%s", n, text)
	}
}

func TestMergeManifestsProduceXMLValido(t *testing.T) {
	dir := t.TempDir()
	main := writeManifest(t, dir, "AndroidManifest.xml", appManifest)
	lib := writeManifest(t, dir, "lib.xml", libManifest)
	out := filepath.Join(dir, "merged.xml")

	if err := MergeManifests(main, []string{lib}, out); err != nil {
		t.Fatalf("MergeManifests: %v", err)
	}

	dec := xml.NewDecoder(strings.NewReader(readFile(t, out)))
	for {
		_, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			t.Fatalf("el manifest fusionado no es XML válido: %v", err)
		}
	}
}

func TestMergeManifestsIgnoraManifiestosInexistentes(t *testing.T) {
	dir := t.TempDir()
	main := writeManifest(t, dir, "AndroidManifest.xml", appManifest)
	out := filepath.Join(dir, "merged.xml")

	if err := MergeManifests(main, []string{filepath.Join(dir, "no-existe.xml"), ""}, out); err != nil {
		t.Fatalf("MergeManifests: %v", err)
	}
	if !strings.Contains(readFile(t, out), ".MainActivity") {
		t.Error("el manifest principal debería conservarse")
	}
}

func TestMergeManifestsConApplicationAutoCerrado(t *testing.T) {
	dir := t.TempDir()
	main := writeManifest(t, dir, "AndroidManifest.xml", `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.ejemplo.app">
    <application android:label="Ejemplo" />
</manifest>
`)
	lib := writeManifest(t, dir, "lib.xml", libManifest)
	out := filepath.Join(dir, "merged.xml")

	if err := MergeManifests(main, []string{lib}, out); err != nil {
		t.Fatalf("MergeManifests: %v", err)
	}
	text := readFile(t, out)

	if !strings.Contains(text, "InitializationProvider") {
		t.Errorf("el componente no se insertó en el <application/>:\n%s", text)
	}
	if strings.Count(text, "<application") != 1 {
		t.Errorf("debería quedar un solo <application>:\n%s", text)
	}
}

// libManifestConTools reproduce lo que hacen las librerías de AndroidX:
// declaran el namespace "tools" en su raíz y lo usan en los componentes
// (tools:node="merge").
const libManifestConTools = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    xmlns:tools="http://schemas.android.com/tools"
    package="com.ejemplo.lib">
    <application>
        <provider
            android:name="androidx.startup.InitializationProvider"
            android:authorities="${applicationId}.androidx-startup"
            android:exported="false"
            tools:node="merge" />
    </application>
</manifest>
`

// TestMergeManifestsDeclaraLosNamespacesDeLasLibrerias comprueba que los
// prefijos que usan los fragmentos copiados (tools:node="merge" de AndroidX
// Startup) quedan declarados en el manifest fusionado.
//
// Sin esto, aapt2 falla al enlazar con "unbound prefix" y el build para.
func TestMergeManifestsDeclaraLosNamespacesDeLasLibrerias(t *testing.T) {
	dir := t.TempDir()
	main := writeManifest(t, dir, "AndroidManifest.xml", appManifest)
	lib := writeManifest(t, dir, "lib.xml", libManifestConTools)
	out := filepath.Join(dir, "merged.xml")

	if err := MergeManifests(main, []string{lib}, out); err != nil {
		t.Fatalf("MergeManifests: %v", err)
	}
	text := readFile(t, out)

	if !strings.Contains(text, `xmlns:tools="http://schemas.android.com/tools"`) {
		t.Errorf("el manifest fusionado no declara el namespace tools:\n%s", text)
	}
	if !strings.Contains(text, `tools:node="merge"`) {
		t.Errorf("el fragmento de la librería perdió su atributo tools:node:\n%s", text)
	}
	// El namespace declarado debe estar en el elemento raíz, no dentro.
	rootIdx := strings.Index(text, "<manifest")
	rootEnd := rootIdx + strings.Index(text[rootIdx:], ">")
	if !strings.Contains(text[rootIdx:rootEnd], "xmlns:tools") {
		t.Errorf("xmlns:tools no está en el elemento raíz:\n%s", text)
	}
	// Y el resultado debe seguir siendo XML bien formado.
	dec := xml.NewDecoder(strings.NewReader(text))
	for {
		if _, err := dec.Token(); err != nil {
			if err.Error() == "EOF" {
				break
			}
			t.Fatalf("el manifest fusionado no es XML válido: %v\n%s", err, text)
		}
	}
}

// TestMergeManifestsNoRedeclaraLoQueLaAppYaTrae comprueba que no se duplica
// una declaración que el manifest de la app ya traía.
func TestMergeManifestsNoRedeclaraLoQueLaAppYaTrae(t *testing.T) {
	dir := t.TempDir()
	main := writeManifest(t, dir, "AndroidManifest.xml", appManifest)
	lib := writeManifest(t, dir, "lib.xml", libManifestConTools)
	out := filepath.Join(dir, "merged.xml")

	if err := MergeManifests(main, []string{lib}, out); err != nil {
		t.Fatalf("MergeManifests: %v", err)
	}
	text := readFile(t, out)

	if n := strings.Count(text, `xmlns:android=`); n != 1 {
		t.Errorf("xmlns:android aparece %d veces, quiero 1:\n%s", n, text)
	}
	if n := strings.Count(text, `xmlns:tools=`); n != 1 {
		t.Errorf("xmlns:tools aparece %d veces, quiero 1:\n%s", n, text)
	}
}

// readFile lee un fichero y falla el test si hay error.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
