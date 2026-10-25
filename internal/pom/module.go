package pom

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/itskreisler/rain/internal/httpx"
	"github.com/itskreisler/rain/internal/mavenrepo"
)

// ModuleMetadata modela el fichero ".module" de Gradle (Gradle Module
// Metadata). Es la fuente de verdad que usa Gradle para decidir qué artefacto
// concreto se descarga de un módulo publicado como Kotlin Multiplatform.
//
// Sin esto, rain solo mira el POM y se queda con el artefacto raíz, que en los
// módulos KMP es una "cáscara" sin clases: el código real vive en la variante
// "-android" (p. ej. androidx.compose.ui:ui-graphics → ui-graphics-android).
// El .module lo indica en la variante Android mediante "available-at".
type ModuleMetadata struct {
	Variants []ModuleVariant `json:"variants"`
}

// ModuleVariant es una variante de publicación (android, jvm, metadata...).
type ModuleVariant struct {
	Name string `json:"name"`
	// Attributes usa map[string]any porque algunos atributos no son cadenas
	// (p. ej. "androidx.kmp.hasCInterop": false) y con map[string]string el
	// parseo del .module fallaba entero.
	Attributes   map[string]any `json:"attributes"`
	AvailableAt  *AvailableAt   `json:"available-at"`
	Dependencies []ModuleDep    `json:"dependencies"`
	Files        []ModuleFile   `json:"files"`
}

// attributeString devuelve el valor de un atributo como cadena. Devuelve ""
// si el atributo no existe o no es una cadena.
func attributeString(attrs map[string]any, key string) string {
	if attrs == nil {
		return ""
	}
	if s, ok := attrs[key].(string); ok {
		return s
	}
	return ""
}

// AvailableAt apunta al módulo real de otra variante.
type AvailableAt struct {
	Group   string `json:"group"`
	Module  string `json:"module"`
	Version string `json:"version"`
}

// Coordinate devuelve la coordenada "group:module:version" del destino.
func (a *AvailableAt) Coordinate() string {
	if a == nil || a.Group == "" || a.Module == "" {
		return ""
	}
	version := a.Version
	if version == "" {
		version = "1.0"
	}
	return a.Group + ":" + a.Module + ":" + version
}

// ModuleDep es una dependencia declarada en una variante.
type ModuleDep struct {
	Group   string `json:"group"`
	Module  string `json:"module"`
	Version struct {
		Requires string `json:"requires"`
		Prefer   string `json:"prefer"`
	} `json:"version"`
}

// ModuleFile es un artefacto (AAR/JAR) publicado por una variante.
type ModuleFile struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// DownloadModuleMetadata intenta descargar el fichero ".module" de una
// coordenada. Devuelve error si el módulo no lo publica (la mayoría de los
// artefactos antiguos no lo hacen).
func DownloadModuleMetadata(coordinate string, destDir string) (string, error) {
	parts := strings.Split(coordinate, ":")
	if len(parts) != 3 {
		return "", fmt.Errorf("coordenada inválida: %s", coordinate)
	}
	group, artifact, version := parts[0], parts[1], parts[2]
	groupPath := filepath.Join(destDir, group)

	path := filepath.Join(groupPath, artifact+"-"+version+".module")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	if err := os.MkdirAll(groupPath, 0755); err != nil {
		return "", err
	}

	client := httpx.NewClient()
	var lastErr error
	for _, repo := range mavenrepo.For(repositories, group) {
		url := fmt.Sprintf("%s/%s/%s/%s/%s-%s.module",
			repo, strings.ReplaceAll(group, ".", "/"), artifact, version, artifact, version)

		resp, err := client.Get(url)
		if err != nil {
			lastErr = fmt.Errorf("%w en %s", err, url)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("%s devolvió HTTP %d", url, resp.StatusCode)
			continue
		}

		f, err := os.Create(path)
		if err != nil {
			_ = resp.Body.Close()
			return "", err
		}
		_, copyErr := f.ReadFrom(resp.Body)
		_ = resp.Body.Close()
		_ = f.Close()

		if copyErr != nil {
			_ = os.Remove(path)
			lastErr = copyErr
			continue
		}
		return path, nil
	}

	if lastErr != nil {
		return "", fmt.Errorf("no se encontró metadata (.module) para %s: %w", coordinate, lastErr)
	}
	return "", fmt.Errorf("no se encontró metadata (.module) para %s", coordinate)
}

