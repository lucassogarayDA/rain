package doctor

import (
	"archive/zip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseProfile(t *testing.T) {
	tests := []struct {
		in      string
		want    Profile
		wantErr bool
	}{
		{"simple", Simple, false},
		{"complex", Complex, false},
		{"  SIMPLE  ", Simple, false},
		{"ComPlex", Complex, false},
		{"", "", true},
		{"completo", "", true},
		{"simple ", Simple, false},
	}
	for _, tc := range tests {
		got, err := ParseProfile(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("ParseProfile(%q) error = %v, wantErr %v", tc.in, err, tc.wantErr)
			continue
		}
		if !tc.wantErr && got != tc.want {
			t.Errorf("ParseProfile(%q) = %q, quiero %q", tc.in, got, tc.want)
		}
	}
}

func TestMajorOf(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"17.0.20", 17},
		{"21", 21},
		{"1.8.0_392", 8},
		{"25.0.4", 25},
		{"", 0},
		{"no-es-version", 0},
	}
	for _, tc := range tests {
		if got := majorOf(tc.in); got != tc.want {
			t.Errorf("majorOf(%q) = %d, quiero %d", tc.in, got, tc.want)
		}
	}
}

func TestParseJavaVersion(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{`openjdk version "17.0.20" 2026-07-21`, "17.0.20"},
		{`openjdk version "1.8.0_392"`, "1.8.0_392"},
		{`java 21.0.1 2024-01-16`, "21.0.1"},
		{"", ""},
	}
	for _, tc := range tests {
		if got := parseJavaVersion(tc.in); got != tc.want {
			t.Errorf("parseJavaVersion(%q) = %q, quiero %q", tc.in, got, tc.want)
		}
	}
}

func TestFirstVersion(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Android Asset Packaging Tool (aapt) 2.20-android-16.0.0_r2", "2.20"},
		{"kotlinc-jvm 2.4.21 (JRE 17.0.20+8)", "2.4.21"},
		{"Zip 3.0", "3.0"},
		{"sin numeros", ""},
	}
	for _, tc := range tests {
		if got := firstVersion(tc.in); got != tc.want {
			t.Errorf("firstVersion(%q) = %q, quiero %q", tc.in, got, tc.want)
		}
	}
}

func TestValidAndroidJar(t *testing.T) {
	t.Run("jar válido", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "android.jar")
		writeJar(t, path, []string{"android/Manifest.class", "android/R.class"})
		if !validAndroidJar(path) {
			t.Error("validAndroidJar debería aceptar un android.jar con android/Manifest.class")
		}
	})

	t.Run("jar con R clásico", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "android.jar")
		writeJar(t, path, []string{"android/R.class"})
		if !validAndroidJar(path) {
			t.Error("validAndroidJar debería aceptar android/R.class")
		}
	})

	t.Run("zip que no es android.jar", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "otro.jar")
		writeJar(t, path, []string{"com/ejemplo/Main.class"})
		if validAndroidJar(path) {
			t.Error("validAndroidJar no debería aceptar un jar cualquiera")
		}
	})

	t.Run("archivo que no es zip", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "roto.jar")
		if err := os.WriteFile(path, []byte("no soy un zip"), 0o644); err != nil {
			t.Fatal(err)
		}
		if validAndroidJar(path) {
			t.Error("validAndroidJar no debería aceptar un archivo corrupto")
		}
	})
}

func TestFindAndroidJarUsesEnv(t *testing.T) {
	dir := t.TempDir()
	jar := filepath.Join(dir, "android.jar")
	writeJar(t, jar, []string{"android/Manifest.class"})

	t.Setenv("RAIN_ANDROID_JAR", jar)

	got, ok := FindAndroidJar()
	if !ok {
		t.Fatal("FindAndroidJar debería encontrar el jar de RAIN_ANDROID_JAR")
	}
	if got != jar {
		t.Errorf("FindAndroidJar = %q, quiero %q", got, jar)
	}
}

