package toolchain

import (
	"archive/zip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewUsesHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	manager, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if want := filepath.Join(home, ".rain"); manager.Root != want {
		t.Errorf("Root = %q, quiero %q", manager.Root, want)
	}
	if manager.BinDir() != filepath.Join(home, ".rain", "bin") {
		t.Errorf("BinDir = %q", manager.BinDir())
	}
	if manager.AndroidJar() != filepath.Join(home, ".rain", "toolchain", "android.jar") {
		t.Errorf("AndroidJar = %q", manager.AndroidJar())
	}
}

func TestEnsureIsIdempotentWithCachedFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	manager, err := New()
	if err != nil {
		t.Fatal(err)
	}
	// Servidor que falla: si los archivos ya existen, no debe llamarse.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("no debería descargar %s: los archivos ya están en caché", r.URL.Path)
		http.Error(w, "no", http.StatusInternalServerError)
	}))
	defer server.Close()
	manager.Client = server.Client()

	if err := os.MkdirAll(manager.ToolDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{manager.R8Jar(), manager.ApksignerJar(), manager.AndroidJar()} {
		if err := os.WriteFile(path, []byte("fake"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := manager.Ensure(context.Background(), false); err != nil {
		t.Fatalf("Ensure con caché: %v", err)
	}

	for _, name := range []string{"d8", "apksigner"} {
		info, err := os.Stat(filepath.Join(manager.BinDir(), name))
		if err != nil {
			t.Fatalf("wrapper %s no creado: %v", name, err)
		}
		if info.Mode().Perm() != 0o755 {
			t.Errorf("wrapper %s permisos = %v, quiero 0755", name, info.Mode().Perm())
		}
	}
}

func TestExtractOne(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "build-tools.zip")
	writeZipFile(t, zipPath, map[string]string{
		"android-16/lib/d8.jar":        "d8",
		"android-16/lib/apksigner.jar": "apksigner",
		"android-16/aapt2":             "binario",
	})

	dest := filepath.Join(dir, "out", "apksigner.jar")
	err := extractOne(zipPath, func(name string) bool {
		return filepath.Base(name) == "apksigner.jar"
	}, dest)
	if err != nil {
		t.Fatalf("extractOne: %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "apksigner" {
		t.Errorf("contenido = %q, quiero apksigner", data)
	}
}

func TestExtractOneNotFound(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "x.zip")
	writeZipFile(t, zipPath, map[string]string{"a.txt": "a"})

	err := extractOne(zipPath, func(name string) bool { return name == "noexiste" }, filepath.Join(dir, "out"))
	if err == nil {
		t.Fatal("extractOne debería fallar si no encuentra la entrada")
	}
}

func TestExtractPrefix(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "kotlin.zip")
	writeZipFile(t, zipPath, map[string]string{
		"kotlinc/bin/kotlinc":           "#!/bin/sh\n",
		"kotlinc/lib/kotlin-stdlib.jar": "stdlib",
		"otro/archivo.txt":              "no copiar",
	})

	destDir := filepath.Join(dir, "toolchain", "kotlinc")
	if err := extractPrefix(zipPath, "kotlinc/", destDir); err != nil {
		t.Fatalf("extractPrefix: %v", err)
	}

	if _, err := os.Stat(filepath.Join(destDir, "bin", "kotlinc")); err != nil {
		t.Errorf("falta kotlinc: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "lib", "kotlin-stdlib.jar")); err != nil {
		t.Errorf("falta kotlin-stdlib.jar: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "toolchain", "kotlinc", "otro")); err == nil {
		t.Error("extractPrefix no debería copiar entradas fuera del prefijo")
	}
}

func TestSafeJoinRejectsZipSlip(t *testing.T) {
	base := t.TempDir()

	if _, err := safeJoin(base, "../fuera.txt"); err == nil {
		t.Error("safeJoin debería rechazar ../fuera.txt")
	}
	if _, err := safeJoin(base, "../../etc/passwd"); err == nil {
		t.Error("safeJoin debería rechazar rutas con traversal")
	}

	got, err := safeJoin(base, "sub/archivo.txt")
	if err != nil {
		t.Fatalf("safeJoin con ruta válida falló: %v", err)
	}
	if want := filepath.Join(base, "sub", "archivo.txt"); got != want {
		t.Errorf("safeJoin = %q, quiero %q", got, want)
	}
}

func TestSafeJoinRejectsAbsoluteEscape(t *testing.T) {
	base := filepath.Join(t.TempDir(), "base")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := safeJoin(base, "../base-evil/x"); err == nil {
		t.Error("safeJoin debería rechazar salir del prefijo base")
	}
}

func TestWrappersContent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	manager, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(manager.BinDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := manager.writeWrappers(true); err != nil {
		t.Fatalf("writeWrappers: %v", err)
	}

	for _, name := range []string{"d8", "apksigner", "kotlinc"} {
		path := filepath.Join(manager.BinDir(), name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("wrapper %s: %v", name, err)
		}
		if len(data) == 0 || data[0] != '#' {
			t.Errorf("wrapper %s no empieza con shebang: %q", name, data)
		}
	}

	// En perfil simple no debe crearse el wrapper de kotlinc.
	if err := os.RemoveAll(manager.BinDir()); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(manager.BinDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := manager.writeWrappers(false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(manager.BinDir(), "kotlinc")); err == nil {
		t.Error("writeWrappers(false) no debería crear kotlinc")
	}
}

func writeZipFile(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	w := zip.NewWriter(f)
	for name, content := range entries {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		if filepath.Base(name) == "kotlinc" {
			header.SetMode(0o755)
		}
		entry, err := w.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}
