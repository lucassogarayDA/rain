package pom

import (
	"fmt"
	"strings"
)

// ResolveProperties reemplaza las referencias ${...} en una cadena
// usando el mapa de propiedades fusionadas.
func ResolveProperties(value string, props map[string]string) string {
	if value == "" {
		return ""
	}
	for i := 0; i < 10; i++ {
		changed := false
		for key, val := range props {
			placeholder := "${" + key + "}"
			if strings.Contains(value, placeholder) {
				value = strings.ReplaceAll(value, placeholder, val)
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return value
}

// ResolveDependencies devuelve la lista de dependencias con versiones resueltas.
func ResolveDependencies(p *Project, mgmt map[string]string, props map[string]string) ([]Dependency, error) {
	var resolved []Dependency

	for _, dep := range p.Dependencies {
		groupID := ResolveProperties(dep.GroupID, props)
		artifactID := ResolveProperties(dep.ArtifactID, props)
		version := ResolveProperties(dep.Version, props)

		if version == "" {
			key := groupID + ":" + artifactID
			if managed, ok := mgmt[key]; ok {
				version = ResolveProperties(managed, props)
			} else {
				return nil, fmt.Errorf("sin versión para %s:%s y no está en dependencyManagement", groupID, artifactID)
			}
		}

		// Colapsar rangos de versión ([2.1.0] -> 2.1.0)
		version = NormalizeVersion(version)
		if version == "" {
			return nil, fmt.Errorf("rango de versión no soportado para %s:%s", groupID, artifactID)
		}

		resolved = append(resolved, Dependency{
			GroupID:    groupID,
			ArtifactID: artifactID,
			Version:    version,
			Scope:      dep.Scope,
			Optional:   dep.Optional,
		})
	}

	return resolved, nil
}
