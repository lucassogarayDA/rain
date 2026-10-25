package build

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Tests de integración del ejemplo Compose.
//
// Están escritos solo en Go: compilan el binario de Rain y ejecutan builds
// reales del ejemplo, sin emulador ni framework de tests de Android. Se
// saltan por defecto porque necesitan toolchain Android, las dependencias ya
// descargadas en examples/compose/.rain-deps y varios minutos de CPU.
//
//	RAIN_COMPOSE_IT=1 go test ./internal/build -run TestEjemploCompose -v
//
// El segundo test cambia MainScreen.kt y comprueba que:
//   - el DEX de dependencias se reutiliza (misma marca y mismos ficheros),
//   - el DEX de la app se regenera,
//   - el cambio aparece en el APK.
//
// El objetivo es que la caché de DEX de dependencias no se rompa sin que los
// tests se enteren; el tiempo se registra en el log, pero no se usa como
// aserción porque varía con la carga de la máquina.

var (
	composeRainOnce sync.Once
	composeRainBin  string
	composeRainErr  error
)

func requireComposeIT(t *testing.T) {
	t.Helper()
	if os.Getenv("RAIN_COMPOSE_IT") == "" {
		t.Skip("test de integración del ejemplo Compose; define RAIN_COMPOSE_IT=1 para ejecutarlo")
	}
}

func composeRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

// composeRainBinary compila Rain una vez por proceso de test. Es importante
// que los dos tests usen el mismo binario: la clave del DEX de dependencias
// incluye el ejecutable, así que recompilarlo entre tests invalidaría la caché
// que precisamente se quiere comprobar.
func composeRainBinary(t *testing.T, repoRoot string) string {
	t.Helper()
	composeRainOnce.Do(func() {
		dir, err := os.MkdirTemp("", "rain-compose-it-")
		if err != nil {
			composeRainErr = err
			return
		}
		composeRainBin = filepath.Join(dir, "rain")
		cmd := exec.Command("go", "build", "-o", composeRainBin, ".")
		cmd.Dir = repoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			composeRainErr = fmt.Errorf("compilando rain: %w\n%s", err, out)
		}
	})
	if composeRainErr != nil {
		t.Fatal(composeRainErr)
	}
	return composeRainBin
}

func runComposeBuild(t *testing.T, bin, exampleDir string) (string, time.Duration) {
	t.Helper()
	start := time.Now()
	cmd := exec.Command(bin, "--devlog", "build")
	cmd.Dir = exampleDir
	out, err := cmd.CombinedOutput()
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("rain build: %v (tras %s)\n%s", err, elapsed, out)
	}
	return string(out), elapsed
}

// apkDexBytes concatena los classes*.dex del APK. Para estas comprobaciones
// basta con buscar descriptores y cadenas: el DEX guarda ambos en su tabla de
// strings.
func apkDexBytes(t *testing.T, apk string) []byte {
	t.Helper()
	zr, err := zip.OpenReader(apk)
	if err != nil {
		t.Fatalf("abriendo %s: %v", apk, err)
	}
	defer func() { _ = zr.Close() }()

	var dex bytes.Buffer
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "classes") || !strings.HasSuffix(f.Name, ".dex") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("abriendo %s: %v", f.Name, err)
		}
		if _, err := io.Copy(&dex, rc); err != nil {
			_ = rc.Close()
			t.Fatalf("leyendo %s: %v", f.Name, err)
		}
		_ = rc.Close()
	}
	return dex.Bytes()
}

func leerFichero(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("leyendo %s: %v", path, err)
	}
	return data
}

// mtimesDeCacheDexDevuelve el mtime de los DEX y de la marca del DEX de
// dependencias. Si la caché se reutiliza, no debe cambiar ninguno.
func mtimesDeCacheDex(t *testing.T, dir string) map[string]time.Time {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("leyendo %s: %v", dir, err)
	}
	out := make(map[string]time.Time)
	for _, e := range entries {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), ".dex") && e.Name() != dexDepsHashFile) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			t.Fatalf("stat de %s: %v", e.Name(), err)
		}
		out[e.Name()] = info.ModTime()
	}
	return out
}

func igualesMtimes(a, b map[string]time.Time) bool {
	if len(a) != len(b) {
		return false
	}
	for name, when := range a {
		other, ok := b[name]
		if !ok || !other.Equal(when) {
			return false
		}
	}
	return true
}

func TestEjemploComposeBuild(t *testing.T) {
	requireComposeIT(t)
	toolchainIntegracion(t)

	repo := composeRepoRoot(t)
	exampleDir := filepath.Join(repo, "examples", "compose")
	depsCache := filepath.Join(exampleDir, ".rain-deps", ".rain-deps.json")
	if !fileExists(depsCache) {
		t.Skip("el ejemplo no tiene .rain-deps; ejecuta 'rain build' una vez en examples/compose")
	}

	bin := composeRainBinary(t, repo)
	out, elapsed := runComposeBuild(t, bin, exampleDir)
	t.Logf("build del ejemplo: %s", elapsed)

	if !strings.Contains(out, "APK generado") {
		t.Errorf("el build no confirmó el APK:\n%s", out)
	}
	apk := filepath.Join(exampleDir, "app", "build", "app-debug.apk")
	dex := apkDexBytes(t, apk)
	for _, want := range []string{
		"Lcom/kyant/backdrop/catalog/MainScreenKt;",
		"Presióname",
	} {
		if !bytes.Contains(dex, []byte(want)) {
			t.Errorf("el DEX del APK no contiene %q", want)
		}
	}
	if !fileExists(filepath.Join(exampleDir, "app", "build", "dexdeps", "classes.dex")) {
		t.Error("no se generó el DEX de dependencias cacheado")
	}
}

