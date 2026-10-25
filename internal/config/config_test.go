package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWriteReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rain.toml")

	original := Config{
		Project:      Project{Name: "Demo", Package: "com.demo.app", Version: "2.1"},
		Build:        Build{Debug: true},
		Mode:         "completo",
		Dependencies: []string{"androidx.core:core:1.12.0"},
		Modules:      []Module{{Name: "app", Path: "app"}},
	}
	if err := Write(path, original); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if got.Project != original.Project {
		t.Errorf("Project = %+v, quiero %+v", got.Project, original.Project)
	}
	if got.Mode != original.Mode {
		t.Errorf("Mode = %q, quiero %q", got.Mode, original.Mode)
	}
	if len(got.Dependencies) != 1 || got.Dependencies[0] != original.Dependencies[0] {
		t.Errorf("Dependencies = %v, quiero %v", got.Dependencies, original.Dependencies)
	}
	if len(got.Modules) != 1 || got.Modules[0] != original.Modules[0] {
		t.Errorf("Modules = %v, quiero %v", got.Modules, original.Modules)
	}
	if got.Signing != nil {
		t.Errorf("Signing = %+v, quiero nil", got.Signing)
	}
}

func TestReadMissingFile(t *testing.T) {
	if _, err := Read(filepath.Join(t.TempDir(), "noexiste.toml")); err == nil {
		t.Fatal("Read con archivo inexistente debería fallar")
	}
}

func TestReadInvalidTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rain.toml")
	if err := os.WriteFile(path, []byte("esto no es = toml valido ["), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(path); err == nil {
		t.Fatal("Read con TOML inválido debería fallar")
	}
}

func TestSigningRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rain.toml")

	original := Config{
		Project: Project{Name: "Firma", Package: "com.firma", Version: "1"},
		Signing: &Signing{
			StoreFile:     "release.keystore",
			StorePassword: "secreto",
			KeyAlias:      "release",
			KeyPassword:   "otro",
		},
	}
	if err := Write(path, original); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Signing == nil {
		t.Fatal("Signing quedó nil tras el round-trip")
	}
	if *got.Signing != *original.Signing {
		t.Errorf("Signing = %+v, quiero %+v", *got.Signing, *original.Signing)
	}
}

func TestConfigBuildFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rain.toml")

	original := Config{
		Project: Project{Name: "Demo", Package: "com.demo.app", Version: "2.1"},
		Build: Build{
			Debug:         true,
			MinSdk:        24,
			TargetSdk:     34,
			VersionCode:   2,
			VersionName:   "2.1",
			ApplicationID: "com.demo.app.id",
			JavaTarget:    17,
		},
	}
	if err := Write(path, original); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	// El TOML no distingue la lista vacía de la ausente: al leer, las listas
	// vacías vienen como [] y no como nil. Se comparan como iguales.
	gotBuild, wantBuild := got.Build, original.Build
	for _, p := range []*[]string{
		&gotBuild.ProguardFiles, &wantBuild.ProguardFiles,
		&gotBuild.DebugDependencies, &wantBuild.DebugDependencies,
		&gotBuild.ReleaseDependencies, &wantBuild.ReleaseDependencies,
	} {
		if len(*p) == 0 {
			*p = nil
		}
	}
	if !reflect.DeepEqual(gotBuild, wantBuild) {
		t.Errorf("Build = %+v, quiero %+v", got.Build, original.Build)
	}
}

// TestReadRejectsMisplacedRootKeys comprueba que una clave raíz colocada DESPUÉS
// de una cabecera de tabla falla en lugar de ignorarse en silencio. En TOML todo
// lo que sigue a [build] pertenece a [build]; "dependencies" se perdía y el build
// continuaba sin dependencias.
func TestReadRejectsMisplacedRootKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rain.toml")

	content := `[project]
name = "Demo"
package = "com.demo.app"
version = "1.0"

[build]
debug = true

mode = "completo"

dependencies = [
    "androidx.activity:activity-compose:1.8.2",
]
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("escribiendo rain.toml: %v", err)
	}

	if _, err := Read(path); err == nil {
		t.Fatal("Read aceptó claves raíz situadas dentro de [build]; quiero error")
	}
}

// TestReadAcceptsRootKeysBeforeTables es el caso correcto: las claves raíz van
// antes de cualquier cabecera de tabla.
func TestReadAcceptsRootKeysBeforeTables(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rain.toml")

	content := `mode = "completo"

dependencies = [
    "androidx.activity:activity-compose:1.8.2",
]

[project]
name = "Demo"
package = "com.demo.app"
version = "1.0"

[build]
debug = true
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("escribiendo rain.toml: %v", err)
	}

	cfg, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if cfg.Mode != "completo" {
		t.Errorf("Mode = %q, quiero %q", cfg.Mode, "completo")
	}
	if len(cfg.Dependencies) != 1 {
		t.Fatalf("Dependencies = %v, quiero 1 dependencia", cfg.Dependencies)
	}
	if cfg.Project.Name != "Demo" {
		t.Errorf("Project.Name = %q, quiero %q", cfg.Project.Name, "Demo")
	}
}
