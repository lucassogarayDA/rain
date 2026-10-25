package build

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/itskreisler/rain/internal/aar"
)

// TestLibraryJarsIgnoraLosArtefactosRanciosDelCache comprueba que el classpath
// se construye solo con las dependencias resueltas.
//
// El caché de deps se reutiliza entre builds, así que guarda artefactos de
// resoluciones anteriores (p. ej. una versión vieja de androidx.core que ya no
// está en el grafo). Si esos ficheros entraran en el classpath, el APK
// dependería del historial de la caché y dos builds "iguales" con distinta
// caché darían APKs distintos.
func TestLibraryJarsIgnoraLosArtefactosRanciosDelCache(t *testing.T) {
	deps := t.TempDir()
	resuelto := filepath.Join(deps, "jars", "core-1.19.1.jar")
	writeFile(t, resuelto, "resuelto")
	// Restos de una resolución anterior.
	writeFile(t, filepath.Join(deps, "jars", "core-1.18.0.jar"), "rancio")
	writeFile(t, filepath.Join(deps, "aars", "jspecify-1.0.0.jar"), "rancio")

	cfg := Config{
		Libs: &aar.Set{
			Dir: deps,
			Libraries: []aar.Library{
				{Coordinate: "androidx.core:core:1.19.1", ClassJar: resuelto},
			},
		},
		DepsDir: deps,
	}

	got := cfg.libraryJars()
	if !slices.Equal(got, []string{resuelto}) {
		t.Errorf("libraryJars = %v, quiero solo %v (el caché no debe filtrar jars rancios)", got, resuelto)
	}
}

// TestLibraryJarsSinSetUsaElDirectorioDeDependencias comprueba que, cuando no
// hay set explícito, los jars del directorio de dependencias sí se usan.
func TestLibraryJarsSinSetUsaElDirectorioDeDependencias(t *testing.T) {
	deps := t.TempDir()
	enJars := filepath.Join(deps, "jars", "a.jar")
	enAars := filepath.Join(deps, "aars", "b.jar")
	writeFile(t, enJars, "a")
	writeFile(t, enAars, "b")

	cfg := Config{DepsDir: deps}

	got := cfg.libraryJars()
	for _, want := range []string{enJars, enAars} {
		if !slices.Contains(got, want) {
			t.Errorf("libraryJars = %v, faltaba %s", got, want)
		}
	}
}
