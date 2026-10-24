package build

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/itskreisler/rain/internal/ui"
)

// stampFile guarda la fecha de la última compilación correcta. Sirve para
// saber qué fuentes han cambiado y así poder hacer un chequeo incremental.
const stampFile = ".rain-sources-stamp"

// sourcesStamp devuelve la fecha de la última compilación correcta.
func sourcesStamp(buildDir string) (time.Time, error) {
	data, err := os.ReadFile(filepath.Join(buildDir, stampFile))
	if err != nil {
		return time.Time{}, err
	}
	unix, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(unix, 0), nil
}

// writeSourcesStamp marca el momento actual como última compilación correcta.
func writeSourcesStamp(buildDir string) error {
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	return os.WriteFile(filepath.Join(buildDir, stampFile), []byte(stamp), 0644)
}

// declPatterns captura los nombres de las declaraciones de primer nivel de un
// fichero Kotlin. Se usan para saber qué ficheros dependen de los que han
// cambiado: si se renombra o cambia la firma de una función, hay que
// recompilar también a quien la usa.
var declPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?m)^\s*(?:@\w+(?:\([^)]*\))?\s+)*(?:public\s+|internal\s+|private\s+|protected\s+|open\s+|abstract\s+|sealed\s+|data\s+|value\s+|enum\s+|annotation\s+|inline\s+|suspend\s+|operator\s+|infix\s+|external\s+|expect\s+|actual\s+|tailrec\s+|const\s+|lateinit\s+|override\s+|final\s+|actual\s+)*(?:fun|class|object|interface|typealias|enum\s+class|value\s+class)\s+(?:<[^>]*>\s*)?([A-Za-z_][A-Za-z0-9_]*)`),
	regexp.MustCompile(`(?m)^\s*(?:public\s+|internal\s+|private\s+|protected\s+|const\s+|lateinit\s+|open\s+|override\s+|final\s+|@\w+(?:\([^)]*\))?\s+)*(?:val|var)\s+(?:<[^>]*>\s*)?([A-Za-z_][A-Za-z0-9_]*)`),
}

// declarations devuelve los nombres declarados en un fichero Kotlin.
func declarations(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	text := string(data)
	seen := map[string]bool{}
	var out []string
	for _, re := range declPatterns {
		for _, m := range re.FindAllStringSubmatch(text, -1) {
			name := m[1]
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// changedSources devuelve los ficheros modificados desde stamp y los que
// dependen de ellos (los que mencionan alguna de sus declaraciones).
func changedSources(cfg Config, stamp time.Time) (changed []string, dependents []string, err error) {
	root := sourceRoot(cfg.AppDir)
	if root == "" {
		return nil, nil, nil
	}

	var all []string
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".kt" && ext != ".java" {
			return nil
		}
		all = append(all, path)
		if !stamp.IsZero() && info.ModTime().After(stamp) {
			changed = append(changed, path)
		}
		return nil
	})
	if err != nil && os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if len(changed) == 0 {
		return nil, nil, nil
	}

	// Nombres declarados en los ficheros modificados.
	names := map[string]bool{}
	for _, path := range changed {
		for _, name := range declarations(path) {
			names[name] = true
		}
	}
	if len(names) == 0 {
		return changed, nil, nil
	}

	// Ficheros que mencionan esos nombres: son los que pueden romperse.
	for _, path := range all {
		if containsPath(changed, path) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := string(data)
		for name := range names {
			if mentionsIdentifier(text, name) {
				dependents = append(dependents, path)
				break
			}
		}
	}

	return changed, dependents, nil
}

// mentionsIdentifier busca un identificador completo en el texto, sin que
// forme parte de otro más largo.
func mentionsIdentifier(text, name string) bool {
	idx := 0
	for {
		i := strings.Index(text[idx:], name)
		if i < 0 {
			return false
		}
		i += idx
		before := byte(' ')
		if i > 0 {
			before = text[i-1]
		}
		after := byte(' ')
		if i+len(name) < len(text) {
			after = text[i+len(name)]
		}
		if !isIdentChar(before) && !isIdentChar(after) {
			return true
		}
		idx = i + len(name)
	}
}

func isIdentChar(c byte) bool {
	return c == '_' || c == '$' ||
		(c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9')
}

func containsPath(list []string, path string) bool {
	for _, p := range list {
		if p == path {
			return true
		}
	}
	return false
}

// describeFastCheck resume qué se va a recompilar, para el log.
func describeFastCheck(changed, dependents []string) string {
	msg := fmt.Sprintf("%d fichero(s) modificado(s)", len(changed))
	if len(dependents) > 0 {
		msg += fmt.Sprintf(" y %d dependiente(s)", len(dependents))
	}
	return msg
}

// fastCheck hace un chequeo de tipos incremental: recompila solo los ficheros
// modificados desde la última compilación correcta (y los que dependen de
// ellos) reutilizando las clases ya compiladas en objDir.
//
// Es mucho más rápido que el chequeo completo, pero no es tan exhaustivo:
// un error provocado en un fichero que no se recompila puede no aparecer.
// Para una comprobación completa hay que usar "rain check" sin --fast.
func fastCheck(cfg Config, genDir string) error {
	objDir := filepath.Join(cfg.BuildDir, "obj")
	if err := os.MkdirAll(objDir, 0755); err != nil {
		return fmt.Errorf("creando directorio de objetos: %w", err)
	}

	// Sin clases compiladas no hay contra qué compilar: se cae al chequeo
	// completo para no dar falsos errores.
	if !hasClassFiles(objDir) {
		ui.Info("sin clases compiladas previamente: se comprueba todo")
		if _, err := compileSources(cfg, genDir, objDir); err != nil {
			return err
		}
		if err := writeSourcesStamp(cfg.BuildDir); err != nil {
			return fmt.Errorf("guardando la marca de compilación: %w", err)
		}
		ui.Info("chequeo de tipos completado: sin errores")
		return nil
	}

	stamp, err := sourcesStamp(cfg.BuildDir)
	if err != nil {
		ui.VerboseLog("sin marca de compilación previa: se comprueba todo")
		stamp = time.Time{}
	}

	changed, dependents, err := changedSources(cfg, stamp)
	if err != nil {
		return fmt.Errorf("buscando los ficheros modificados: %w", err)
	}
	if len(changed) == 0 {
		ui.Info("nada modificado desde la última compilación correcta")
		return nil
	}

	ui.Info("chequeo incremental: %s", describeFastCheck(changed, dependents))

	sources := append(append([]string{}, changed...), dependents...)
	var kotlinFiles, javaFiles []string
	for _, path := range sources {
		switch filepath.Ext(path) {
		case ".kt":
			kotlinFiles = append(kotlinFiles, path)
		case ".java":
			javaFiles = append(javaFiles, path)
		}
	}

	if _, err := compileSourceSet(cfg, genDir, objDir, kotlinFiles, javaFiles); err != nil {
		return err
	}
	if err := writeSourcesStamp(cfg.BuildDir); err != nil {
		return fmt.Errorf("guardando la marca de compilación: %w", err)
	}
	ui.Info("chequeo de tipos completado: sin errores")
	return nil
}

// hasClassFiles indica si el directorio de objetos tiene clases compiladas.
func hasClassFiles(objDir string) bool {
	found := false
	_ = filepath.Walk(objDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if found || info == nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ".class") {
			found = true
		}
		return nil
	})
	return found
}
