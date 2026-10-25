package pom

import "strings"

// NormalizeVersion colapsa rangos de versión de Maven a una versión concreta.
// [2.1.0]      -> 2.1.0   (rango exacto)
// [1.0,2.0)    -> 1.0     (coge el límite inferior; suficiente para AndroidX)
// (1.0,2.0]    -> ""      (sin límite inferior claro; no soportado)
// 2.1.0        -> 2.1.0   (sin cambios)
func NormalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}

	// Rango con corchete al inicio: [2.1.0], [1.0,2.0), [1.0,2.0]
	if strings.HasPrefix(v, "[") {
		inner := strings.TrimRight(strings.TrimPrefix(v, "["), "]")
		// Si no hay coma, es un rango exacto con un solo valor
		if !strings.Contains(inner, ",") {
			return strings.TrimSpace(inner)
		}
		// Rango con coma: coge el límite inferior
		parts := strings.SplitN(inner, ",", 2)
		lower := strings.TrimSpace(parts[0])
		if lower != "" {
			return lower
		}
		return ""
	}

	// Rango con paréntesis al inicio: (1.0,2.0] -> sin límite inferior
	if strings.HasPrefix(v, "(") {
		return ""
	}

	return v
}
