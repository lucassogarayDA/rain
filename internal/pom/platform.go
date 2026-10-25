package pom

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ManagedVersions devuelve las versiones que fija una plataforma (BOM),
// indexadas por "grupo:artifacto".
//
// Un BOM ("bill of materials") es un POM sin artefacto propio que declara en su
// <dependencyManagement> las versiones de un conjunto de librerías. Gradle lo
// usa con platform("grupo:artifacto:version"): las dependencias declaradas
// sin versión heredan la que fija el BOM.
//
// Por ejemplo, con el BOM de Compose, "androidx.compose.ui:ui" sin versión
// se resuelve a la versión que el BOM declare para ese artifact.
func ManagedVersions(coordinate string, cacheDir string) (map[string]string, error) {
	parts := strings.Split(coordinate, ":")
	if len(parts) != 3 {
		return nil, fmt.Errorf("coordenada inválida: %s", coordinate)
	}
	group, artifact, version := parts[0], parts[1], parts[2]
	groupPath := filepath.Join(cacheDir, group)

	var p *Project
	var err error
	pomPath := filepath.Join(groupPath, artifact+"-"+version+".pom")
	if _, statErr := os.Stat(pomPath); statErr == nil {
		p, err = ParsePOM(pomPath)
	} else {
		var path string
		path, err = DownloadPOM(coordinate, groupPath)
		if err == nil {
			p, err = ParsePOM(path)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("descargando el BOM %s: %w", coordinate, err)
	}

	chain, err := ResolveParentChain(p, cacheDir)
	if err != nil {
		return nil, fmt.Errorf("resolviendo parents de %s: %w", coordinate, err)
	}
	props := MergeProperties(chain)
	mgmt := MergeDependencyManagement(chain)

	out := make(map[string]string, len(mgmt))
	for key, value := range mgmt {
		out[key] = ResolveProperties(value, props)
	}
	return out, nil
}
