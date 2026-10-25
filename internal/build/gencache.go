package build

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/itskreisler/rain/internal/ui"
)

// El Java generado (R.java de cada librería, BuildConfig.java) solo cambia
// cuando cambian los recursos o la configuración del build. Compilarlo con
// javac en cada build con cambios de código es tirar segundos a la basura:
// en un proyecto Compose son decenas de R.java con la tabla completa de
// recursos. Se guardan las clases resultantes y se reutilizan mientras la
// entrada no cambie.
const (
	genClassesDir  = "genclasses"
	genClassesHash = ".rain-gen-hash"
)

// compileGeneratedJava compila el Java generado dentro de objDir, reutilizando
// las clases de la compilación anterior si el conjunto de fuentes y el
// classpath no han cambiado.
func compileGeneratedJava(cfg Config, objDir string, sources []string) error {
	hash, err := generatedJavaHash(cfg, sources)
	if err != nil {
		return err
	}

	cacheDir := filepath.Join(cfg.BuildDir, genClassesDir)
	hashPath := filepath.Join(cfg.BuildDir, genClassesHash)

	if prev, err := os.ReadFile(hashPath); err == nil && string(prev) == hash && hasClassFiles(cacheDir) {
		ui.VerboseLog("Java generado sin cambios, reutilizando %s", cacheDir)
		return copyTree(cacheDir, objDir)
	}

	// Se compila en un directorio aparte para poder reutilizarlo después. Si
	// javac falla, la caché anterior se descarta y no se reutilizará.
	if err := os.RemoveAll(cacheDir); err != nil {
		return err
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}

	var genClasspath []string
	genClasspath = append(genClasspath, cfg.AndroidJar)
	genClasspath = append(genClasspath, cfg.libraryJars()...)
	genArgs := []string{"-d", cacheDir, "--release", fmt.Sprint(cfg.jvmTarget()), "-encoding", "UTF-8", "-Xlint:-options",
		"-classpath", strings.Join(genClasspath, ":")}
	genArgs = append(genArgs, sources...)

	var genErr bytes.Buffer
	genCmd := exec.Command("javac", genArgs...)
	genCmd.Stdout = os.Stdout
	genCmd.Stderr = &genErr
	if err := genCmd.Run(); err != nil {
		ui.Error("Errores de compilación Java (generado):")
		fmt.Println(genErr.String())
		return fmt.Errorf("javac falló")
	}

	if err := os.WriteFile(hashPath, []byte(hash), 0o644); err != nil {
		return fmt.Errorf("guardando hash del Java generado: %w", err)
	}
	return copyTree(cacheDir, objDir)
}

// generatedJavaHash resume todo lo que influye en las clases generadas: el
// contenido de las fuentes, el target de JVM y los jars contra los que se
// compilan.
func generatedJavaHash(cfg Config, sources []string) (string, error) {
	h := sha256.New()
	fmt.Fprintf(h, "rain-gen-v1\n%d\n", cfg.jvmTarget())
	for _, src := range sources {
		data, err := os.ReadFile(src)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s\n", filepath.Base(src))
		sum := sha256.Sum256(data)
		h.Write(sum[:])
	}
	classpath := append([]string{cfg.AndroidJar}, cfg.libraryJars()...)
	for _, path := range classpath {
		st, err := os.Stat(path)
		if err != nil {
			continue
		}
		fmt.Fprintf(h, "%s|%d|%d\n", path, st.Size(), st.ModTime().UnixNano())
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// copyTree copia src dentro de dst conservando la estructura de directorios.
// Se copia (en vez de enlazar) porque javac puede reescribir clases en objDir
// y no debe contaminar la caché.
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = in.Close() }()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			_ = out.Close()
			return err
		}
		return out.Close()
	})
}
