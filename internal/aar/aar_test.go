package aar

import (
	"archive/zip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeZip crea un zip con las entradas dadas (nombre -> contenido).
func writeZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	zw := zip.NewWriter(f)
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExtractAARFull(t *testing.T) {
	dir := t.TempDir()
	aarPath := filepath.Join(dir, "lib.aar")
	writeZip(t, aarPath, map[string]string{
		"classes.jar": "fake jar bytes",
		"res/values/strings.xml": `<?xml version="1.0"?><resources>` +
			`<string name="lib_name">Lib</string></resources>`,
		"res/layout/lib_view.xml":   `<LinearLayout />`,
		"AndroidManifest.xml":       `<manifest package="com.example.lib"></manifest>`,
		"jni/arm64-v8a/libfoo.so":   "ELF arm64",
		"jni/armeabi-v7a/libfoo.so": "ELF arm",
		"proguard.txt":              "-keep class com.example.lib.** { *; }",
	})

	c := NewClient()
	lib, err := c.extractAAR("com.example:lib:1.0", "lib", "1.0", aarPath, dir)
	if err != nil {
		t.Fatalf("extractAAR: %v", err)
	}

	if lib.Coordinate != "com.example:lib:1.0" {
		t.Errorf("Coordinate = %q", lib.Coordinate)
	}
	if lib.ClassJar != filepath.Join(dir, "jars", "lib-1.0.jar") {
		t.Errorf("ClassJar = %q", lib.ClassJar)
	}
	if _, err := os.Stat(lib.ClassJar); err != nil {
		t.Errorf("classes.jar no extraído: %v", err)
	}
	if lib.Package != "com.example.lib" {
		t.Errorf("Package = %q, quiero com.example.lib", lib.Package)
	}
	if want := filepath.Join(dir, "res", "lib-1.0"); lib.ResDir != want {
		t.Errorf("ResDir = %q, quiero %q", lib.ResDir, want)
	}
	for _, rel := range []string{"values/strings.xml", "layout/lib_view.xml"} {
		if _, err := os.Stat(filepath.Join(lib.ResDir, rel)); err != nil {
			t.Errorf("recurso %s no extraído: %v", rel, err)
		}
	}
	// Los .so se extraen en un subdirectorio por coordenada y las ABIs se
	// registran ordenadas.
	for _, abi := range []string{"arm64-v8a", "armeabi-v7a"} {
		so := filepath.Join(dir, "jni", "lib-1.0", abi, "libfoo.so")
		if _, err := os.Stat(so); err != nil {
			t.Errorf("nativo %s no extraído: %v", so, err)
		}
	}
	if len(lib.NativeABIs) != 2 || lib.NativeABIs[0] != "arm64-v8a" || lib.NativeABIs[1] != "armeabi-v7a" {
		t.Errorf("NativeABIs = %v", lib.NativeABIs)
	}
	if len(lib.NativeFiles) != 2 {
		t.Fatalf("NativeFiles = %v, quiero las dos .so", lib.NativeFiles)
	}
	for _, path := range lib.NativeFiles {
		if !strings.HasPrefix(path, filepath.Join(dir, "jni", "lib-1.0")+string(filepath.Separator)) || filepath.Base(path) != "libfoo.so" {
			t.Errorf("NativeFiles incluye una ruta inesperada: %q", path)
		}
	}
	// Las reglas ProGuard del AAR se extraen.
	if lib.ProguardRules != filepath.Join(dir, "proguard", "lib-1.0.pro") {
		t.Errorf("ProguardRules = %q", lib.ProguardRules)
	}
	if _, err := os.Stat(lib.ProguardRules); err != nil {
		t.Errorf("proguard.txt no extraído: %v", err)
	}
}

func TestSetProguardFiles(t *testing.T) {
	set := &Set{
		Libraries: []Library{
			{Coordinate: "b:b:1", ProguardRules: "/b/b.pro"},
			{Coordinate: "a:a:1", ProguardRules: "/a/a.pro"},
			{Coordinate: "c:c:1"}, // sin reglas
		},
	}
	got := set.ProguardFiles()
	if len(got) != 2 || got[0] != "/a/a.pro" || got[1] != "/b/b.pro" {
		t.Errorf("ProguardFiles = %v, quiero [/a/a.pro /b/b.pro]", got)
	}
}

func TestExtractAARResourcesWithoutCode(t *testing.T) {
	// Un AAR sin classes.jar no debe aportar --extra-packages aunque su
	// manifest declare un package.
	dir := t.TempDir()
	aarPath := filepath.Join(dir, "res.aar")
	writeZip(t, aarPath, map[string]string{
		"res/values/strings.xml": `<?xml version="1.0"?><resources><string name="x">y</string></resources>`,
		"AndroidManifest.xml":    `<manifest package="com.example.res"></manifest>`,
	})

	c := NewClient()
	lib, err := c.extractAAR("com.example:res:1.0", "res", "1.0", aarPath, dir)
	if err != nil {
		t.Fatalf("extractAAR: %v", err)
	}
	if lib.ClassJar != "" {
		t.Errorf("ClassJar = %q, quiero vacío", lib.ClassJar)
	}
	if lib.Package != "" {
		t.Errorf("Package = %q, un AAR sin código no debe aportar package", lib.Package)
	}
	if lib.ResDir == "" {
		t.Error("se esperaba ResDir con el resource extraído")
	}
}

func TestExtractAARRejectsInvalid(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.aar")
	if err := os.WriteFile(bad, []byte("no soy un zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := NewClient()
	if _, err := c.extractAAR("a:b:1", "b", "1", bad, dir); err == nil {
		t.Fatal("extractAAR de un fichero no-zip debería fallar")
	}
}

func TestPrepareInvalidCoordinate(t *testing.T) {
	c := NewClient()
	if _, err := c.Prepare([]string{"sin:version"}, t.TempDir()); err == nil {
		t.Fatal("coordenada inválida debería fallar")
	}
}

func TestSetAccessors(t *testing.T) {
	dir := t.TempDir()
	set := &Set{
		Dir: dir,
		Libraries: []Library{
			{Coordinate: "a:a:1", ClassJar: "/a/a-1.jar", ResDir: "/a/res", Package: "com.a"},
			{Coordinate: "b:b:1", ClassJar: "/b/b-1.jar", ResDir: "/b/res", Package: "com.b"},
			{Coordinate: "c:c:1", ClassJar: "/c/c-1.jar", Package: "com.a"}, // package duplicado
			{Coordinate: "d:d:1", ResDir: "/d/res"},                         // sin código
		},
	}

	if got := set.ClassJars(); len(got) != 3 {
		t.Errorf("ClassJars = %v, quiero 3", got)
	}
	if got := set.ResDirs(); len(got) != 3 {
		t.Errorf("ResDirs = %v, quiero 3", got)
	}
	if got := set.ExtraPackages(); len(got) != 2 || got[0] != "com.a" || got[1] != "com.b" {
		t.Errorf("ExtraPackages = %v, quiero [com.a com.b]", got)
	}

	// NativeDir solo existe si hay jni/.
	if set.NativeDir() != "" {
		t.Errorf("NativeDir = %q, quiero vacío sin jni/", set.NativeDir())
	}
	if err := os.MkdirAll(filepath.Join(dir, "jni", "arm64-v8a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if set.NativeDir() != filepath.Join(dir, "jni") {
		t.Errorf("NativeDir = %q, quiero %q", set.NativeDir(), filepath.Join(dir, "jni"))
	}
}

func TestSetNativeLibsSoloDelSetActual(t *testing.T) {
	dir := t.TempDir()
	jni := filepath.Join(dir, "jni", "arm64-v8a")
	if err := os.MkdirAll(jni, 0o755); err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(jni, "libactual.so")
	stale := filepath.Join(jni, "libretirada.so")
	for _, path := range []string{current, stale} {
		if err := os.WriteFile(path, []byte("so"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	set := &Set{
		Dir: dir,
		Libraries: []Library{
			{Coordinate: "a:a:1", NativeABIs: []string{"arm64-v8a"}, NativeFiles: []string{current}},
		},
	}

	libs := set.NativeLibs()
	if len(libs) != 1 {
		t.Fatalf("NativeLibs = %v, quiero solo la del set actual", libs)
	}
	if got := libs["lib/arm64-v8a/libactual.so"]; got != current {
		t.Errorf("NativeLibs[actual] = %q, quiero %q", got, current)
	}
	if _, ok := libs["lib/arm64-v8a/libretirada.so"]; ok {
		t.Error("NativeLibs incluye una .so de una dependencia retirada")
	}
}

func TestSetNativeLibsAislaDependenciasParalelas(t *testing.T) {
	dir := t.TempDir()
	jni := filepath.Join(dir, "jni")

	// Dos dependencias con la misma .so. Al prepararse en paralelo, cada una
	// escribe en su subdirectorio; en el APK solo puede quedar una entrada y
	// debe ser la última del set (orden determinista de coordenadas).
	primera := filepath.Join(jni, "a-1", "arm64-v8a", "libc++_shared.so")
	segunda := filepath.Join(jni, "b-2", "arm64-v8a", "libc++_shared.so")
	otra := filepath.Join(jni, "b-2", "armeabi-v7a", "libextra.so")
	for path, data := range map[string]string{primera: "a", segunda: "b", otra: "c"} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	set := &Set{Dir: dir, Libraries: []Library{
		{Coordinate: "g:a:1", NativeFiles: []string{primera}},
		{Coordinate: "g:b:2", NativeFiles: []string{segunda, otra}},
	}}

	libs := set.NativeLibs()
	if len(libs) != 2 {
		t.Fatalf("NativeLibs = %v, quiero 2 entradas sin duplicados", libs)
	}
	if got := libs["lib/arm64-v8a/libc++_shared.so"]; got != segunda {
		t.Errorf("la .so duplicada = %q, quiero la última del set (%q)", got, segunda)
	}
	if got := libs["lib/armeabi-v7a/libextra.so"]; got != otra {
		t.Errorf("NativeLibs[extra] = %q, quiero %q", got, otra)
	}
	for key := range libs {
		if strings.Contains(key, "a-1/") || strings.Contains(key, "b-2/") {
			t.Errorf("la clave del APK conserva el subdirectorio de la coordenada: %q", key)
		}
	}
}

func TestJarHasClasses(t *testing.T) {
	dir := t.TempDir()

	// Stub de Kotlin Multiplatform: solo metadata, sin .class.
	stub := filepath.Join(dir, "stub.jar")
	writeZip(t, stub, map[string]string{
		"META-INF/MANIFEST.MF":                            "Manifest-Version: 1.0\n",
		"META-INF/kotlin-project-structure-metadata.json": "{}",
	})
	if jarHasClasses(stub) {
		t.Error("jarHasClasses(stub) = true, quiero false")
	}

	// JAR con clases reales.
	full := filepath.Join(dir, "full.jar")
	writeZip(t, full, map[string]string{"androidx/annotation/NonNull.class": "bytecode"})
	if !jarHasClasses(full) {
		t.Error("jarHasClasses(full) = false, quiero true")
	}

	// Archivo inexistente o corrupto: false sin pánico.
	if jarHasClasses(filepath.Join(dir, "nope.jar")) {
		t.Error("jarHasClasses(inexistente) = true, quiero false")
	}
}

func TestPrepareJARFallsBackToJVMVariant(t *testing.T) {
	dir := t.TempDir()
	serveDir := t.TempDir()

	// El artefacto base es un stub KMP sin clases.
	writeZip(t, filepath.Join(serveDir, "annotation-1.6.0.jar"), map[string]string{
		"META-INF/kotlin-project-structure-metadata.json": "{}",
	})
	// La variante -jvm trae las clases.
	writeZip(t, filepath.Join(serveDir, "annotation-jvm-1.6.0.jar"), map[string]string{
		"androidx/annotation/NonNull.class": "bytecode",
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Base(r.URL.Path)
		data, err := os.ReadFile(filepath.Join(serveDir, name))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	old := repositories
	repositories = []string{srv.URL}
	defer func() { repositories = old }()

	c := NewClient()
	lib, err := c.prepareOne("androidx.annotation:annotation:1.6.0", dir)
	if err != nil {
		t.Fatalf("prepareOne: %v", err)
	}
	if lib.Coordinate != "androidx.annotation:annotation-jvm:1.6.0" {
		t.Errorf("Coordinate = %q, quiero la variante -jvm", lib.Coordinate)
	}
	if !jarHasClasses(lib.ClassJar) {
		t.Errorf("ClassJar %q no tiene clases", lib.ClassJar)
	}
}

func TestPrepareJARRedirectsToAndroidVariant(t *testing.T) {
	dir := t.TempDir()
	serveDir := t.TempDir()

	// La raíz KMP es un JAR de metadata sin clases.
	writeZip(t, filepath.Join(serveDir, "backdrop-2.0.1.jar"), map[string]string{
		"META-INF/kotlin-project-structure-metadata.json": "{}",
	})
	const metadata = `{
	  "variants": [
	    {
	      "name": "androidApiElements-published",
	      "attributes": {
	        "org.gradle.jvm.environment": "android",
	        "org.gradle.usage": "java-api"
	      },
	      "available-at": {
	        "group": "io.github.kyant0",
	        "module": "backdrop-android",
	        "version": "2.0.1"
	      }
	    }
	  ]
	}`
	if err := os.WriteFile(filepath.Join(serveDir, "backdrop-2.0.1.module"), []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}

	// La variante Android es un AAR con clases reales.
	writeZip(t, filepath.Join(serveDir, "classes.jar"), map[string]string{
		"com/kyant/backdrop/Backdrop.class": "bytecode",
	})
	classes, err := os.ReadFile(filepath.Join(serveDir, "classes.jar"))
	if err != nil {
		t.Fatal(err)
	}
	writeZip(t, filepath.Join(serveDir, "backdrop-android-2.0.1.aar"), map[string]string{
		"classes.jar":         string(classes),
		"AndroidManifest.xml": `<manifest package="com.kyant.backdrop"></manifest>`,
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(filepath.Join(serveDir, filepath.Base(r.URL.Path)))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	old := repositories
	repositories = []string{srv.URL}
	defer func() { repositories = old }()

	c := NewClient()
	lib, err := c.prepareOne("io.github.kyant0:backdrop:2.0.1", dir)
	if err != nil {
		t.Fatalf("prepareOne: %v", err)
	}
	if lib.Coordinate != "io.github.kyant0:backdrop-android:2.0.1" {
		t.Errorf("Coordinate = %q, quiero la variante Android", lib.Coordinate)
	}
	if !jarHasClasses(lib.ClassJar) {
		t.Errorf("ClassJar %q no tiene clases", lib.ClassJar)
	}
}

func TestPrepareJARKeepsPlainJar(t *testing.T) {
	dir := t.TempDir()
	serveDir := t.TempDir()

	// JAR normal con clases: no se busca variante -jvm.
	writeZip(t, filepath.Join(serveDir, "guava-1.0.jar"), map[string]string{
		"com/google/common/Base.class": "bytecode",
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(filepath.Join(serveDir, filepath.Base(r.URL.Path)))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	old := repositories
	repositories = []string{srv.URL}
	defer func() { repositories = old }()

	c := NewClient()
	lib, err := c.prepareOne("com.google.guava:guava:1.0", dir)
	if err != nil {
		t.Fatalf("prepareOne: %v", err)
	}
	if lib.Coordinate != "com.google.guava:guava:1.0" {
		t.Errorf("Coordinate = %q, quiero el JAR original", lib.Coordinate)
	}
}
