package pom

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolveParentChain descarga recursivamente la cadena de parents de un POM
// y devuelve la lista ordenada de hijo a raíz (el primero es el POM original).
func ResolveParentChain(child *Project, cacheDir string) ([]*Project, error) {
	chain := []*Project{child}
	current := child

	for current.Parent != nil {
		parentCoord := fmt.Sprintf("%s:%s:%s",
			current.Parent.GroupID, current.Parent.ArtifactID, current.Parent.Version)

		// Verificar si ya está en caché
		groupPath := filepath.Join(cacheDir, current.Parent.GroupID)
		cachedPath := filepath.Join(groupPath, current.Parent.ArtifactID+"-"+current.Parent.Version+".pom")

		var parentPOM *Project
		var err error

		if _, statErr := os.Stat(cachedPath); statErr == nil {
			// Ya está descargado
			parentPOM, err = ParsePOM(cachedPath)
		} else {
			// Descargar
			path, dlErr := DownloadPOM(parentCoord, groupPath)
			if dlErr != nil {
				return nil, fmt.Errorf("descargando parent %s: %w", parentCoord, dlErr)
			}
			parentPOM, err = ParsePOM(path)
		}

		if err != nil {
			return nil, fmt.Errorf("parseando parent %s: %w", parentCoord, err)
		}

		chain = append(chain, parentPOM)
		current = parentPOM
	}

	return chain, nil
}

// MergeProperties fusiona las propiedades de la cadena de parents.
// El hijo sobrescribe al padre (el primero en la cadena tiene prioridad).
//
// Además añade las propiedades integradas de Maven (project.version,
// project.groupId, project.artifactId y sus alias pom.*), que no aparecen en
// <properties> pero se usan en muchos POMs reales. Por ejemplo el
// dependencyManagement de dagger-parent declara sus módulos con
// ${project.version}: sin resolverlo, la coordenada queda literally
// "com.google.dagger:dagger:${project.version}" y la dependencia no se puede
// descargar. Como en Maven, estas propiedades se evalúan en el contexto del
// POM hijo (chain[0]), que es quien hereda ese dependencyManagement.
func MergeProperties(chain []*Project) map[string]string {
	props := make(map[string]string)
	// Recorrer de raíz a hijo para que el hijo sobrescriba
	for i := len(chain) - 1; i >= 0; i-- {
		for _, prop := range chain[i].Properties {
			props[prop.XMLName.Local] = prop.Value
		}
	}

	// Propiedades integradas del POM hijo de la cadena.
	if len(chain) > 0 {
		child := chain[0]
		version := child.Version
		groupID := child.GroupID
		if child.Parent != nil {
			if version == "" {
				version = child.Parent.Version
			}
			if groupID == "" {
				groupID = child.Parent.GroupID
			}
		}
		props["project.version"] = version
		props["project.groupId"] = groupID
		props["project.artifactId"] = child.ArtifactID
		props["pom.version"] = version
		props["pom.groupId"] = groupID
		props["pom.artifactId"] = child.ArtifactID
	}
	return props
}

// MergeDependencyManagement fusiona los dependencyManagement de la cadena.
// El hijo sobrescribe al padre para la misma clave groupId:artifactId.
func MergeDependencyManagement(chain []*Project) map[string]string {
	mgmt := make(map[string]string)
	// Recorrer de raíz a hijo para que el hijo sobrescriba
	for i := len(chain) - 1; i >= 0; i-- {
		p := chain[i]
		if p.DependencyManagement == nil {
			continue
		}
		for _, dep := range p.DependencyManagement.Dependencies {
			key := dep.GroupID + ":" + dep.ArtifactID
			if dep.Version != "" {
				mgmt[key] = dep.Version
			}
		}
	}
	return mgmt
}