// ParseModuleMetadata lee y parsea un fichero .module.
func ParseModuleMetadata(path string) (*ModuleMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var md ModuleMetadata
	if err := json.Unmarshal(data, &md); err != nil {
		return nil, fmt.Errorf("parseando metadata de módulo: %w", err)
	}
	return &md, nil
}

// AndroidVariant devuelve la variante de publicación para Android (API), o nil
// si el módulo no la tiene. Prioriza la variante "ApiElements" porque es la
// que Gradle usa para el classpath de compilación.
func (md *ModuleMetadata) AndroidVariant() *ModuleVariant {
	if md == nil {
		return nil
	}
	// Pasada estricta: el atributo de entorno es el que usa Gradle.
	if v := md.androidVariantBy(func(v *ModuleVariant) bool {
		return attributeString(v.Attributes, "org.gradle.jvm.environment") == "android"
	}); v != nil {
		return v
	}
	// Pasada laxa: hay módulos publicados con versiones antiguas de Gradle que
	// no incluyen el atributo de entorno. En ellos la variante Android se
	// identifica por el tipo de plataforma ("androidJvm") o, en su defecto, por
	// el módulo al que apunta (sufijo "-android"). Sin esto, la raíz KMP se
	// queda como "cáscara" vacía y sus clases no llegan al classpath.
	return md.androidVariantBy(func(v *ModuleVariant) bool {
		if attributeString(v.Attributes, "org.jetbrains.kotlin.platform.type") == "androidJvm" {
			return true
		}
		return v.AvailableAt != nil && strings.HasSuffix(v.AvailableAt.Module, "-android")
	})
}

// androidVariantBy busca la variante que cumple el predicado, prefiriendo la
// de uso "java-api" y la de nombre "ApiElements" (la que Gradle mete en el
// classpath de compilación).
func (md *ModuleMetadata) androidVariantBy(match func(*ModuleVariant) bool) *ModuleVariant {
	var fallback *ModuleVariant
	for i := range md.Variants {
		v := &md.Variants[i]
		if !match(v) {
			continue
		}
		usage := attributeString(v.Attributes, "org.gradle.usage")
		if usage != "" && usage != "java-api" {
			continue
		}
		if strings.Contains(v.Name, "ApiElements") {
			return v
		}
		if fallback == nil {
			fallback = v
		}
	}
	return fallback
}

// AndroidTargetCoord devuelve la coordenada del artefacto real cuando la
// variante Android redirige a otro módulo (available-at). Devuelve "" si no hay
// redirección.
func (md *ModuleMetadata) AndroidTargetCoord() string {
	v := md.AndroidVariant()
	if v == nil {
		return ""
	}
	return v.AvailableAt.Coordinate()
}

// AndroidVariantCoordinate resuelve la coordenada real de la variante Android
// de un módulo publicado con Gradle Module Metadata. Si el módulo no publica
// metadata, no tiene variante Android o no redirige a otro artefacto, devuelve
// la coordenada original.
//
// Se usa antes de recorrer el grafo transitivo: un módulo KMP como
// io.github.kyant0:backdrop solo publica un JAR de metadata en la raíz, y sus
// POMs declaran dependencias de las variantes common/JVM (skiko, ui-uikit…)
// que no existen en Android. Siguiendo available-at se resuelve el POM de
// backdrop-android, que es el que Gradle usaría.
func AndroidVariantCoordinate(coordinate, destDir string) string {
	path, err := DownloadModuleMetadata(coordinate, destDir)
	if err != nil {
		return coordinate
	}
	md, err := ParseModuleMetadata(path)
	if err != nil {
		return coordinate
	}
	if target := md.AndroidTargetCoord(); target != "" {
		return target
	}
	return coordinate
}

// AndroidDependencies devuelve las dependencias de la variante Android, en
// formato "group:module:version". Los módulos sin versión explícita se
// omiten: no se pueden resolver sin más información.
func (md *ModuleMetadata) AndroidDependencies() []string {
	v := md.AndroidVariant()
	if v == nil {
		return nil
	}
	var out []string
	for _, d := range v.Dependencies {
		version := d.Version.Requires
		if version == "" {
			version = d.Version.Prefer
		}
		if version == "" {
			continue
		}
		out = append(out, fmt.Sprintf("%s:%s:%s", d.Group, d.Module, version))
	}
	return out
}
