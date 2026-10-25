package cli

import (
	"os"
	"reflect"
	"testing"

	"github.com/itskreisler/rain/internal/mavenrepo"
)

func chdirTemp(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
}

func TestResolvedCacheHitEInvalidacion(t *testing.T) {
	chdirTemp(t)

	deps := []string{"androidx.core:core:1.12.0", "androidx.compose.ui:ui:1.6.0"}
	coords := []string{"androidx.core:core:1.12.0", "androidx.compose.ui:ui:1.6.0", "androidx.annotation:annotation:1.7.1"}

	if err := saveResolvedCache("deps", deps, coords); err != nil {
		t.Fatalf("saveResolvedCache: %v", err)
	}

	got, ok := loadResolvedCache("deps", deps)
	if !ok {
		t.Fatal("se esperaba un acierto de caché")
	}
	if !reflect.DeepEqual(got, coords) {
		t.Fatalf("coordenadas = %v, quiero %v", got, coords)
	}

	if _, ok := loadResolvedCache("deps", []string{"androidx.core:core:1.13.0"}); ok {
		t.Fatal("un cambio de dependencias no debe reutilizar la caché")
	}
	if _, ok := loadResolvedCache("kapt", deps); ok {
		t.Fatal("la caché de kapt es independiente de la de dependencias")
	}
}

func TestResolvedCacheCorrupta(t *testing.T) {
	chdirTemp(t)

	path := resolvedCachePath("deps")
	if err := os.MkdirAll("cache", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{esto no es json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, ok := loadResolvedCache("deps", []string{"a:b:1"}); ok {
		t.Fatal("un JSON roto no debe usarse")
	}
}

func TestResolvedCacheVersionDistinta(t *testing.T) {
	chdirTemp(t)

	deps := []string{"a:b:1"}
	if err := saveResolvedCache("deps", deps, []string{"a:b:1"}); err != nil {
		t.Fatalf("saveResolvedCache: %v", err)
	}
	// Simular un caché de una versión anterior del resolvedor.
	if err := os.WriteFile(resolvedCachePath("deps"), []byte(`{"version":0,"key":"x","coordinates":["a:b:1"]}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, ok := loadResolvedCache("deps", deps); ok {
		t.Fatal("una versión distinta del resolvedor debe invalidar la caché")
	}
}

func TestResolveCachedEvitaElResolvedorEnUnAcierto(t *testing.T) {
	chdirTemp(t)

	deps := []string{"a:b:1"}
	calls := 0
	resolve := func([]string) ([]string, error) {
		calls++
		return []string{"a:b:1", "c:d:2"}, nil
	}

	first, err := resolveCached("deps", deps, resolve)
	if err != nil {
		t.Fatalf("primera resolución: %v", err)
	}
	second, err := resolveCached("deps", deps, resolve)
	if err != nil {
		t.Fatalf("segunda resolución: %v", err)
	}
	if calls != 1 {
		t.Fatalf("el resolvedor se llamó %d veces, quiero 1", calls)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("resultados distintos: %v vs %v", first, second)
	}
}

func TestResolvedCacheSeInvalidaSiCambianLosRepositorios(t *testing.T) {
	chdirTemp(t)

	previous := mavenrepo.ExtraRepositories
	t.Cleanup(func() { mavenrepo.ExtraRepositories = previous })

	deps := []string{"a:b:1"}
	calls := 0
	resolve := func([]string) ([]string, error) {
		calls++
		return []string{"a:b:1"}, nil
	}

	mavenrepo.ExtraRepositories = []string{"https://nexus.local/repo"}
	if _, err := resolveCached("deps", deps, resolve); err != nil {
		t.Fatalf("primera resolución: %v", err)
	}

	mavenrepo.ExtraRepositories = []string{"https://otro.local/repo"}
	if _, err := resolveCached("deps", deps, resolve); err != nil {
		t.Fatalf("segunda resolución: %v", err)
	}
	if calls != 2 {
		t.Fatalf("el resolvedor se llamó %d veces, quiero 2", calls)
	}
}
