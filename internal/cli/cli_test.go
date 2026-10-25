package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/itskreisler/rain/internal/config"
	"github.com/itskreisler/rain/internal/doctor"
)

// runCmd ejecuta el árbol de comandos con args, capturando stdout, y devuelve
// la salida y el error (sin os.Exit).
func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd(BuildInfo{Version: "test", Commit: "abc", Date: "ayer"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestVersionCommand(t *testing.T) {
	out, err := runCmd(t, "version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	for _, want := range []string{"rain test", "commit:   abc", "built:    ayer", "platform:"} {
		if !strings.Contains(out, want) {
			t.Errorf("salida de version no contiene %q:\n%s", want, out)
		}
	}
}

func TestVersionFlag(t *testing.T) {
	out, err := runCmd(t, "--version")
	if err != nil {
		t.Fatalf("--version: %v", err)
	}
	if !strings.Contains(out, "rain test") {
		t.Errorf("--version = %q, quiero la versión del build", out)
	}
}

func TestInitWritesConfig(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	out, err := runCmd(t, "init", "--non-interactive", "--mode", "simple", "--name", "Demo", "--package", "com.demo", "--version", "3.2")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if !strings.Contains(out, "creado en") {
		t.Errorf("salida de init = %q", out)
	}

	cfg, err := config.Read(filepath.Join(dir, "rain.toml"))
	if err != nil {
		t.Fatalf("leyendo rain.toml: %v", err)
	}
	if cfg.Project.Name != "Demo" || cfg.Project.Package != "com.demo" || cfg.Project.Version != "3.2" {
		t.Errorf("Project = %+v", cfg.Project)
	}
	if cfg.Mode != "simple" {
		t.Errorf("Mode = %q, quiero simple", cfg.Mode)
	}

	// La plantilla simple crea el proyecto completo, no solo rain.toml.
	for _, rel := range []string{
		"app/AndroidManifest.xml",
		"app/res/values/strings.xml",
		"app/res/values/themes.xml",
		filepath.Join("app", "java", "com", "demo", "MainActivity.java"),
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("falta %s tras init: %v", rel, err)
		}
	}

	manifest, err := os.ReadFile(filepath.Join(dir, "app", "AndroidManifest.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifest), `package="com.demo"`) {
		t.Errorf("manifest sin package:\n%s", manifest)
	}
	if !strings.Contains(string(manifest), `android:name=".MainActivity"`) {
		t.Errorf("manifest sin actividad:\n%s", manifest)
	}
}

func TestInitCompletoScaffoldsCompose(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	out, err := runCmd(t, "init", "--non-interactive", "--mode", "completo", "--name", "Compose Demo")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if !strings.Contains(out, "completo") {
		t.Errorf("salida de init = %q", out)
	}

	cfg, err := config.Read(filepath.Join(dir, "rain.toml"))
	if err != nil {
		t.Fatalf("leyendo rain.toml: %v", err)
	}
	if cfg.Mode != "completo" {
		t.Errorf("Mode = %q, quiero completo", cfg.Mode)
	}
	if len(cfg.Dependencies) == 0 {
		t.Error("la plantilla completo debería declarar dependencias de Compose")
	}

	// El nombre con espacios produce un package válido derivado.
	if !strings.HasPrefix(cfg.Project.Package, "com.rain.") {
		t.Errorf("Package = %q, quiero prefijo com.rain.", cfg.Project.Package)
	}

	kt := filepath.Join(dir, "app", "java", filepath.FromSlash(strings.ReplaceAll(cfg.Project.Package, ".", "/")), "MainActivity.kt")
	src, err := os.ReadFile(kt)
	if err != nil {
		t.Fatalf("leyendo MainActivity.kt: %v", err)
	}
	if !strings.Contains(string(src), "setContent") || !strings.Contains(string(src), "@Composable") {
		t.Errorf("la plantilla completo debería usar Compose:\n%s", src)
	}
}

func TestInitNonInteractiveCreatesDefaultPackage(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	if _, err := runCmd(t, "init", "--non-interactive", "--name", "1 App"); err != nil {
		t.Fatalf("init: %v", err)
	}
	cfg, err := config.Read(filepath.Join(dir, "rain.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Project.Package != "com.rain.app1app" {
		t.Errorf("Package = %q, quiero com.rain.app1app", cfg.Project.Package)
	}
}

func TestInitDirFlag(t *testing.T) {
	base := t.TempDir()
	chdir(t, base)

	out, err := runCmd(t, "init", "--non-interactive", "--name", "Sub", "--dir", filepath.Join("sub", "app"))
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if !strings.Contains(out, filepath.Join("sub", "app")) {
		t.Errorf("salida de init = %q", out)
	}
	if _, err := os.Stat(filepath.Join(base, "sub", "app", "rain.toml")); err != nil {
		t.Errorf("rain.toml no creado en sub/app: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "sub", "app", "app", "AndroidManifest.xml")); err != nil {
		t.Errorf("manifest no creado en sub/app/app: %v", err)
	}
}

func TestInitInvalidPackageIsUsageError(t *testing.T) {
	chdir(t, t.TempDir())

	_, err := runCmd(t, "init", "--non-interactive", "--package", "com..malo")
	if err == nil {
		t.Fatal("un package inválido debería fallar")
	}
	if code := ExitCode(err); code != 2 {
		t.Errorf("ExitCode = %d, quiero 2 (error de uso)", code)
	}
}

func TestDefaultPackageAndValidate(t *testing.T) {
	cases := []struct {
		name, want string
	}{
		{"MiApp", "com.rain.miapp"},
		{"Hello Rain", "com.rain.hellorain"},
		{"1App", "com.rain.app1app"},
		{"!!!", "com.rain.miapp"},
	}
	for _, tc := range cases {
		if got := defaultPackage(tc.name); got != tc.want {
			t.Errorf("defaultPackage(%q) = %q, quiero %q", tc.name, got, tc.want)
		}
	}

	valid := []string{"com.demo", "a.b.c", "com.example.app_1", "x"}
	for _, pkg := range valid {
		if err := validatePackage(pkg); err != nil {
			t.Errorf("validatePackage(%q) = %v, quiero nil", pkg, err)
		}
	}
	invalid := []string{"", "com..malo", ".com", "com.", "com.1app", "com.mi-app"}
	for _, pkg := range invalid {
		if err := validatePackage(pkg); err == nil {
			t.Errorf("validatePackage(%q) = nil, quiero error", pkg)
		}
	}
}

func TestInitDefaultsNonInteractive(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	if _, err := runCmd(t, "init", "--non-interactive"); err != nil {
		t.Fatalf("init: %v", err)
	}

	cfg, err := config.Read(filepath.Join(dir, "rain.toml"))
	if err != nil {
		t.Fatalf("leyendo rain.toml: %v", err)
	}
	if cfg.Project.Name != "MiApp" || cfg.Mode != "simple" {
		t.Errorf("valores por defecto = %+v (mode %q)", cfg.Project, cfg.Mode)
	}
}

func TestInitInteractiveChoiceCompleto(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	root := NewRootCmd(BuildInfo{Version: "test"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader("2\n"))
	root.SetArgs([]string{"init"})
	if err := root.Execute(); err != nil {
		t.Fatalf("init interactivo: %v", err)
	}

	cfg, err := config.Read(filepath.Join(dir, "rain.toml"))
	if err != nil {
		t.Fatalf("leyendo rain.toml: %v", err)
	}
	if cfg.Mode != "completo" {
		t.Errorf("Mode = %q, quiero completo tras elegir 2", cfg.Mode)
	}
}

func TestInitDoesNotOverwriteWithoutForce(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	if _, err := runCmd(t, "init", "--non-interactive", "--name", "Primero"); err != nil {
		t.Fatalf("init inicial: %v", err)
	}

	out, err := runCmd(t, "init", "--non-interactive", "--name", "Segundo")
	if err != nil {
		t.Fatalf("init segundo: %v", err)
	}
	if !strings.Contains(out, "ya existe") {
		t.Errorf("se esperaba aviso de que ya existe:\n%s", out)
	}

	cfg, _ := config.Read(filepath.Join(dir, "rain.toml"))
	if cfg.Project.Name != "Primero" {
		t.Errorf("Name = %q, no debería haberse sobrescrito", cfg.Project.Name)
	}

	if _, err := runCmd(t, "init", "--non-interactive", "--name", "Segundo", "--force"); err != nil {
		t.Fatalf("init --force: %v", err)
	}
	cfg, _ = config.Read(filepath.Join(dir, "rain.toml"))
	if cfg.Project.Name != "Segundo" {
		t.Errorf("Name = %q, --force debería sobrescribir", cfg.Project.Name)
	}
}

func TestInitInvalidModeIsUsageError(t *testing.T) {
	chdir(t, t.TempDir())

	_, err := runCmd(t, "init", "--mode", "inventado")
	if err == nil {
		t.Fatal("init con modo inválido debería fallar")
	}
	if code := ExitCode(err); code != 2 {
		t.Errorf("ExitCode = %d, quiero 2 (error de uso)", code)
	}
}

func TestCleanRemovesArtifacts(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	for _, sub := range []string{"libs", "cache", filepath.Join("app", "build")} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Un módulo personalizado declarado en rain.toml.
	cfg := config.Config{
		Project: config.Project{Name: "x"},
		Modules: []config.Module{{Name: "core", Path: "core"}},
	}
	if err := config.Write(filepath.Join(dir, "rain.toml"), cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "core", "build"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := runCmd(t, "clean"); err != nil {
		t.Fatalf("clean: %v", err)
	}

	for _, sub := range []string{"libs", "cache", filepath.Join("core", "build")} {
		if _, err := os.Stat(filepath.Join(dir, sub)); !os.IsNotExist(err) {
			t.Errorf("%s debería haberse borrado", sub)
		}
	}
	// app/build no se declara en rain.toml (módulos: core), así que no se toca.
	if _, err := os.Stat(filepath.Join(dir, "app", "build")); err != nil {
		t.Errorf("app/build no debería borrarse si no está en los módulos: %v", err)
	}
}

func TestDoctorUnknownProfileIsUsageError(t *testing.T) {
	_, err := runCmd(t, "doctor", "--profile", "inventado")
	if err == nil {
		t.Fatal("doctor con perfil inválido debería fallar")
	}
	if code := ExitCode(err); code != 2 {
		t.Errorf("ExitCode = %d, quiero 2", code)
	}
}

func TestDoctorJSONIsValidAndVersioned(t *testing.T) {
	out, err := runCmd(t, "doctor", "--json", "--profile", "simple")
	// Puede faltar algo en el entorno de test; el exit code 3 es aceptable.
	if err != nil && ExitCode(err) != 3 {
		t.Fatalf("doctor --json: %v", err)
	}

	var report doctor.Report
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("salida JSON inválida: %v\n%s", err, out)
	}
	if report.SchemaVersion == "" {
		t.Error("el reporte JSON debe incluir SchemaVersion")
	}
	if report.Profile != doctor.Simple {
		t.Errorf("Profile = %q, quiero simple", report.Profile)
	}
	if len(report.Checks) == 0 {
		t.Error("el reporte debería incluir checks")
	}
}

func TestDoctorPrintInstall(t *testing.T) {
	out, err := runCmd(t, "doctor", "--print-install")
	if err != nil && ExitCode(err) != 3 {
		t.Fatalf("doctor --print-install: %v", err)
	}
	if !strings.Contains(out, "Comandos recomendados") {
		t.Errorf("salida = %q, quiero comandos recomendados", out)
	}
}

func TestExitCodeDefault(t *testing.T) {
	if code := ExitCode(nil); code != 1 {
		t.Errorf("ExitCode(nil) = %d, quiero 1", code)
	}
	if code := ExitCode(errors.New("x")); code != 1 {
		t.Errorf("ExitCode(error normal) = %d, quiero 1", code)
	}
	if code := ExitCode(&exitError{code: 3, err: errors.New("x")}); code != 3 {
		t.Errorf("ExitCode(exitError 3) = %d, quiero 3", code)
	}
}

func TestNormalizeLegacyArgs(t *testing.T) {
	tests := []struct {
		in   []string
		want []string
	}{
		{[]string{"-release"}, []string{"--release"}},
		{[]string{"-android-jar", "/x.jar"}, []string{"--android-jar", "/x.jar"}},
		{[]string{"--release"}, []string{"--release"}},
		{[]string{"-mode=completo"}, []string{"--mode=completo"}},
		{[]string{"-x"}, []string{"-x"}},
		{[]string{"build", "-release"}, []string{"build", "--release"}},
	}
	for _, tc := range tests {
		got := normalizeLegacyArgs(tc.in)
		if strings.Join(got, " ") != strings.Join(tc.want, " ") {
			t.Errorf("normalizeLegacyArgs(%v) = %v, quiero %v", tc.in, got, tc.want)
		}
	}
}

func TestBuildRequiresProject(t *testing.T) {
	chdir(t, t.TempDir())

	// Sin AndroidManifest.xml en ningún sitio, build debe fallar con error claro.
	_, err := runCmd(t, "build", ".")
	if err == nil {
		t.Fatal("build sin proyecto debería fallar")
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}
