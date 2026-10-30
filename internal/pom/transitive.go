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

// ResolveTransitive resuelve las dependencias transitivas de una coordenada.
// La dependencia raíz no se incluye en el resultado.
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

// ResolveTransitives resuelve varias raíces en un único grafo y aplica
// mediación Maven "nearest definition": menor profundidad gana y, en empate,
// gana la primera dependencia declarada. BFS garantiza ese orden.
func ResolveTransitives(rootCoords []string, cacheDir string) ([]ResolvedDep, error) {
	type node struct {
		coord string
	}

	selected := make(map[string]bool)
	visited := make(map[string]bool)
	queue := make([]node, 0)
	order := make([]ResolvedDep, 0)

	parseCoordinate := func(coord string) (string, string, string, error) {
		parts := strings.Split(coord, ":")
		if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
			return "", "", "", fmt.Errorf("coordenada inválida: %s", coord)
		}
		return parts[0], parts[1], parts[2], nil
	}

	// Las raíces se seleccionan antes de recorrer cualquier POM.
	for _, coord := range rootCoords {
		group, artifact, version, err := parseCoordinate(coord)
		if err != nil {
			return nil, err
		}
		key := group + ":" + artifact
		if selected[key] {
			continue
		}
		selected[key] = true
		order = append(order, ResolvedDep{GroupID: group, ArtifactID: artifact, Version: version})
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
			if selected[key] {
				continue
			}

			resolved := ResolvedDep{
				GroupID: dep.GroupID, ArtifactID: dep.ArtifactID, Version: dep.Version,
			}
			selected[key] = true
			order = append(order, resolved)
			queue = append(queue, node{coord: resolved.Coordinate()})
		}
	}

	return order, nil
}
