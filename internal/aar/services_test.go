package aar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWithImplicitPermissionsAñadePostNotifications comprueba que, como
// hace AGP, se declara POST_NOTIFICATIONS cuando el targetSdk lo exige.
func TestWithImplicitPermissionsAñadePostNotifications(t *testing.T) {
	manifest := `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="com.example.app">

    <application android:label="@string/app_name" />
</manifest>`

	out, err := WithImplicitPermissions(manifest, 36)
	if err != nil {
		t.Fatalf("WithImplicitPermissions: %v", err)
	}
	if !strings.Contains(out, `android.permission.POST_NOTIFICATIONS`) {
		t.Errorf("el manifest no declara POST_NOTIFICATIONS:\n%s", out)
	}
	// El permiso tiene que ir dentro de <manifest> y antes de <application>.
	perm := strings.Index(out, "POST_NOTIFICATIONS")
	app := strings.Index(out, "<application")
	if perm < 0 || app < 0 || perm > app {
		t.Errorf("el permiso debe ir antes de <application>:\n%s", out)
	}
	// La etiqueta de apertura debe seguir intacta.
	if !strings.Contains(out, `package="com.example.app">`) {
		t.Errorf("la etiqueta <manifest> se ha roto:\n%s", out)
	}
}

// TestWithImplicitPermissionsNoDuplicaNiAplicaAntesDeApi33 comprueba los dos
// casos en los que no hay que tocar el manifest.
func TestWithImplicitPermissionsNoDuplicaNiAplicaAntesDeApi33(t *testing.T) {
	manifest := `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <application />
</manifest>`

	// targetSdk bajo: no se toca nada.
	out, err := WithImplicitPermissions(manifest, 32)
	if err != nil {
		t.Fatalf("WithImplicitPermissions: %v", err)
	}
	if out != manifest {
		t.Errorf("con targetSdk 32 el manifest no debería cambiar:\n%s", out)
	}

	// Ya declarado: no se duplica.
	declarado := `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <uses-permission android:name="android.permission.POST_NOTIFICATIONS" />
    <application />
</manifest>`
	out, err = WithImplicitPermissions(declarado, 36)
	if err != nil {
		t.Fatalf("WithImplicitPermissions: %v", err)
	}
	if out != declarado {
		t.Errorf("el permiso ya declarado no debería duplicarse:\n%s", out)
	}
}

// TestServiceFilesDeLosJarYLosAAR comprueba que se recogen los proveedores de
// ServiceLoader tanto de los JAR como de la raíz de los AAR, y que se
// fusionan sin duplicar.
func TestServiceFilesDeLosJarYLosAAR(t *testing.T) {
	dir := t.TempDir()

	// JAR plano con un fichero de servicio.
	jar := filepath.Join(dir, "coroutines-android.jar")
	writeZip(t, jar, map[string]string{
		"kotlinx/coroutines/android/AndroidDispatcherFactory.class":           "clase",
		"META-INF/services/kotlinx.coroutines.internal.MainDispatcherFactory": "kotlinx.coroutines.android.AndroidDispatcherFactory\n",
	})

	// AAR: el servicio está en la raíz del AAR, no en classes.jar.
	aarService := filepath.Join(dir, "aar-service.txt")
	if err := os.WriteFile(aarService, []byte("com.example.Provider\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	set := &Set{
		Dir: dir,
		Libraries: []Library{
			{Coordinate: "org.jetbrains.kotlinx:kotlinx-coroutines-android:1.9.0", ClassJar: jar},
			{Coordinate: "com.example:lib:1.0", ServiceFiles: map[string]string{
				"com.example.Service": aarService,
			}},
		},
	}

	services, err := set.ServiceFiles()
	if err != nil {
		t.Fatalf("ServiceFiles: %v", err)
	}

	main := services["kotlinx.coroutines.internal.MainDispatcherFactory"]
	if len(main) != 1 || main[0] != "kotlinx.coroutines.android.AndroidDispatcherFactory" {
		t.Errorf("servicio principal = %v, quiero el AndroidDispatcherFactory", main)
	}
	other := services["com.example.Service"]
	if len(other) != 1 || other[0] != "com.example.Provider" {
		t.Errorf("servicio del AAR = %v, quiero el Provider", other)
	}
}

// TestServiceFilesIgnoraComentariosYDupes comprueba el formato del fichero
// de servicio: líneas vacías, comentarios y repetidos se descartan.
func TestServiceFilesIgnoraComentariosYDupes(t *testing.T) {
	dir := t.TempDir()
	jar := filepath.Join(dir, "lib.jar")
	writeZip(t, jar, map[string]string{
		"META-INF/services/com.example.Service": "# un comentario\n\ncom.example.A\ncom.example.A\ncom.example.B\n",
	})

	services, err := ServiceFilesFromJars([]string{jar})
	if err != nil {
		t.Fatalf("ServiceFilesFromJars: %v", err)
	}
	got := services["com.example.Service"]
	if len(got) != 2 || got[0] != "com.example.A" || got[1] != "com.example.B" {
		t.Errorf("servicio = %v, quiero [com.example.A com.example.B]", got)
	}
}
