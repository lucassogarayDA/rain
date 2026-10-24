package aar

import (
	"os"
	"path/filepath"
	"testing"
)

// makeLib crea una dependencia preparada con los ficheros que la caché
// comprueba (classes.jar, manifest, reglas de proguard y directorio de res).
func makeLib(t *testing.T, dir, artifact, version string) Library {
	t.Helper()
	classJar := filepath.Join(dir, "jars", artifact+"-"+version+".jar")
	manifest := filepath.Join(dir, "manifests", artifact+"-"+version+".xml")
	proguard := filepath.Join(dir, "proguard", artifact+"-"+version+".pro")
	resDir := filepath.Join(dir, "res", artifact+"-"+version)

	for _, path := range []string{classJar, manifest, proguard} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("creando %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte("contenido"), 0o644); err != nil {
			t.Fatalf("escribiendo %s: %v", path, err)
		}
	}
	if err := os.MkdirAll(resDir, 0o755); err != nil {
		t.Fatalf("creando %s: %v", resDir, err)
	}

	return Library{
		Coordinate:    artifact + ":" + version,
		ClassJar:      classJar,
		ResDir:        resDir,
		Package:       artifact,
		ManifestPath:  manifest,
		ProguardRules: proguard,
	}
}

func TestSaveYLoadCache(t *testing.T) {
	dir := t.TempDir()
	lib := makeLib(t, dir, "core", "1.0")
	set := &Set{Dir: dir, Libraries: []Library{lib}}
	coords := []string{"androidx.core:core:1.0"}

	if err := SaveCache(dir, set, coords); err != nil {
		t.Fatalf("SaveCache: %v", err)
	}

	loaded, ok := LoadCache(dir, coords)
	if !ok {
		t.Fatal("LoadCache debería encontrar la caché recién guardada")
	}
	if len(loaded.Libraries) != 1 {
		t.Fatalf("librerías = %d, quiero 1", len(loaded.Libraries))
	}
	if loaded.Libraries[0].ClassJar != lib.ClassJar {
		t.Errorf("classes.jar = %q, quiero %q", loaded.Libraries[0].ClassJar, lib.ClassJar)
	}
	if loaded.Libraries[0].Package != "core" {
		t.Errorf("paquete = %q, quiero core", loaded.Libraries[0].Package)
	}
}

func TestLoadCacheConCoordenadasDistintas(t *testing.T) {
	dir := t.TempDir()
	set := &Set{Dir: dir, Libraries: []Library{makeLib(t, dir, "core", "1.0")}}
	if err := SaveCache(dir, set, []string{"androidx.core:core:1.0"}); err != nil {
		t.Fatalf("SaveCache: %v", err)
	}

	if _, ok := LoadCache(dir, []string{"androidx.core:core:1.1"}); ok {
		t.Error("la caché no debe servir si cambian las coordenadas")
	}
}

func TestLoadCacheIgnoraElOrdenDeLasCoordenadas(t *testing.T) {
	dir := t.TempDir()
	set := &Set{Dir: dir, Libraries: []Library{makeLib(t, dir, "core", "1.0")}}
	coords := []string{"a:1:1", "b:2:2"}
	if err := SaveCache(dir, set, coords); err != nil {
		t.Fatalf("SaveCache: %v", err)
	}

	// Las mismas coordenadas en otro orden describen el mismo conjunto.
	if _, ok := LoadCache(dir, []string{"b:2:2", "a:1:1"}); !ok {
		t.Error("el orden de las coordenadas no debe invalidar la caché")
	}
}

func TestLoadCacheSiFaltaUnFichero(t *testing.T) {
	dir := t.TempDir()
	lib := makeLib(t, dir, "core", "1.0")
	set := &Set{Dir: dir, Libraries: []Library{lib}}
	coords := []string{"androidx.core:core:1.0"}
	if err := SaveCache(dir, set, coords); err != nil {
		t.Fatalf("SaveCache: %v", err)
	}

	// Si alguien borra el jar, la caché ya no vale.
	if err := os.Remove(lib.ClassJar); err != nil {
		t.Fatalf("borrando %s: %v", lib.ClassJar, err)
	}
	if _, ok := LoadCache(dir, coords); ok {
		t.Error("la caché no debe servir si falta un fichero de la dependencia")
	}
}

func TestLoadCacheSinCopia(t *testing.T) {
	if _, ok := LoadCache(t.TempDir(), []string{"a:1:1"}); ok {
		t.Error("no debería haber caché en un directorio vacío")
	}
}

func TestInvalidateCache(t *testing.T) {
	dir := t.TempDir()
	set := &Set{Dir: dir, Libraries: []Library{makeLib(t, dir, "core", "1.0")}}
	coords := []string{"androidx.core:core:1.0"}
	if err := SaveCache(dir, set, coords); err != nil {
		t.Fatalf("SaveCache: %v", err)
	}
	if err := InvalidateCache(dir); err != nil {
		t.Fatalf("InvalidateCache: %v", err)
	}
	if _, ok := LoadCache(dir, coords); ok {
		t.Error("la caché debería estar invalidada")
	}
	// Invalidar una caché que no existe no es un error.
	if err := InvalidateCache(dir); err != nil {
		t.Errorf("InvalidateCache sobre una caché inexistente: %v", err)
	}
}
