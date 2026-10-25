package toolchain

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	BuildToolsURL = "https://dl.google.com/android/repository/build-tools_r36_linux.zip"
	PlatformURL   = "https://dl.google.com/android/repository/platform-36_r02.zip"

	R8Version = "9.5.28"
	R8URL     = "https://dl.google.com/dl/android/maven2/com/android/tools/r8/" + R8Version + "/r8-" + R8Version + ".jar"

	KotlinVersion = "2.4.21"
	KotlinURL     = "https://github.com/JetBrains/kotlin/releases/download/v" + KotlinVersion + "/kotlin-compiler-" + KotlinVersion + ".zip"

	// Analizadores estáticos opcionales para "rain lint". No hacen falta para
	// compilar: ktlint revisa el estilo de Kotlin y detekt los "code smells".
	KtlintVersion = "1.8.0"
	KtlintURL     = "https://repo1.maven.org/maven2/com/pinterest/ktlint/ktlint-cli/" + KtlintVersion + "/ktlint-cli-" + KtlintVersion + "-all.jar"

	DetektVersion = "1.23.8"
	DetektURL     = "https://github.com/detekt/detekt/releases/download/v" + DetektVersion + "/detekt-cli-" + DetektVersion + "-all.jar"
)

// Manager gestiona el toolchain propio de Rain en ~/.rain, sin root.
type Manager struct {
	Root   string
	Client *http.Client
	Out    io.Writer
}

func New() (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("no se pudo determinar el HOME: %w", err)
	}
	return &Manager{
		Root:   filepath.Join(home, ".rain"),
		Client: &http.Client{Timeout: 20 * time.Minute},
		Out:    os.Stdout,
	}, nil
}

func (m *Manager) BinDir() string     { return filepath.Join(m.Root, "bin") }
func (m *Manager) ToolDir() string    { return filepath.Join(m.Root, "toolchain") }
func (m *Manager) AndroidJar() string { return filepath.Join(m.ToolDir(), "android.jar") }
func (m *Manager) R8Jar() string      { return filepath.Join(m.ToolDir(), "r8.jar") }
func (m *Manager) ApksignerJar() string {
	return filepath.Join(m.ToolDir(), "apksigner.jar")
}
func (m *Manager) Kotlinc() string { return filepath.Join(m.ToolDir(), "kotlinc", "bin", "kotlinc") }
func (m *Manager) KtlintJar() string {
	return filepath.Join(m.ToolDir(), "ktlint-cli.jar")
}
func (m *Manager) DetektJar() string {
	return filepath.Join(m.ToolDir(), "detekt-cli.jar")
}
func (m *Manager) ComposePlugin() string {
	return filepath.Join(m.ToolDir(), "kotlinc", "lib", "compose-compiler-plugin.jar")
}

// Ensure descarga e instala en ~/.rain solo lo que falte.
func (m *Manager) Ensure(ctx context.Context, complex bool) error {
	if err := os.MkdirAll(m.BinDir(), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(m.ToolDir(), 0o755); err != nil {
		return err
	}

	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"R8 " + R8Version, m.ensureR8},
		{"apksigner", m.ensureApksigner},
		{"android.jar (API 36)", m.ensureAndroidJar},
	}
	if complex {
		steps = append(steps, struct {
			name string
			fn   func(context.Context) error
		}{"Kotlin " + KotlinVersion, m.ensureKotlin})
	}

	for _, step := range steps {
		if err := step.fn(ctx); err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
	}

	return m.writeWrappers(complex)
}

// EnsureLint descarga los analizadores estáticos opcionales (ktlint y detekt)
// en el toolchain de Rain. No son necesarios para compilar: solo los usa
// "rain lint".
func (m *Manager) EnsureLint(ctx context.Context) error {
	if err := os.MkdirAll(m.ToolDir(), 0o755); err != nil {
		return err
	}
	if err := m.ensureKtlint(ctx); err != nil {
		return fmt.Errorf("ktlint: %w", err)
	}
	if err := m.ensureDetekt(ctx); err != nil {
		return fmt.Errorf("detekt: %w", err)
	}
	return nil
}

func (m *Manager) ensureKtlint(ctx context.Context) error {
	if fileExists(m.KtlintJar()) {
		return nil
	}
	return m.download(ctx, KtlintURL, m.KtlintJar(), "ktlint "+KtlintVersion)
}

func (m *Manager) ensureDetekt(ctx context.Context) error {
	if fileExists(m.DetektJar()) {
		return nil
	}
	return m.download(ctx, DetektURL, m.DetektJar(), "detekt "+DetektVersion)
}

func (m *Manager) ensureR8(ctx context.Context) error {
	if fileExists(m.R8Jar()) {
		return nil
	}
	return m.download(ctx, R8URL, m.R8Jar(), "R8 "+R8Version)
}

func (m *Manager) ensureApksigner(ctx context.Context) error {
	if fileExists(m.ApksignerJar()) {
		return nil
	}

	tmp, err := m.downloadTemp(ctx, BuildToolsURL, "build-tools 36")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp) }()

	return extractOne(tmp, func(name string) bool {
		return strings.HasSuffix(name, "/lib/apksigner.jar")
	}, m.ApksignerJar())
}

func (m *Manager) ensureAndroidJar(ctx context.Context) error {
	if fileExists(m.AndroidJar()) {
		return nil
	}

	tmp, err := m.downloadTemp(ctx, PlatformURL, "Android platform 36")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp) }()

	return extractOne(tmp, func(name string) bool {
		return strings.HasSuffix(name, "/android.jar")
	}, m.AndroidJar())
}

