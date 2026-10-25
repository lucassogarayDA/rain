package build

import (
	"archive/zip"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/itskreisler/rain/internal/aar"
)

// El test de integración no usa emulador ni adb: la verificación es estática
// sobre el APK ya construido (manifiesto fusionado, tabla de recursos, dex y
// firma). Arranque en dispositivo queda pendiente por falta de emulador en el
// entorno de CI.

const manifestAppIntegracion = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="com.example.integration">

    <uses-permission android:name="android.permission.INTERNET" />

    <application android:label="@string/app_name">
        <activity android:name=".MainActivity" android:exported="true">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>
    </application>
</manifest>
`

const recursosAppIntegracion = `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="app_name">App de Integración</string>
</resources>
`

const fuenteAppIntegracion = `package com.example.integration;

public class MainActivity extends android.app.Activity {
    @Override
    protected void onCreate(android.os.Bundle estado) {
        super.onCreate(estado);
    }
}
`

// manifestLibreriaIntegracion imita lo que declara una librería real de
// AndroidX (AppCompat, Startup...): un permiso propio y un provider que se
// inicializa al arrancar la app.
const manifestLibreriaIntegracion = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="com.example.androidx">

    <uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />

    <application>
        <provider
            android:name="com.example.androidx.InitializationProvider"
            android:authorities="com.example.integration.androidx-init"
            android:exported="false"
            android:initOrder="100" />
    </application>
</manifest>
`

const recursosLibreriaIntegracion = `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="lib_name">Recurso de la librería</string>
    <style name="AppTheme" parent="android:Theme.Material.Light" />
</resources>
`

const fuenteLibreriaIntegracion = `package com.example.androidx;

public final class LibraryHelper {
    public static String saludar() {
        return "hola";
    }
}
`

// toolchainIntegracion deja el PATH listo y comprueba que está todo el
// toolchain. Si falta algo, el test se salta en lugar de fallar: el pipeline
// completo necesita aapt2, javac, keytool, zipalign, d8 y apksigner.
func toolchainIntegracion(t *testing.T) string {
	t.Helper()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no se pudo resolver el home: %v", err)
	}
	if bin := filepath.Join(home, ".rain", "bin"); dirExists(bin) {
		t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	}

	androidJar := filepath.Join(home, ".rain", "toolchain", "android.jar")
	if !fileExists(androidJar) {
		t.Skip("falta ~/.rain/toolchain/android.jar (ejecuta 'rain doctor --fix')")
	}
	for _, tool := range []string{"aapt2", "javac", "keytool", "zipalign", "d8", "apksigner"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("falta la herramienta %s en el PATH", tool)
		}
	}
	return androidJar
}