func TestFindAndroidJarInvalidReturnsPath(t *testing.T) {
	dir := t.TempDir()
	jar := filepath.Join(dir, "android.jar")
	if err := os.WriteFile(jar, []byte("no zip"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("RAIN_ANDROID_JAR", jar)
	t.Setenv("ANDROID_JAR", "")

	// Aísla el HOME para no encontrar un android.jar válido del sistema.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ANDROID_HOME", "")
	t.Setenv("ANDROID_SDK_ROOT", "")

	got, ok := FindAndroidJar()
	if ok {
		t.Fatal("FindAndroidJar no debería marcar ok un jar inválido")
	}
	if got != jar {
		t.Errorf("FindAndroidJar = %q, quiero la ruta inválida %q para diagnosticar", got, jar)
	}
}

func TestDetectPlatform(t *testing.T) {
	got := DetectPlatform()
	switch got {
	case Alpine, Termux, Ubuntu, Generic:
	default:
		t.Errorf("DetectPlatform devolvió %q, que no es una plataforma conocida", got)
	}

	if runtime.GOOS == "linux" {
		t.Setenv("PREFIX", "/data/data/com.termux/files/usr")
		if got := DetectPlatform(); got != Termux {
			t.Errorf("DetectPlatform con PREFIX de Termux = %q, quiero termux", got)
		}
	}
}

func TestRecommendations(t *testing.T) {
	t.Run("alpine simple", func(t *testing.T) {
		cmds := Recommendations(Alpine, Simple)
		if len(cmds) == 0 {
			t.Fatal("Recommendations no debería estar vacío")
		}
		if !containsSubstring(cmds, "apk add") {
			t.Errorf("se esperaba un comando apk add, got %v", cmds)
		}
	})

	t.Run("alpine complex añade fix", func(t *testing.T) {
		cmds := Recommendations(Alpine, Complex)
		if !containsSubstring(cmds, "--profile complex") {
			t.Errorf("se esperaba --profile complex, got %v", cmds)
		}
	})

	t.Run("ubuntu simple", func(t *testing.T) {
		cmds := Recommendations(Ubuntu, Simple)
		if !containsSubstring(cmds, "apt") {
			t.Errorf("se esperaba un comando apt, got %v", cmds)
		}
	})

	t.Run("termux complex instala kotlin", func(t *testing.T) {
		cmds := Recommendations(Termux, Complex)
		if !containsSubstring(cmds, "kotlin") {
			t.Errorf("se esperaba kotlin, got %v", cmds)
		}
	})
}

func TestMissingDetail(t *testing.T) {
	if got := missingDetail(""); got != "no encontrado" {
		t.Errorf("missingDetail(\"\") = %q", got)
	}
	if got := missingDetail("/usr/bin/java"); got != "" {
		t.Errorf("missingDetail(ruta) = %q, quiero vacío", got)
	}
}

func TestJavaToolDetail(t *testing.T) {
	if got := javaToolDetail("", true, true); got != "no encontrado" {
		t.Errorf("javaToolDetail sin ruta = %q", got)
	}
	if got := javaToolDetail("/x/d8", false, false); got != "requiere Java 17+" {
		t.Errorf("javaToolDetail sin java = %q", got)
	}
	if got := javaToolDetail("/x/d8", true, true); got != "" {
		t.Errorf("javaToolDetail ok = %q, quiero vacío", got)
	}
}

func TestJarCandidatesIncludesEnv(t *testing.T) {
	t.Setenv("ANDROID_HOME", "/tmp/sdk")
	t.Setenv("RAIN_ANDROID_JAR", "/tmp/custom.jar")

	candidates := androidJarCandidates()
	if !containsSubstring(candidates, "/tmp/custom.jar") {
		t.Errorf("androidJarCandidates no incluye RAIN_ANDROID_JAR: %v", candidates)
	}
	if !containsSubstring(candidates, filepath.Join("/tmp/sdk", "platforms", "android-36", "android.jar")) {
		t.Errorf("androidJarCandidates no incluye ANDROID_HOME: %v", candidates)
	}
}

// fakeCommand crea un ejecutable en un directorio temporal.
func fakeCommand(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("escribiendo %s: %v", path, err)
	}
	return path
}

func TestRunVersionIgnoraLosComandosQueFallan(t *testing.T) {
	dir := t.TempDir()
	fakeCommand(t, dir, "falso", "#!/bin/sh\nexit 1\n")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	path, _ := runVersion("falso", "--version")
	if path != "" {
		t.Errorf("runVersion con un comando que falla = %q, want \"\"", path)
	}
}

func TestRunVersionAceptaLosComandosQueFuncionan(t *testing.T) {
	dir := t.TempDir()
	fakeCommand(t, dir, "bueno", "#!/bin/sh\necho 1.2.3\n")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	path, out := runVersion("bueno", "--version")
	if path == "" {
		t.Fatal("runVersion devolvió una ruta vacía para un comando que funciona")
	}
	if out != "1.2.3" {
		t.Errorf("runVersion devolvió %q, want %q", out, "1.2.3")
	}
}

func TestDiagnoseNoCuentaUnLanzadorRotoComoDisponible(t *testing.T) {
	// Rain antepone ~/.rain/bin al PATH. Si el toolchain no está descargado,
	// los lanzadores existen pero fallan al ejecutarse: d8 y apksigner no
	// deben aparecer como disponibles.
	//
	// Se aísla el HOME para que el toolchain gestionado (~/.rain/toolchain)
	// no acted como fallback: sin él, el único d8 es el lanzador roto.
	dir := t.TempDir()
	fakeCommand(t, dir, "d8", "#!/bin/sh\nexit 1\n")
	fakeCommand(t, dir, "apksigner", "#!/bin/sh\nexit 127\n")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HOME", t.TempDir())

	report := Diagnose(Complex)
	for _, check := range report.Checks {
		switch check.Name {
		case "d8", "apksigner":
			if check.OK {
				t.Errorf("el check %q aparece como disponible con un lanzador roto", check.Name)
			}
		}
	}
	if report.Ready {
		t.Error("el informe dice que está listo con d8 y apksigner rotos")
	}
	if !strings.Contains(strings.Join(report.Missing, ","), "d8") {
		t.Errorf("Missing = %v, se esperaba d8", report.Missing)
	}
}

func writeJar(t *testing.T, path string, entries []string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	w := zip.NewWriter(f)
	for _, name := range entries {
		entry, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte("contenido")); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func containsSubstring(values []string, substr string) bool {
	for _, value := range values {
		if len(value) >= len(substr) {
			for i := 0; i+len(substr) <= len(value); i++ {
				if value[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}
