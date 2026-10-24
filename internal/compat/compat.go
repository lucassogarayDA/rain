// Package compat codifica la matriz de compatibilidad entre las piezas del
// toolchain (JDK, R8, Kotlin, build-tools/SDK) para que `rain doctor` pueda
// avisar antes de que el usuario se choque con un error críptico de R8/Kotlin.
//
// Origen de las reglas:
//   - R8 declara la versión de Kotlin (metadata) que entiende. R8 < 8.3 no
//     entiende metadata de Kotlin 2.x; usar R8 moderno evita
//     "Unsupported class file major version" y errores de Kotlin metadata.
//   - El bytecode objetivo de Rain es Java 8 (javac --release 8) y Kotlin
//     jvmTarget 1.8, así que el JDK solo necesita ser ≥ 17.
//   - android.jar (compileSdk) debe existir como plataforma instalada.
package compat

import (
	"fmt"
	"strconv"
	"strings"
)

// Severity indica la gravedad de un hallazgo de compatibilidad.
type Severity string

const (
	// Advice es una recomendación que no rompe la compilación.
	Advice Severity = "advice"
	// Warning es una combinación probablemente problemática.
	Warning Severity = "warning"
	// Error es una combinación que fallará casi con seguridad.
	Error Severity = "error"
)

// Issue es un hallazgo concreto de la matriz.
type Issue struct {
	Severity Severity `json:"severity"`
	Topic    string   `json:"topic"`
	Message  string   `json:"message"`
}

// Versions reúne las versiones a comprobar. Los campos vacíos se ignoran.
type Versions struct {
	JDK        string // p. ej. "17.0.20"
	R8         string // p. ej. "9.5.28"
	Kotlin     string // p. ej. "2.4.21"
	BuildTools string // p. ej. "36.0.0"
	CompileSDK int    // p. ej. 36 (0 = desconocido)
}

// mínimos soportados por Rain.
const (
	MinJDK = 17
	// MinR8ForKotlin2x es el primer R8 que entiende la metadata de Kotlin 2.x.
	MinR8ForKotlin2x = 8
)

// CheckMatrix aplica la matriz de compatibilidad y devuelve los hallazgos
// ordenados de más grave a menos.
func CheckMatrix(v Versions) []Issue {
	var issues []Issue

	// --- JDK ---
	if v.JDK != "" {
		major := Major(v.JDK)
		switch {
		case major == 0:
			issues = append(issues, Issue{
				Severity: Warning,
				Topic:    "jdk",
				Message:  fmt.Sprintf("no se pudo interpretar la versión del JDK (%q)", v.JDK),
			})
		case major < MinJDK:
			issues = append(issues, Issue{
				Severity: Error,
				Topic:    "jdk",
				Message:  fmt.Sprintf("JDK %d es demasiado antiguo: Rain requiere Java %d+ (r8/apksigner y Kotlin 2.x lo necesitan)", major, MinJDK),
			})
		}
	}

	// --- R8 vs Kotlin ---
	if v.R8 != "" && v.Kotlin != "" {
		r8Major := Major(v.R8)
		kotlinMajor := kotlinMajorOf(v.Kotlin)
		switch {
		case kotlinMajor >= 2 && r8Major > 0 && r8Major < MinR8ForKotlin2x:
			issues = append(issues, Issue{
				Severity: Error,
				Topic:    "r8-kotlin",
				Message: fmt.Sprintf(
					"R8 %s no entiende la metadata de Kotlin %s: actualiza R8 a 8.3+ o usa Kotlin 1.9",
					v.R8, v.Kotlin),
			})
		case kotlinMajor >= 2 && r8Major >= MinR8ForKotlin2x:
			issues = append(issues, Issue{
				Severity: Advice,
				Topic:    "r8-kotlin",
				Message:  fmt.Sprintf("R8 %s + Kotlin %s: combinación moderna soportada", v.R8, v.Kotlin),
			})
		}
	}

	// --- Compile SDK ---
	if v.CompileSDK > 0 && v.CompileSDK < 33 {
		issues = append(issues, Issue{
			Severity: Warning,
			Topic:    "sdk",
			Message:  fmt.Sprintf("compileSdk %d es antiguo: se recomienda 34+ para librerías AndroidX/Compose actuales", v.CompileSDK),
		})
	}

	// --- build-tools vs compileSdk ---
	if v.BuildTools != "" && v.CompileSDK > 0 {
		btMajor := Major(v.BuildTools)
		if btMajor > 0 && btMajor < v.CompileSDK-1 {
			issues = append(issues, Issue{
				Severity: Warning,
				Topic:    "build-tools",
				Message: fmt.Sprintf(
					"build-tools %s es más antiguo que compileSdk %d: usa build-tools %d+",
					v.BuildTools, v.CompileSDK, v.CompileSDK),
			})
		}
	}

	return issues
}

// HasBlocking indica si algún hallazgo es de severidad Error.
func HasBlocking(issues []Issue) bool {
	for _, issue := range issues {
		if issue.Severity == Error {
			return true
		}
	}
	return false
}

// Major extrae la parte mayor de una versión ("2.4.21" -> 2, "1.8.0_392" -> 8).
func Major(version string) int {
	version = strings.TrimSpace(version)
	if version == "" {
		return 0
	}
	parts := strings.FieldsFunc(version, func(r rune) bool {
		return r == '.' || r == '_' || r == '-' || r == '+'
	})
	if len(parts) == 0 {
		return 0
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0
	}
	// Esquema antiguo de Java 1.8.0 -> 8
	if major == 1 && len(parts) > 1 {
		if legacy, err := strconv.Atoi(parts[1]); err == nil {
			return legacy
		}
	}
	return major
}

// kotlinMajorOf extrae la mayor de Kotlin sin la regla heredada de Java
// (Kotlin 1.9 es mayor 1, no 9).
func kotlinMajorOf(version string) int {
	version = strings.TrimSpace(version)
	if version == "" {
		return 0
	}
	if dot := strings.IndexByte(version, '.'); dot >= 0 {
		version = version[:dot]
	}
	major, _ := strconv.Atoi(version)
	return major
}