// compilarJarLibreria compila la librería sintética: javac + un zip con las clases.
func compilarJarLibreria(t *testing.T, srcDir, dest string) {
	t.Helper()

	classDir := filepath.Join(filepath.Dir(dest), "classes")
	if err := os.MkdirAll(classDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("javac", "-d", classDir, "--release", "11")
	cmd.Args = append(cmd.Args, filepath.Join(srcDir, "com", "example", "androidx", "LibraryHelper.java"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac de la librería: %v\n%s", err, out)
	}

	f, err := os.Create(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	zw := zip.NewWriter(f)
	err = filepath.Walk(classDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(classDir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestIntegracionAPKConLibreriaAndroidX es el test de integración de Rain:
// compila una app completa con una dependencia estilo AndroidX y comprueba que
// el APK resultante lleva todo lo que hace falta en un dispositivo real.
//
// Se verifica de forma estática (sin emulador):
//   - el manifiesto fusionado declara los componentes de la librería (el
//     provider de inicialización) y conserva los de la app;
//   - los permisos de la app y de la librería están los dos;
//   - la tabla de recursos incluye los recursos de la librería;
//   - el dex contiene el código de la librería;
//   - el APK está firmado y la firma valida.
func TestIntegracionAPKConLibreriaAndroidX(t *testing.T) {
	androidJar := toolchainIntegracion(t)

	tmp := t.TempDir()
	appDir := filepath.Join(tmp, "app")
	buildDir := filepath.Join(tmp, "build")
	depsDir := filepath.Join(tmp, "deps")

	writeFile(t, filepath.Join(appDir, "AndroidManifest.xml"), manifestAppIntegracion)
	writeFile(t, filepath.Join(appDir, "res", "values", "strings.xml"), recursosAppIntegracion)
	writeFile(t, filepath.Join(appDir, "java", "com", "example", "integration", "MainActivity.java"), fuenteAppIntegracion)

	// --- Librería sintética empaquetada como AAR ---
	libSrc := filepath.Join(tmp, "lib-src")
	writeFile(t, filepath.Join(libSrc, "com", "example", "androidx", "LibraryHelper.java"), fuenteLibreriaIntegracion)
	classesJar := filepath.Join(tmp, "classes.jar")
	compilarJarLibreria(t, libSrc, classesJar)

	aarPath := filepath.Join(tmp, "androidx-1.0.0.aar")
	crearAAR(t, aarPath, map[string][]byte{
		"AndroidManifest.xml":    []byte(manifestLibreriaIntegracion),
		"res/values/strings.xml": []byte(recursosLibreriaIntegracion),
		"classes.jar":            debeLeer(t, classesJar),
	})

	lib, err := aar.ExtractAAR(aarPath, "com.example:androidx:1.0.0", depsDir)
	if err != nil {
		t.Fatalf("preparando el AAR: %v", err)
	}
	if lib.ClassJar == "" {
		t.Fatal("el AAR no traía classes.jar")
	}

	cfg := Config{
		AppDir:      appDir,
		BuildDir:    buildDir,
		AndroidJar:  androidJar,
		Package:     "com.example.integration",
		MinSdk:      21,
		TargetSdk:   36,
		VersionCode: 1,
		VersionName: "1.0",
		Libs:        &aar.Set{Dir: depsDir, Libraries: []aar.Library{*lib}},
		JVMTarget:   17,
	}

	if err := Run(cfg); err != nil {
		t.Fatalf("Run falló: %v", err)
	}

	apk := filepath.Join(buildDir, "app-debug.apk")
	if !fileExists(apk) {
		t.Fatalf("no se generó %s", apk)
	}

	// --- Firma ---
	salida, err := exec.Command("apksigner", "verify", "--print-certs", apk).CombinedOutput()
	if err != nil {
		t.Fatalf("apksigner verify: %v\n%s", err, salida)
	}
	if !strings.Contains(string(salida), "certificate DN") {
		t.Errorf("apksigner verify no devolvió el certificado:\n%s", salida)
	}

	// --- Contenido del APK ---
	entradas := readZip(t, apk)
	for _, want := range []string{"AndroidManifest.xml", "resources.arsc", "classes.dex"} {
		if _, ok := entradas[want]; !ok {
			t.Errorf("el APK no contiene %s", want)
		}
	}
	if !tieneFirma(entradas) {
		t.Error("el APK no lleva los ficheros de firma (META-INF/*.SF)")
	}

	// --- Manifiesto fusionado ---
	arbol := salidaComando(t, "aapt2", "dump", "xmltree", apk, "--file", "AndroidManifest.xml")
	for _, want := range []string{
		"com.example.androidx.InitializationProvider", // componente de la librería
		".MainActivity",                         // componente de la app
		"com.example.integration.androidx-init", // authorities del provider
	} {
		if !strings.Contains(arbol, want) {
			t.Errorf("el manifiesto fusionado no contiene %q:\n%s", want, arbol)
		}
	}

	// --- Permisos: los de la app y los de la librería ---
	permisos := salidaComando(t, "aapt2", "dump", "permissions", apk)
	for _, want := range []string{"android.permission.INTERNET", "android.permission.ACCESS_NETWORK_STATE"} {
		if !strings.Contains(permisos, want) {
			t.Errorf("el APK no declara el permiso %s:\n%s", want, permisos)
		}
	}

	// --- Tabla de recursos ---
	recursos := salidaComando(t, "aapt2", "dump", "resources", apk)
	for _, want := range []string{"app_name", "lib_name", "AppTheme"} {
		if !strings.Contains(recursos, want) {
			t.Errorf("la tabla de recursos no incluye %q", want)
		}
	}

	// --- Dex: el código de la librería y el de la app tienen que estar dentro ---
	// En debug el DEX se genera en dos pasadas: la app ocupa classes.dex y las
	// dependencias classes2.dex, así que hay que buscar en todos los DEX.
	var dex bytes.Buffer
	for name, contenido := range entradas {
		if strings.HasPrefix(name, "classes") && strings.HasSuffix(name, ".dex") {
			dex.WriteString(contenido)
		}
	}
	if !bytes.Contains(dex.Bytes(), []byte("Lcom/example/androidx/LibraryHelper;")) {
		t.Error("el dex no contiene la clase de la librería")
	}
	if !bytes.Contains(dex.Bytes(), []byte("Lcom/example/integration/MainActivity;")) {
		t.Error("el dex no contiene la activity de la app")
	}
}

// tieneFirma indica si el APK lleva los ficheros de firma de la v1.
func tieneFirma(entradas map[string]string) bool {
	for name := range entradas {
		if strings.HasPrefix(name, "META-INF/") && strings.HasSuffix(name, ".SF") {
			return true
		}
	}
	return false
}

// crearAAR escribe un .aar con las entradas dadas.
func crearAAR(t *testing.T, path string, entries map[string][]byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	zw := zip.NewWriter(f)
	for name, data := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

// debeLeer lee un fichero y falla el test si no puede.
func debeLeer(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// salidaComando ejecuta una herramienta y devuelve su salida (stdout+stderr).
func salidaComando(t *testing.T, name string, args ...string) string {
	t.Helper()
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return string(out)
}
