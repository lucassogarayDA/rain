package pom

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/itskreisler/rain/internal/ui"
)

// ResolvedDep representa una dependencia final resuelta.
type ResolvedDep struct {
	GroupID    string
	ArtifactID string
	Version    string
}

func (d ResolvedDep) Coordinate() string {
	return fmt.Sprintf("%s:%s:%s", d.GroupID, d.ArtifactID, d.Version)
}

// ResolveTransitive resuelve todas las dependencias transitivas de una coordenada raíz.
// Usa "highest wins" para conflictos de versión. Las ramas que no se pueden
// descargar se omiten (con Warning) en vez de abortar toda la resolución: el
// grafo de AndroidX trae artefactos antiguos/no publicados que no aportan nada.
func ResolveTransitive(rootCoord string, cacheDir string) ([]ResolvedDep, error) {
	resolved := make(map[string]ResolvedDep) // key: group:artifact
	order := make([]string, 0, 16)           // claves en orden de descubrimiento
	visited := make(map[string]bool)         // key: group:artifact:version
	skipped := make(map[string]bool)         // coordenadas omitidas

	// record guarda la dependencia conservando el orden de descubrimiento.
	// Iterar el mapa directamente daría un orden aleatorio en cada ejecución:
	// el classpath cambiaría de un build al siguiente y el DEX saldría
	// distinto, rompiendo la reproducibilidad del APK.
	record := func(key string, dep ResolvedDep) {
		if _, ok := resolved[key]; !ok {
			order = append(order, key)
		}
		resolved[key] = dep
	}

	var walk func(coord string, depth int) error
	walk = func(coord string, depth int) error {
		if visited[coord] || skipped[coord] {
			return nil
		}
		visited[coord] = true

		parts := strings.Split(coord, ":")
		if len(parts) != 3 {
			return fmt.Errorf("coordenada inválida: %s", coord)
		}
		group, artifact, version := parts[0], parts[1], parts[2]

		// Descargar POM (o usar caché)
		groupPath := filepath.Join(cacheDir, group)
		pomPath := filepath.Join(groupPath, artifact+"-"+version+".pom")

		var p *Project
		var err error
		if _, statErr := os.Stat(pomPath); statErr == nil {
			p, err = ParsePOM(pomPath)
		} else {
			path, dlErr := DownloadPOM(coord, groupPath)
			if dlErr != nil {
				// No abortamos: la rama se omite. Solo la raíz es crítica.
				if coord == rootCoord {
					return fmt.Errorf("descargando POM raíz %s: %w", coord, dlErr)
				}
				skipped[coord] = true
				ui.Warn("dependencia omitida (no descargable): %s", coord)
				return nil
			}
			p, err = ParsePOM(path)
		}
		if err != nil {
			if coord == rootCoord {
				return fmt.Errorf("parseando POM %s: %w", coord, err)
			}
			skipped[coord] = true
			ui.Warn("dependencia omitida (POM inválido): %s", coord)
			return nil
		}

		// Resolver cadena de parents
		chain, err := ResolveParentChain(p, cacheDir)
		if err != nil {
			if coord == rootCoord {
				return fmt.Errorf("resolviendo parents de %s: %w", coord, err)
			}
			skipped[coord] = true
			ui.Warn("dependencia omitida (parents no resolubles): %s", coord)
			return nil
		}

		// Fusionar propiedades y management
		props := MergeProperties(chain)
		mgmt := MergeDependencyManagement(chain)

		// Resolver dependencias directas de este POM
		deps, err := ResolveDependencies(p, mgmt, props)
		if err != nil {
			if coord == rootCoord {
				return fmt.Errorf("resolviendo dependencias de %s: %w", coord, err)
			}
			skipped[coord] = true
			ui.Warn("dependencia omitida (dependencias no resolubles): %s", coord)
			return nil
		}

		for _, dep := range deps {
			// Filtrar scopes no necesarios para compilación
			if dep.Scope == "test" || dep.Scope == "provided" || dep.Scope == "system" {
				continue
			}
			if dep.Optional == "true" {
				continue
			}
			// kotlin-stdlib-jdk7/jdk8 se fusionaron en kotlin-stdlib desde
			// Kotlin 1.8; arrastrarlos trae versiones antiguas (1.5.x) que
			// además ya no se publican en algunos repos.
			if isMergedKotlinStdlib(dep.GroupID, dep.ArtifactID) {
				continue
			}
			// El stdlib lo aporta el kotlinc del toolchain: no lo dupliquemos.
			if isKotlinStdlib(dep.GroupID, dep.ArtifactID) {
				continue
			}
			// Artefactos absorbidos por otros: evitan clases duplicadas en d8.
			if deprecatedArtifact(dep.GroupID, dep.ArtifactID) {
				continue
			}

			// Clave de módulo: normaliza variantes de publicación (foo,
			// foo-android, foo-jvm) a un mismo módulo lógico, para que
			// "highest wins" elija la versión nueva y no se cuelen a la vez
			// la 2.6.1 sin sufijo y la 2.10.0 -android (clases duplicadas).
			key := moduleKey(dep.GroupID, dep.ArtifactID)
			newCoord := dep.GroupID + ":" + dep.ArtifactID + ":" + dep.Version

			// Highest wins: si ya existe, comparar versiones
			if existing, ok := resolved[key]; ok {
				if versionLess(existing.Version, dep.Version) {
					record(key, ResolvedDep{dep.GroupID, dep.ArtifactID, dep.Version})
				}
			} else {
				record(key, ResolvedDep{dep.GroupID, dep.ArtifactID, dep.Version})
			}

			// Recursión
			if err := walk(newCoord, depth+1); err != nil {
				return err
			}
		}

		// Añadir este artefacto al resultado (si no es la raíz)
		if coord != rootCoord {
			key := moduleKey(group, artifact)
			if existing, ok := resolved[key]; !ok || versionLess(existing.Version, version) {
				record(key, ResolvedDep{group, artifact, version})
			}
		}

		return nil
	}

	if err := walk(rootCoord, 0); err != nil {
		return nil, err
	}

	// Convertir a slice respetando el orden de descubrimiento.
	out := make([]ResolvedDep, 0, len(order))
	for _, key := range order {
		out = append(out, resolved[key])
	}
	return out, nil
}