func (m *Manager) ensureKotlin(ctx context.Context) error {
	if fileExists(m.Kotlinc()) {
		return nil
	}

	tmp, err := m.downloadTemp(ctx, KotlinURL, "Kotlin "+KotlinVersion)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp) }()

	dest := filepath.Join(m.ToolDir(), "kotlinc")
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	return extractPrefix(tmp, "kotlinc/", dest)
}

// writeWrappers crea los scripts lanzadores de las herramientas de Java.
//
// El heap de la JVM se acota con RAIN_JAVA_MAX_HEAP porque, por defecto, la
// JVM reserva 1/4 de la RAM física: en un contenedor con 2 GB libres y 12 GB de
// máquina eso significa 3 GB reservados y un OOM killer del kernel.
//
//	RAIN_JAVA_MAX_HEAP=1g   fija el heap de todas las herramientas
//	RAIN_JAVA_OPTS="..."    opciones extra (p. ej. -XX:+UseSerialGC)
//
// RefreshWrappers reescribe los lanzadores de ~/.rain/bin cuando su contenido
// ha cambiado. Se llama en cada ejecución de Rain para que una actualización
// aplique los nuevos ajustes (heap de la JVM, variables de entorno) sin
// obligar a ejecutar "rain doctor --fix".
func (m *Manager) RefreshWrappers() error {
	return m.writeWrappers(true)
}

func (m *Manager) writeWrappers(complex bool) error {
	wrappers := map[string]string{
		"d8": "#!/bin/sh\n" +
			"# Lanzador de Rain para d8 (R8).\n" +
			"exec java -Xmx\"${RAIN_JAVA_MAX_HEAP:-1g}\" ${RAIN_JAVA_OPTS:-} " +
			"-cp \"$HOME/.rain/toolchain/r8.jar\" com.android.tools.r8.D8 \"$@\"\n",
		"apksigner": "#!/bin/sh\n" +
			"# Lanzador de Rain para apksigner.\n" +
			"exec java -Xmx\"${RAIN_JAVA_MAX_HEAP:-512m}\" ${RAIN_JAVA_OPTS:-} " +
			"-jar \"$HOME/.rain/toolchain/apksigner.jar\" \"$@\"\n",
	}
	if complex {
		// kotlinc ya respeta JAVA_OPTS y JAVA_TOOL_OPTIONS por su cuenta. Aquí
		// solo se traducen las opciones propias de Rain al formato -J que
		// espera el lanzador de Kotlin.
		wrappers["kotlinc"] = "#!/bin/sh\n" +
			"# Lanzador de Rain para kotlinc.\n" +
			"opts=\"\"\n" +
			"for opt in ${RAIN_JAVA_OPTS:-}; do\n" +
			"\topts=\"$opts -J$opt\"\n" +
			"done\n" +
			"if [ -n \"${RAIN_JAVA_MAX_HEAP:-}\" ]; then\n" +
			"\topts=\"$opts -J-Xmx$RAIN_JAVA_MAX_HEAP\"\n" +
			"fi\n" +
			"exec \"$HOME/.rain/toolchain/kotlinc/bin/kotlinc\" $opts \"$@\"\n"
	}

	if err := os.MkdirAll(m.BinDir(), 0o755); err != nil {
		return err
	}

	for name, content := range wrappers {
		path := filepath.Join(m.BinDir(), name)
		if current, err := os.ReadFile(path); err == nil && string(current) == content {
			continue
		}
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) download(ctx context.Context, url, dest, label string) error {
	tmp, err := m.downloadTemp(ctx, url, label)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp) }()

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

func (m *Manager) downloadTemp(ctx context.Context, url, label string) (string, error) {
	if m.Out != nil {
		fmt.Fprintf(m.Out, "  ↓ %s\n", label)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "rain/"+R8Version)

	resp, err := m.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d al descargar %s", resp.StatusCode, url)
	}

	f, err := os.CreateTemp("", "rain-download-*")
	if err != nil {
		return "", err
	}
	path := f.Name()

	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func extractOne(zipPath string, match func(string) bool, dest string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer func() { _ = zr.Close() }()

	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !match(f.Name) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		tmp := dest + ".part"
		if err := copyZipFile(f, tmp, 0o644); err != nil {
			_ = os.Remove(tmp)
			return err
		}
		return os.Rename(tmp, dest)
	}
	return fmt.Errorf("no se encontró %s dentro del ZIP", dest)
}

func extractPrefix(zipPath, prefix, destDir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer func() { _ = zr.Close() }()

	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, prefix) {
			continue
		}
		rel := strings.TrimPrefix(f.Name, prefix)
		if rel == "" {
			continue
		}
		target, err := safeJoin(destDir, rel)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		mode := f.Mode()
		if mode == 0 {
			mode = 0o644
		}
		if err := copyZipFile(f, target, mode.Perm()); err != nil {
			return err
		}
	}
	return nil
}

func copyZipFile(f *zip.File, dest string, mode os.FileMode) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func safeJoin(base, rel string) (string, error) {
	target := filepath.Join(base, filepath.FromSlash(rel))
	cleanBase := filepath.Clean(base)
	cleanTarget := filepath.Clean(target)
	if cleanTarget != cleanBase && !strings.HasPrefix(cleanTarget, cleanBase+string(os.PathSeparator)) {
		return "", fmt.Errorf("ruta insegura en ZIP: %s", rel)
	}
	return cleanTarget, nil
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}
