package pom

import (
	"encoding/xml"
	"fmt"
	"os"
	"strings"

	"github.com/itskreisler/rain/internal/httpx"
	"github.com/itskreisler/rain/internal/mavenrepo"
)

// Repositorios base en orden de prioridad. JitPack se añade por coordenada
// (solo grupos com.github.*/io.github.*) vía mavenrepo.For, y los extras de
// rain.toml se añaden al final.
var repositories = mavenrepo.Base

// Project representa la estructura relevante de un POM de Maven.
type Project struct {
	XMLName              xml.Name     `xml:"project"`
	GroupID              string       `xml:"groupId"`
	ArtifactID           string       `xml:"artifactId"`
	Version              string       `xml:"version"`
	Parent               *Parent      `xml:"parent"`
	Properties           Properties   `xml:"properties"`
	DependencyManagement *DepMgmt     `xml:"dependencyManagement"`
	Dependencies         []Dependency `xml:"dependencies>dependency"`
}

type Parent struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
}

// Property es una entrada de <properties>; el nombre del elemento es la clave.
type Property struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

// Properties es un slice de propiedades con unmarshal propio: encoding/xml no
// puede mapear elementos de nombre arbitrario (<kotlin.version>,
// <androidx.core.version>...) a un slice, así que los recogemos con `,any`.
type Properties []Property

// UnmarshalXML implementa la lectura de <properties> elemento a elemento:
// cada hijo directo es una propiedad cuyo nombre es la clave.
func (props *Properties) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			var value string
			if err := d.DecodeElement(&value, &t); err != nil {
				return err
			}
			*props = append(*props, Property{XMLName: t.Name, Value: value})
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

type DepMgmt struct {
	Dependencies []Dependency `xml:"dependencies>dependency"`
}

type Dependency struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
	Scope      string `xml:"scope"`
	Optional   string `xml:"optional"`
}

// needFetch indica si un artefacto hay que descargarlo o si ya está en la
// caché. Se separa del resto para poder testearlo sin red.
func needFetch(path string) bool {
	_, err := os.Stat(path)
	return err != nil
}

// DownloadPOM intenta descargar el POM desde cada repositorio hasta encontrarlo.
func DownloadPOM(coordinate string, destDir string) (string, error) {
	parts := strings.Split(coordinate, ":")
	if len(parts) != 3 {
		return "", fmt.Errorf("coordenada inválida: %s", coordinate)
	}
	group, artifact, version := parts[0], parts[1], parts[2]
	groupPath := strings.ReplaceAll(group, ".", "/")

	// Verificar caché primero
	path := fmt.Sprintf("%s/%s-%s.pom", destDir, artifact, version)
	if !needFetch(path) {
		return path, nil
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}

	client := httpx.NewClient()
	var lastErr error
	for _, repo := range mavenrepo.For(repositories, group) {
		url := fmt.Sprintf("%s/%s/%s/%s/%s-%s.pom",
			repo, groupPath, artifact, version, artifact, version)

		resp, err := client.Get(url)
		if err != nil {
			lastErr = fmt.Errorf("%w en %s", err, url)
			continue
		}

		// Descarga atómica: se escribe a un .part y se renombra al final.
		// Así una descarga interrumpida no deja un POM corrupto en la caché,
		// que es exactamente lo que pasa al matar el proceso a mitad.
		part := path + ".part"
		f, err := os.Create(part)
		if err != nil {
			_ = resp.Body.Close()
			return "", err
		}
		_, copyErr := f.ReadFrom(resp.Body)
		_ = resp.Body.Close()
		_ = f.Close()

		if copyErr != nil {
			_ = os.Remove(part)
			lastErr = copyErr
			continue
		}

		if err := os.Rename(part, path); err != nil {
			_ = os.Remove(part)
			return "", err
		}

		return path, nil
	}

	if lastErr != nil {
		return "", fmt.Errorf("no se encontró POM para %s en ningún repositorio: %w", coordinate, lastErr)
	}
	return "", fmt.Errorf("no se encontró POM para %s en ningún repositorio", coordinate)
}

// ParsePOM lee un POM y devuelve el proyecto.
func ParsePOM(path string) (*Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Project
	if err := xml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parseando POM: %w", err)
	}
	return &p, nil
}