// DedupeCoordinates reduce una lista de coordenadas "grupo:artefacto:versión"
// a una sola versión por módulo, conservando la más alta. Es el paso de
// "highest wins" global: cada dependencia declarada se resuelve por separado,
// así que la unión puede traer varias versiones del mismo artefacto (p. ej.
// androidx.core 1.7.0/1.9.0/1.12.0/1.13.1). Dejar todas en el classpath hace
// que kotlinc/javac vean clases contradictorias y fallen, así que se colapsan
// aquí. Se conserva el orden de primera aparición de cada módulo.
func DedupeCoordinates(coords []string) []string {
	best := make(map[string]string) // moduleKey -> coordenada ganadora
	order := make([]string, 0, len(coords))
	for _, coord := range coords {
		parts := strings.SplitN(coord, ":", 3)
		if len(parts) != 3 {
			// Coordenada con formato inesperado: se conserva sin tocar.
			order = append(order, coord)
			continue
		}
		group, artifact, version := parts[0], parts[1], parts[2]
		key := moduleKey(group, artifact)
		existing, ok := best[key]
		if !ok {
			best[key] = coord
			order = append(order, key)
			continue
		}
		existingParts := strings.SplitN(existing, ":", 3)
		if len(existingParts) == 3 && versionLess(existingParts[2], version) {
			best[key] = coord
		}
	}

	out := make([]string, 0, len(order))
	for _, key := range order {
		if coord, ok := best[key]; ok {
			out = append(out, coord)
			delete(best, key)
		} else {
			// Caso de formato inesperado conservado tal cual.
			out = append(out, key)
		}
	}
	return out
}