func TestEjemploComposeCambioDeCodigo(t *testing.T) {
	requireComposeIT(t)
	toolchainIntegracion(t)

	repo := composeRepoRoot(t)
	exampleDir := filepath.Join(repo, "examples", "compose")
	depsCache := filepath.Join(exampleDir, ".rain-deps", ".rain-deps.json")
	if !fileExists(depsCache) {
		t.Skip("el ejemplo no tiene .rain-deps; ejecuta 'rain build' una vez en examples/compose")
	}

	src := filepath.Join(exampleDir, "app", "java", "com", "kyant", "backdrop", "catalog", "MainScreen.kt")
	bin := composeRainBinary(t, repo)

	// Si una ejecución anterior falló a mitad, se parte siempre del texto
	// original para que el test sea repetible.
	original := string(leerFichero(t, src))
	limpio := strings.ReplaceAll(original, `Text("Presióname ahora")`, `Text("Presióname")`)
	t.Cleanup(func() {
		if err := os.WriteFile(src, []byte(limpio), 0o644); err != nil {
			t.Errorf("restaurando %s: %v", src, err)
		}
	})
	// El build base parte siempre del texto limpio: si una ejecución anterior
	// murió después de cambiar el fichero, sin esto el "cambio" del test sería
	// un no-op y el DEX de la app no se regeneraría.
	if err := os.WriteFile(src, []byte(limpio), 0o644); err != nil {
		t.Fatal(err)
	}

	// Build base: deja el DEX de dependencias y el de la app al día.
	baseOut, baseElapsed := runComposeBuild(t, bin, exampleDir)
	t.Logf("build base: %s", baseElapsed)
	if !strings.Contains(baseOut, "APK generado") {
		t.Errorf("el build base no confirmó el APK:\n%s", baseOut)
	}

	depsDir := filepath.Join(exampleDir, "app", "build", "dexdeps")
	appDexDir := filepath.Join(exampleDir, "app", "build", "dex")
	dexDepsHash := filepath.Join(depsDir, dexDepsHashFile)
	appDexHash := filepath.Join(exampleDir, "app", "build", dexHashFile)

	depsBefore := mtimesDeCacheDex(t, depsDir)
	depsHashBefore := string(leerFichero(t, dexDepsHash))
	appHashBefore := string(leerFichero(t, appDexHash))
	appDexBefore, err := os.Stat(filepath.Join(appDexDir, "classes.dex"))
	if err != nil {
		t.Fatalf("stat del DEX de la app: %v", err)
	}

	// Cambio de código deliberado: el texto del botón.
	cambiado := strings.Replace(limpio, `Text("Presióname")`, `Text("Presióname ahora")`, 1)
	if cambiado == limpio {
		t.Fatal("no se encontró el texto del botón en MainScreen.kt")
	}
	if err := os.WriteFile(src, []byte(cambiado), 0o644); err != nil {
		t.Fatal(err)
	}

	out, elapsed := runComposeBuild(t, bin, exampleDir)
	t.Logf("build con cambio de código: %s", elapsed)

	if !strings.Contains(out, "dependencias sin cambios, reutilizando su DEX") {
		t.Errorf("no se reutilizó el DEX de dependencias:\n%s", out)
	}
	if got := mtimesDeCacheDex(t, depsDir); !igualesMtimes(depsBefore, got) {
		t.Errorf("el DEX de dependencias se regeneró: antes=%v ahora=%v", depsBefore, got)
	}
	if got := string(leerFichero(t, dexDepsHash)); got != depsHashBefore {
		t.Errorf("cambió la marca del DEX de dependencias: %s != %s", depsHashBefore, got)
	}
	if got := string(leerFichero(t, appDexHash)); got == appHashBefore {
		t.Error("la marca del DEX de la app no cambió tras modificar el código")
	}
	appDexAfter, err := os.Stat(filepath.Join(appDexDir, "classes.dex"))
	if err != nil {
		t.Fatalf("stat del DEX de la app: %v", err)
	}
	if appDexAfter.ModTime().Equal(appDexBefore.ModTime()) {
		t.Error("el DEX de la app no se regeneró")
	}

	dex := apkDexBytes(t, filepath.Join(exampleDir, "app", "build", "app-debug.apk"))
	if !bytes.Contains(dex, []byte("Presióname ahora")) {
		t.Error("el APK no contiene el texto nuevo: el build no aplicó el cambio")
	}
}
