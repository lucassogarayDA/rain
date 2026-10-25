package gradle

import (
	"regexp"
	"strings"
)

// reLocalVal busca una declaración de val local dentro del bloque
// dependencies: "val composeBom = platform("androidx.compose:compose-bom:...")".
var reLocalVal = regexp.MustCompile(`^val\s+([A-Za-z_]\w*)\s*=\s*(.+)$`)

// reDependencyLine busca una llamada de configuración:
// implementation("g:a:v"), "baselineProfile"(project(":x")), kapt("g:a:v")...
var reDependencyLine = regexp.MustCompile(`^(?:"([^"]+)"|([A-Za-z_]\w*))\s*\((.*)\)$`)

// reQuotedString extrae la primera cadena entre comillas del texto.
var reQuotedString = regexp.MustCompile(`"([^"]*)"`)

// dependencySets son las dependencias de un módulo, separadas por la variante
// en la que aplican.
//
// En Gradle, una dependencia declarada como debugImplementation solo existe en
// la variante debug: en un release no está en el classpath. Rain necesita
// distinguirlo, porque si una dependencia de debug se colase en un release,
// R8 abortaría al no encontrar las clases opcionales que esa dependencia
// referencia. El caso típico es androidx.compose.ui:ui-tooling, que referencia
// androidx.compose.animation.tooling.*, un artefacto opcional que no está en
// el classpath. d8 no valida referencias (el debug compila sin problema), pero
// R8 sí y aborta con "Missing class".
type dependencySets struct {
	// Common son las de implementation/api: van en todas las variantes.
	Common []Dependency
	// Debug son las de debugImplementation/debugApi.
	Debug []Dependency
	// Release son las de releaseImplementation/releaseApi.
	Release []Dependency
	// Kapt son los annotation processors.
	Kapt []Dependency
	// Local son las dependencias que apuntan a otro módulo del proyecto.
	Local []string
}

// parseDependencies parsea el bloque "dependencies" de un módulo.
//
// Devuelve las dependencias de compilación y runtime separadas por variante,
// los annotation processors de kapt y las dependencias que apuntan a otro
// módulo del proyecto.
func parseDependencies(body string) dependencySets {
	var sets dependencySets

	// Primero los val locales del propio bloque: las dependencias sin versión
	// suelen referenciar un BOM declarado así.
	vals := map[string]Dependency{}
	for _, stmt := range splitStatements(body) {
		m := reLocalVal.FindStringSubmatch(stmt)
		if m == nil {
			continue
		}
		if dep, ok := parseDependencyValue(m[2]); ok {
			vals[m[1]] = dep
		}
	}

	for _, stmt := range splitStatements(body) {
		// Los val locales ya se han resuelto.
		if reLocalVal.MatchString(stmt) {
			continue
		}
		m := reDependencyLine.FindStringSubmatch(stmt)
		if m == nil {
			continue
		}
		scope := m[1]
		if scope == "" {
			scope = m[2]
		}
		argument := strings.TrimSpace(m[3])

		dep, ok := parseDependencyValue(argument)
		if !ok {
			// Puede ser una referencia a un val local.
			if v, found := vals[argument]; found {
				dep = v
			} else {
				continue
			}
		}
		dep.Scope = scope

		switch {
		case dep.Local:
			sets.Local = append(sets.Local, dep.Coordinate)
		case scope == "kapt":
			sets.Kapt = append(sets.Kapt, dep)
		case scope == "ksp":
			// rain no soporta KSP: los processors de KSP no se ejecutan.
		case strings.HasPrefix(scope, "test") || strings.HasPrefix(scope, "androidTest"):
			// Configuraciones de test: no van en la app.
		case strings.HasPrefix(scope, "debug"):
			sets.Debug = append(sets.Debug, dep)
		case strings.HasPrefix(scope, "release"):
			sets.Release = append(sets.Release, dep)
		default:
			sets.Common = append(sets.Common, dep)
		}
	}
	return sets
}

// parseDependencyValue interpreta el argumento de una dependencia.
//
// Acepta una coordenada ("g:a:v"), un platform(...) (un BOM), un project(...)
// (otro módulo del proyecto) o un val local ya resuelto.
func parseDependencyValue(value string) (Dependency, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return Dependency{}, false
	}

	// platform("g:a:v") → BOM.
	if strings.HasPrefix(value, "platform(") {
		inner := strings.TrimSuffix(strings.TrimPrefix(value, "platform("), ")")
		coord := firstQuoted(inner)
		if coord == "" {
			return Dependency{}, false
		}
		return Dependency{Coordinate: coord, Platform: true}, true
	}

	// project(":otro") → otro módulo del proyecto.
	if strings.HasPrefix(value, "project(") {
		inner := strings.TrimSuffix(strings.TrimPrefix(value, "project("), ")")
		path := firstQuoted(inner)
		if path == "" {
			return Dependency{}, false
		}
		return Dependency{Coordinate: path, Local: true}, true
	}

	// Una cadena suelta: la coordenada.
	if strings.HasPrefix(value, `"`) {
		coord := firstQuoted(value)
		if coord == "" {
			return Dependency{}, false
		}
		return Dependency{Coordinate: coord}, true
	}

	// Un identificador: un val local que no se ha podido resolver, o un
	// catálogo de versiones de Gradle (libs.foo).
	return Dependency{}, false
}

// firstQuoted devuelve la primera cadena entre comillas del texto.
func firstQuoted(src string) string {
	if m := reQuotedString.FindStringSubmatch(src); m != nil {
		return m[1]
	}
	return ""
}
