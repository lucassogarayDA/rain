// Package mavenrepo centraliza las URLs de repositorios Maven que usa Rain y
// decide, por coordenada, en cuáles buscar.
//
// Política (opción B + A del plan):
//
//   - Base fija y siempre presente: Google Maven y Maven Central.
//   - JitPack se añade *automáticamente* solo cuando el grupo de la coordenada
//     es de GitHub (com.github.* o io.github.*). Esos artefactos no existen en
//     Google/Central, así que añadirlo siempre sería latencia pura para el
//     resto de builds. Ej.: com.github.QWEA0:liquidglass solo está en JitPack.
//   - ExtraRepositories (opción A) permite al usuario declarar repos adicionales
//     arbitrarios desde rain.toml ([repositories].extra), p. ej. un Nexus
//     propio o cualquier repositorio de terceros. Se añaden al final.
package mavenrepo

import "strings"

// Base son los repositorios consultados siempre, en orden de prioridad.
var Base = []string{
	"https://dl.google.com/dl/android/maven2",
	"https://repo1.maven.org/maven2",
}

// JitPack es el repositorio que sirve artefactos publicados desde GitHub.
const JitPack = "https://jitpack.io"

// ExtraRepositories son repositorios adicionales declarados por el usuario en
// rain.toml ([repositories].extra). Se añaden al final de la lista base.
var ExtraRepositories []string

// For devuelve los repositorios a consultar para un grupo Maven dado, en orden
// de prioridad: base + JitPack (solo si el grupo es de GitHub) + extras.
//
// base es la lista base efectiva; se pasa explícitamente para que los tests
// puedan sustituirla por un servidor httptest sin afectar al paquete global.
func For(base []string, group string) []string {
	repos := make([]string, 0, len(base)+len(ExtraRepositories)+1)
	repos = append(repos, base...)

	if isGitHubGroup(group) && !contains(repos, JitPack) {
		repos = append(repos, JitPack)
	}

	for _, extra := range ExtraRepositories {
		if extra != "" && !contains(repos, extra) {
			repos = append(repos, extra)
		}
	}
	return repos
}

// isGitHubGroup indica si un grupo Maven pertenece a JitPack (artefactos
// publicados desde GitHub): com.github.* o io.github.*.
func isGitHubGroup(group string) bool {
	return strings.HasPrefix(group, "com.github.") ||
		strings.HasPrefix(group, "io.github.")
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
