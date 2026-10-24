package gradle

import (
	"regexp"
	"strings"
)

// Este archivo tiene las utilidades para leer el Kotlin DSL de Gradle
// (build.gradle.kts) sin ejecutarlo.
//
// El Kotlin DSL es un lenguaje completo, así que un parser real sería un
// proyecto entero. Aquí se hace un parseo textual que cubre lo que se usa en
// la configuración de un módulo Android: bloques, asignaciones y llamadas
// simples. Es suficiente para leer namespace, sdk, versiones y dependencias.

// stripComments devuelve el código con los comentarios sustituidos por
// espacios (se conserva la longitud y los saltos de línea). Así los patrones
// de búsqueda no encuentran falsos positivos dentro de un comentario.
//
// Las cadenas de texto se dejan intactas: su contenido se necesita para leer
// valores.
func stripComments(src string) string {
	out := []byte(src)
	inString := false
	escaped := false

	for i := 0; i < len(src); i++ {
		c := src[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch c {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch {
		case c == '"':
			inString = true
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			// Comentario de línea: hasta el final de la línea.
			for i < len(src) && src[i] != '\n' {
				out[i] = ' '
				i++
			}
			i-- // el bucle exterior incrementa i
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			out[i] = ' '
			i++
			for i < len(src) {
				if src[i] == '*' && i+1 < len(src) && src[i+1] == '/' {
					out[i] = ' '
					out[i+1] = ' '
					i++
					break
				}
				if src[i] != '\n' {
					out[i] = ' '
				}
				i++
			}
		}
	}
	return string(out)
}

// findBlock busca un bloque con el nombre dado ("android", "dependencies"...)
// y devuelve su contenido (sin las llaves exterior).
//
// Se buscan varias ocurrencias porque un mismo nombre puede aparecer en
// distintos sitios (p. ej. "kotlin" dentro de "android" y fuera).
func findBlocks(src, name string) []string {
	src = stripComments(src)
	var blocks []string

	for i := 0; i < len(src); i++ {
		if src[i] != name[0] {
			continue
		}
		// El nombre tiene que ser un identificador completo.
		if i > 0 && isIdentChar(src[i-1]) {
			continue
		}
		if i+len(name) < len(src) && isIdentChar(src[i+len(name)]) {
			continue
		}
		// Debe ir seguido de "{".
		j := i + len(name)
		for j < len(src) && (src[j] == ' ' || src[j] == '\t' || src[j] == '\n' || src[j] == '\r') {
			j++
		}
		if j >= len(src) || src[j] != '{' {
			continue
		}
		end := matchBrace(src, j)
		if end < 0 {
			continue
		}
		blocks = append(blocks, src[j+1:end])
		i = end
	}
	return blocks
}

// findBlock devuelve el primer bloque con el nombre dado.
func findBlock(src, name string) (string, bool) {
	blocks := findBlocks(src, name)
	if len(blocks) == 0 {
		return "", false
	}
	return blocks[0], true
}

// matchBrace devuelve la posición de la llave que cierra la de src[start]
// (que debe ser una "{"), o -1 si no está.
func matchBrace(src string, start int) int {
	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(src); i++ {
		c := src[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch c {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func isIdentChar(c byte) bool {
	return c == '_' || c == '.' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// parseProguardFiles extrae las rutas de archivos de reglas que aparecen en
// las llamadas proguardFiles(...) del archivo de build.
//
// Se ignoran las que no son rutas (las que trae getDefaultProguardFile,
// que apuntan al SDK de Android): rain tiene sus propias reglas base.
func parseProguardFiles(src string) []string {
	var out []string
	for _, args := range findCalls(src, "proguardFiles") {
		for _, arg := range splitStrings(args) {
			if !strings.HasSuffix(arg, ".pro") && !strings.HasSuffix(arg, ".txt") {
				continue
			}
			out = append(out, arg)
		}
	}
	return out
}

// findCalls devuelve el texto de los argumentos de cada llamada a name(...) que
// aparece en src.
//
// A diferencia de un regex, tiene en cuenta los paréntesis anidados: una
// llamada puede ser proguardFiles(getDefaultProguardFile("x.txt"), "y.pro") y
// el regex se pararía en el primer ")".
func findCalls(src, name string) []string {
	var out []string
	for i := 0; i < len(src); {
		j := strings.Index(src[i:], name)
		if j < 0 {
			break
		}
		pos := i + j
		i = pos + len(name)

		// El nombre debe ir solo: no vale "misproguardFiles(".
		if pos > 0 && isIdentChar(src[pos-1]) {
			continue
		}
		// Debe seguir un "(".
		k := i
		for k < len(src) && (src[k] == ' ' || src[k] == '\t' || src[k] == '\r' || src[k] == '\n') {
			k++
		}
		if k >= len(src) || src[k] != '(' {
			continue
		}
		depth := 0
		end := k
		for ; end < len(src); end++ {
			switch src[end] {
			case '(':
				depth++
			case ')':
				depth--
			}
			if depth == 0 {
				break
			}
		}
		if end < len(src) {
			out = append(out, src[k+1:end])
			i = end + 1
		}
	}
	return out
}

// stringValue busca la primera asignación de cadena de una clave.
func stringValue(src, key string) string {
	re := regexp.MustCompile(`(?m)` + regexp.QuoteMeta(key) + `\s*=\s*"([^"]*)"`)
	if m := re.FindStringSubmatch(src); m != nil {
		return m[1]
	}
	// También se acepta la forma "key("valor")" (p. ej. jvmTarget("17")).
	re2 := regexp.MustCompile(regexp.QuoteMeta(key) + `\s*\(\s*"([^"]*)"\s*\)`)
	if m := re2.FindStringSubmatch(src); m != nil {
		return m[1]
	}
	return ""
}

// intValue busca `key = 36` (o `key(36)`) y devuelve el número.
func intValue(src, key string) int {
	re := regexp.MustCompile(`(?m)` + regexp.QuoteMeta(key) + `\s*=\s*(-?\d+)`)
	if m := re.FindStringSubmatch(src); m != nil {
		return atoiSafe(m[1])
	}
	re2 := regexp.MustCompile(regexp.QuoteMeta(key) + `\s*\(\s*(-?\d+)\s*\)`)
	if m := re2.FindStringSubmatch(src); m != nil {
		return atoiSafe(m[1])
	}
	return 0
}

func atoiSafe(s string) int {
	n := 0
	neg := false
	for i := 0; i < len(s); i++ {
		if i == 0 && s[i] == '-' {
			neg = true
			continue
		}
		if s[i] < '0' || s[i] > '9' {
			return 0
		}
		n = n*10 + int(s[i]-'0')
	}
	if neg {
		return -n
	}
	return n
}

// reJavaVersion busca "JavaVersion.VERSION_17" o "JvmTarget.JVM_17".
var reJavaVersion = regexp.MustCompile(`(?:JavaVersion|JvmTarget)\.(?:VERSION_|JVM_)(\d+)`)

// javaVersionExtrae el nivel de Java de una expresión como
// "JavaVersion.VERSION_17" o "org.jetbrains...JvmTarget.JVM_17".
func javaVersion(src string) int {
	if m := reJavaVersion.FindStringSubmatch(src); m != nil {
		return atoiSafe(m[1])
	}
	return 0
}

// splitStatements divide un bloque en sentencias, respetando paréntesis,
// llaves y cadenas.
func splitStatements(src string) []string {
	var out []string
	depth := 0
	inString := false
	escaped := false
	start := 0

	for i := 0; i < len(src); i++ {
		c := src[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch c {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '(', '{', '[':
			depth++
		case ')', '}', ']':
			depth--
		case '\n':
			if depth == 0 {
				stmt := strings.TrimSpace(src[start:i])
				if stmt != "" {
					out = append(out, stmt)
				}
				start = i + 1
			}
		}
	}
	if stmt := strings.TrimSpace(src[start:]); stmt != "" {
		out = append(out, stmt)
	}
	return out
}
