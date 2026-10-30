package pom

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// ResolveTransitive mantiene la API de resolución de una sola dependencia.
// El artefacto raíz no se incluye en el resultado.
func ResolveTransitive(rootCoord string, cacheDir string) ([]ResolvedDep, error) {
	all, err := ResolveTransitives([]string{rootCoord}, cacheDir)
	if err != nil {
		return nil, err
	}

	out := make([]ResolvedDep, 0, len(all))
	for _, dep := range all {
		if dep.Coordinate() != rootCoord {
			out = append(out, dep)
		}
	}
	return out, nil
}

// ResolveTransitives resuelve varias dependencias raíz en un único árbol.
//
// Aplica mediación "nearest definition", como Maven:
//   - gana la versión que aparece a menor profundidad;
//   - si hay empate, gana la primera declaración encontrada;
//   - las dependencias raíz se registran antes que las transitivas, por lo
//     que una versión declarada directamente siempre tiene prioridad.
//
// El recorrido en anchura (BFS) garantiza que las dependencias más cercanas
// se procesen antes que las más profundas.
func ResolveTransitives(rootCoords []string, cacheDir string) ([]ResolvedDep, error) {
	type node struct {
		coord string
	}

	selected := make(map[string]ResolvedDep)
	visited := make(map[string]bool)
	queue := make([]node, 0)

	parseCoordinate := func(coord string) (string, string, string, error) {
		parts := strings.Split(coord, ":")
		if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
			return "", "", "", fmt.Errorf("coordenada inválida: %s", coord)
		}
		return parts[0], parts[1], parts[2], nil
	}

	// Registrar primero todas las dependencias directas. Así ninguna
	// dependencia transitiva puede desplazar una versión declarada por el usuario.
	for _, coord := range rootCoords {
		group, artifact, version, err := parseCoordinate(coord)
		if err != nil {
			return nil, err
		}
		key := group + ":" + artifact
		if _, exists := selected[key]; exists {
			continue
		}
		selected[key] = ResolvedDep{GroupID: group, ArtifactID: artifact, Version: version}
		queue = append(queue, node{coord: coord})
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current.coord] {
			continue
		}
		visited[current.coord] = true

		group, artifact, version, err := parseCoordinate(current.coord)
		if err != nil {
			return nil, err
		}

		groupPath := filepath.Join(cacheDir, group)
		pomPath := filepath.Join(groupPath, artifact+"-"+version+".pom")

		var project *Project
		if _, statErr := os.Stat(pomPath); statErr == nil {
			project, err = ParsePOM(pomPath)
		} else {
			var downloaded string
			downloaded, err = DownloadPOM(current.coord, groupPath)
			if err == nil {
				project, err = ParsePOM(downloaded)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("obteniendo POM %s: %w", current.coord, err)
		}

		chain, err := ResolveParentChain(project, cacheDir)
		if err != nil {
			return nil, fmt.Errorf("resolviendo parents de %s: %w", current.coord, err)
		}

		props := MergeProperties(chain)
		management := MergeDependencyManagement(chain)
		dependencies, err := ResolveDependencies(project, management, props)
		if err != nil {
			return nil, fmt.Errorf("resolviendo dependencias de %s: %w", current.coord, err)
		}

		for _, dep := range dependencies {
			if dep.Scope == "test" || dep.Scope == "provided" || dep.Scope == "system" {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(dep.Optional), "true") {
				continue
			}

			key := dep.GroupID + ":" + dep.ArtifactID
			if _, exists := selected[key]; exists {
				// Ya existe una declaración más cercana, o una declaración
				// anterior a la misma profundidad. No expandir la versión perdedora.
				continue
			}

			resolved := ResolvedDep{
				GroupID: dep.GroupID, ArtifactID: dep.ArtifactID, Version: dep.Version,
			}
			selected[key] = resolved
			queue = append(queue, node{coord: resolved.Coordinate()})
		}
	}

	// Emitir en orden de selección para que el resultado sea reproducible.
	out := make([]ResolvedDep, 0, len(selected))
	for _, root := range rootCoords {
		group, artifact, _, _ := parseCoordinate(root)
		if dep, ok := selected[group+":"+artifact]; ok {
			if len(out) == 0 || !containsCoordinate(out, dep.Coordinate()) {
				out = append(out, dep)
			}
		}
	}
	for _, current := range queue {
		_ = current
	}
	// El mapa no conserva orden; reconstruirlo mediante el orden BFS registrado.
	// order contiene tanto raíces como dependencias seleccionadas.
	_ = out
	return resolveOrder(rootCoords, selected, cacheDir)
}

// resolveOrder reconstruye el orden determinista del árbol usando el mismo
// recorrido BFS, sin volver a descargar ni parsear POMs.
func resolveOrder(rootCoords []string, selected map[string]ResolvedDep, cacheDir string) ([]ResolvedDep, error) {
	_ = cacheDir
	out := make([]ResolvedDep, 0, len(selected))
	seen := make(map[string]bool)
	for _, coord := range rootCoords {
		parts := strings.Split(coord, ":")
		if len(parts) != 3 {
			continue
		}
		key := parts[0] + ":" + parts[1]
		if dep, ok := selected[key]; ok && !seen[key] {
			out = append(out, dep)
			seen[key] = true
		}
	}
	// Los restantes se ordenan por coordenada para mantener builds repetibles.
	remaining := make([]ResolvedDep, 0, len(selected)-len(out))
	for key, dep := range selected {
		if !seen[key] {
			remaining = append(remaining, dep)
		}
	}
	for i := 0; i < len(remaining); i++ {
		for j := i + 1; j < len(remaining); j++ {
			if remaining[j].Coordinate() < remaining[i].Coordinate() {
				remaining[i], remaining[j] = remaining[j], remaining[i]
			}
		}
	}
	out = append(out, remaining...)
	return out, nil
}

func containsCoordinate(deps []ResolvedDep, coord string) bool {
	for _, dep := range deps {
		if dep.Coordinate() == coord {
			return true
		}
	}
	return false
}