// moduleKey normaliza el nombre de artefacto de un módulo a su forma base,
// eliminando el sufijo de variante de publicación de AndroidX/Maven
// (-android, -jvm). Así `lifecycle-runtime`, `lifecycle-runtime-android` y
// `lifecycle-runtime-jvm` se tratan como el mismo módulo a efectos de
// "highest wins", evitando definir dos veces las mismas clases en d8.
func moduleKey(group, artifact string) string {
	base := artifact
	for _, suffix := range []string{"-android", "-jvm"} {
		base = strings.TrimSuffix(base, suffix)
	}
	return group + ":" + base
}

// isMergedKotlinStdlib detecta los artefactos kotlin-stdlib-jdk7/jdk8, cuyas
// clases se integraron en kotlin-stdlib en Kotlin 1.8.
func isMergedKotlinStdlib(group, artifact string) bool {
	if group != "org.jetbrains.kotlin" {
		return false
	}
	return artifact == "kotlin-stdlib-jdk7" || artifact == "kotlin-stdlib-jdk8"
}

// isKotlinStdlib detecta cualquier artefacto del stdlib de Kotlin. Rain inyecta
// el stdlib del propio kotlinc (versión del toolchain) al dexear, así que
// arrastrar otra versión desde el grafo provocaría clases duplicadas en d8.
func isKotlinStdlib(group, artifact string) bool {
	if group != "org.jetbrains.kotlin" {
		return false
	}
	switch artifact {
	case "kotlin-stdlib", "kotlin-stdlib-jdk7", "kotlin-stdlib-jdk8", "kotlin-stdlib-common":
		return true
	default:
		return false
	}
}

// deprecatedArtifact detecta artefactos que fueron absorbidos por otro y cuya
// presencia duplica clases (d8 falla con "defined multiple times"). Se omiten
// del cierre para no arrastrar versiones antiguas.
func deprecatedArtifact(group, artifact string) bool {
	switch group + ":" + artifact {
	case "androidx.lifecycle:lifecycle-common-java8": // absorbido por lifecycle-common ≥2.4
		return true
	default:
		return false
	}
}

// versionLess compara dos versiones de forma simple (asume formato semver-like).
func versionLess(a, b string) bool {
	// Implementación básica: comparar numéricamente las partes separadas por "."
	// Si no son numéricas, comparar como strings.
	if a == b {
		return false
	}
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		na, aok := numericPrefix(pa[i])
		nb, bok := numericPrefix(pb[i])
		if aok && bok {
			if na != nb {
				return na < nb
			}
			// Si ambos son puramente numéricos, sigue; si uno tiene
			// cualificador (1.0.0-beta) el otro (1.0.0) gana, y entre
			// cualificadores decide el orden textual.
			if pa[i] != pb[i] {
				return qualifierLess(pa[i], pb[i])
			}
			continue
		}
		return pa[i] < pb[i]
	}
	// Mismas partes numéricas comunes: decide por lo que queda
	// ("1.0" < "1.0.1").
	return len(pa) < len(pb)
}

// numericPrefix extrae el número inicial de un segmento ("0-beta" -> 0, true).
func numericPrefix(segment string) (int, bool) {
	i := 0
	for i < len(segment) && segment[i] >= '0' && segment[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, false
	}
	return atoi(segment[:i]), true
}

// atoi convierte un prefijo numérico ya validado.
func atoi(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		n = n*10 + int(s[i]-'0')
	}
	return n
}

// qualifierLess compara dos segmentos con cualificador textual.
// Un segmento sin cualificador ("0") es mayor que uno con él ("0-beta").
func qualifierLess(a, b string) bool {
	qa := qualifierOf(a)
	qb := qualifierOf(b)
	switch {
	case qa == "" && qb == "":
		return false
	case qa == "":
		return false // 0 > 0-beta
	case qb == "":
		return true // 0-beta < 0
	default:
		return qa < qb
	}
}

// qualifierOf devuelve la parte textual de un segmento ("0-beta" -> "-beta").
func qualifierOf(segment string) string {
	i := 0
	for i < len(segment) && segment[i] >= '0' && segment[i] <= '9' {
		i++
	}
	return segment[i:]
}
