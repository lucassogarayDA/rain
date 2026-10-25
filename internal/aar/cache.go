package aar

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// cacheFile es el nombre del fichero donde se guarda el set preparado.
const cacheFile = ".rain-deps.json"

// coordsHash resume el conjunto de coordenadas resueltas. Si el hash cambia,
// el set guardado ya no sirve (o las versiones han cambiado).
func coordsHash(coords []string) string {
	sorted := append([]string(nil), coords...)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, "\n")))
	return hex.EncodeToString(sum[:])
}

type cachedSet struct {
	Hash  string   `json:"hash"`
	Coord []string `json:"coords"`
	Set   *Set     `json:"set"`
}

// SaveCache guarda el set preparado junto con las coordenadas que lo
// generaron. Así la siguiente ejecución puede reutilizarlo sin volver a
// descargar ni extraer los AAR (que en un proyecto con Compose son decenas y
// tardan varios segundos).
func SaveCache(dir string, set *Set, coords []string) error {
	if set == nil {
		return nil
	}
	data, err := json.MarshalIndent(cachedSet{Hash: coordsHash(coords), Coord: coords, Set: set}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, cacheFile), data, 0o644)
}

// LoadCache devuelve el set guardado si sigue siendo válido: mismas
// coordenadas y todos los ficheros que referencia siguen en disco.
func LoadCache(dir string, coords []string) (*Set, bool) {
	data, err := os.ReadFile(filepath.Join(dir, cacheFile))
	if err != nil {
		return nil, false
	}
	var cached cachedSet
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, false
	}
	if cached.Hash != coordsHash(coords) || cached.Set == nil {
		return nil, false
	}
	for _, lib := range cached.Set.Libraries {
		for _, path := range []string{lib.ClassJar, lib.ManifestPath, lib.ProguardRules} {
			if path == "" {
				continue
			}
			if st, err := os.Stat(path); err != nil || st.IsDir() {
				return nil, false
			}
		}
		for _, path := range lib.ServiceFiles {
			if st, err := os.Stat(path); err != nil || st.IsDir() {
				return nil, false
			}
		}
		if lib.ResDir != "" {
			if st, err := os.Stat(lib.ResDir); err != nil || !st.IsDir() {
				return nil, false
			}
		}
		// Un set antiguo no guardaba las rutas de las .so: sin ellas no se
		// pueden empaquetar solo las del set actual, así que se re-prepara.
		if len(lib.NativeABIs) > 0 && len(lib.NativeFiles) == 0 {
			return nil, false
		}
		for _, path := range lib.NativeFiles {
			if st, err := os.Stat(path); err != nil || st.IsDir() {
				return nil, false
			}
		}
	}
	return cached.Set, true
}

// InvalidateCache borra la caché de dependencias (tras un "rain clean" o
// cuando el usuario cambia las versiones a mano).
func InvalidateCache(dir string) error {
	err := os.Remove(filepath.Join(dir, cacheFile))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("borrando la caché de dependencias: %w", err)
	}
	return nil
}
