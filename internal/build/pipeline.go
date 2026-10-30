package build

import (
"bytes"
"crypto/sha256"
"fmt"
"hash"
"io"
"os"
"os/exec"
"path/filepath"
"sort"
"strings"
"time"

"rain/internal/config"
"rain/internal/ui"
)

type Config struct {
AppDir     string
BuildDir   string
LibsDir    string
AndroidJar string
Package    string
Release    bool
Signing    *config.Signing
}

func hashFile(path string) ([]byte, error) {
f, err := os.Open(path)
if err != nil {
return nil, err
}
defer f.Close()
h := sha256.New()
if _, err := io.Copy(h, f); err != nil {
return nil, err
}
return h.Sum(nil), nil
}

func writeFileHash(h hash.Hash, name, path string) error {
	sum, err := hashFile(path)
	if err != nil {
		return fmt.Errorf("hasheando %s: %w", path, err)
	}
	_, _ = h.Write([]byte(name))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(sum)
	_, _ = h.Write([]byte{0})
	return nil
}

func computeResourceHash(appDir, androidJar string) (string, error) {
	var files []string
	resDir := filepath.Join(appDir, "src/main/res")
	manifest := filepath.Join(appDir, "src/main/AndroidManifest.xml")

	if err := filepath.Walk(resDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info == nil {
			return fmt.Errorf("información de archivo vacía para %s", path)
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return "", fmt.Errorf("recorriendo recursos %s: %w", resDir, err)
	}
	sort.Strings(files)

	global := sha256.New()
	for _, file := range files {
		rel, err := filepath.Rel(resDir, file)
		if err != nil {
			return "", err
		}
		if err := writeFileHash(global, filepath.ToSlash(rel), file); err != nil {
			return "", err
		}
	}

	if err := writeFileHash(global, "AndroidManifest.xml", manifest); err != nil {
		return "", fmt.Errorf("hash del manifest: %w", err)
	}
	if err := writeFileHash(global, "android.jar", androidJar); err != nil {
		return "", fmt.Errorf("hash de android.jar para recursos: %w", err)
	}
	return fmt.Sprintf("%x", global.Sum(nil)), nil
}

func computeJavaHash(appDir, libsDir, genDir, androidJar string, release bool) (string, error) {
	global := sha256.New()
	if release {
		_, _ = global.Write([]byte("release"))
	} else {
		_, _ = global.Write([]byte("debug"))
	}
	_, _ = global.Write([]byte("kotlinc-jvm-target=1.8;javac-source=8;javac-target=8;d8-min-api=21"))
	_, _ = global.Write([]byte{0})

	var sources []string
	srcDir := filepath.Join(appDir, "src/main")
	if err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info == nil {
			return fmt.Errorf("información de archivo vacía para %s", path)
		}
		if !info.IsDir() && (strings.HasSuffix(path, ".java") || strings.HasSuffix(path, ".kt")) {
			sources = append(sources, path)
		}
		return nil
	}); err != nil {
		return "", fmt.Errorf("recorriendo fuentes %s: %w", srcDir, err)
	}
	sort.Strings(sources)
	for _, file := range sources {
		rel, err := filepath.Rel(appDir, file)
		if err != nil {
			return "", err
		}
		if err := writeFileHash(global, "source/"+filepath.ToSlash(rel), file); err != nil {
			return "", err
		}
	}

	var generated []string
	if err := filepath.Walk(genDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info == nil {
			return fmt.Errorf("información de archivo vacía para %s", path)
		}
		if !info.IsDir() && strings.HasSuffix(path, ".java") {
			generated = append(generated, path)
		}
		return nil
	}); err != nil {
		return "", fmt.Errorf("recorriendo fuentes generadas %s: %w", genDir, err)
	}
	sort.Strings(generated)
	for _, file := range generated {
		rel, err := filepath.Rel(genDir, file)
		if err != nil {
			return "", err
		}
		if err := writeFileHash(global, "generated/"+filepath.ToSlash(rel), file); err != nil {
			return "", err
		}
	}

	entries, err := os.ReadDir(libsDir)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("leyendo bibliotecas %s: %w", libsDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".jar") {
			continue
		}
		file := filepath.Join(libsDir, entry.Name())
		if err := writeFileHash(global, "library/"+entry.Name(), file); err != nil {
			return "", err
		}
	}

	if err := writeFileHash(global, "android.jar", androidJar); err != nil {
		return "", fmt.Errorf("hash de android.jar: %w", err)
	}

	return fmt.Sprintf("%x", global.Sum(nil)), nil
}

func Run(cfg Config) error {
start := time.Now()
if err := os.MkdirAll(cfg.BuildDir, 0755); err != nil {
return fmt.Errorf("creando directorio de build: %w", err)
}

genDir := filepath.Join(cfg.BuildDir, "gen")
if err := os.MkdirAll(genDir, 0755); err != nil {
return fmt.Errorf("creando directorio generado: %w", err)
}

// --- Recursos ---
resHashPath := filepath.Join(cfg.BuildDir, ".rain-res-hash")
flatDir := filepath.Join(cfg.BuildDir, "res")
baseApk := filepath.Join(cfg.BuildDir, "base.apk")

currentResHash, err := computeResourceHash(cfg.AppDir, cfg.AndroidJar)
if err != nil {
return fmt.Errorf("hash de recursos: %w", err)
}
prevResHash, _ := os.ReadFile(resHashPath)
resUpToDate := string(prevResHash) == currentResHash && fileExists(baseApk)

if resUpToDate {
ui.Info("recursos sin cambios, reutilizando APK base")
} else {
ui.Info("compilando recursos...")
resDir := filepath.Join(cfg.AppDir, "src/main/res")

// El directorio contiene exclusivamente salidas de aapt2 compile.
// Se limpia antes de recompilar para que recursos eliminados o renombrados
// no permanezcan como archivos .flat obsoletos en el siguiente enlace.
if err := os.RemoveAll(flatDir); err != nil {
return fmt.Errorf("limpiando recursos compilados: %w", err)
}
if err := os.MkdirAll(flatDir, 0755); err != nil {
return fmt.Errorf("creando directorio de recursos compilados: %w", err)
}

cmd := exec.Command("aapt2", "compile", "--dir", resDir, "-o", flatDir)
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
if err := cmd.Run(); err != nil {
return fmt.Errorf("aapt2 compile: %w", err)
}

ui.Info("enlazando recursos...")
manifest := filepath.Join(cfg.AppDir, "src/main/AndroidManifest.xml")
args := []string{"link", "-o", baseApk, "-I", cfg.AndroidJar,
"--manifest", manifest, "--java", genDir}
entries, err := os.ReadDir(flatDir)
if err != nil {
return fmt.Errorf("leyendo recursos compilados: %w", err)
}
for _, e := range entries {
if strings.HasSuffix(e.Name(), ".flat") {
args = append(args, filepath.Join(flatDir, e.Name()))
}
}
cmd = exec.Command("aapt2", args...)
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
if err := cmd.Run(); err != nil {
return fmt.Errorf("aapt2 link: %w", err)
}
if err := os.WriteFile(resHashPath, []byte(currentResHash), 0644); err != nil {
return fmt.Errorf("guardando hash de recursos: %w", err)
}
}

// --- Generar BuildConfig.java ---
buildConfigDir := filepath.Join(genDir, strings.ReplaceAll(cfg.Package, ".", "/"))
os.MkdirAll(buildConfigDir, 0755)

debugValue := "true"
buildType := "debug"
if cfg.Release {
debugValue = "false"
buildType = "release"
}

buildConfigContent := fmt.Sprintf(`package %s;

public final class BuildConfig {
    public static final boolean DEBUG = %s;
    public static final String APPLICATION_ID = "%s";
    public static final String BUILD_TYPE = "%s";
    public static final String VERSION_NAME = "1.0";
}
`, cfg.Package, debugValue, cfg.Package, buildType)

buildConfigPath := filepath.Join(buildConfigDir, "BuildConfig.java")
if err := os.WriteFile(buildConfigPath, []byte(buildConfigContent), 0644); err != nil {
return fmt.Errorf("generando BuildConfig: %w", err)
}

// --- Java / DEX ---
javaHashPath := filepath.Join(cfg.BuildDir, ".rain-java-hash")
dexPath := filepath.Join(cfg.BuildDir, "dex", "classes.dex")

currentJavaHash, err := computeJavaHash(cfg.AppDir, cfg.LibsDir, genDir, cfg.AndroidJar, cfg.Release)
if err != nil {
return fmt.Errorf("hash de java: %w", err)
}
prevJavaHash, _ := os.ReadFile(javaHashPath)
javaUpToDate := string(prevJavaHash) == currentJavaHash && fileExists(dexPath)

if javaUpToDate {
ui.Info("java sin cambios, reutilizando classes.dex")
} else {
objDir := filepath.Join(cfg.BuildDir, "obj")
os.MkdirAll(objDir, 0755)

// --- Compilar Kotlin (si hay archivos .kt) ---
var kotlinSources []string
filepath.Walk(filepath.Join(cfg.AppDir, "src/main"), func(path string, info os.FileInfo, err error) error {
if strings.HasSuffix(path, ".kt") {
kotlinSources = append(kotlinSources, path)
}
return nil
})

if len(kotlinSources) > 0 {
ui.Info("compilando Kotlin (%d archivos)...", len(kotlinSources))

var kotlinClasspath []string
kotlinClasspath = append(kotlinClasspath, cfg.AndroidJar)
libEntries, _ := os.ReadDir(cfg.LibsDir)
for _, e := range libEntries {
if strings.HasSuffix(e.Name(), ".jar") {
kotlinClasspath = append(kotlinClasspath, filepath.Join(cfg.LibsDir, e.Name()))
}
}
kotlinClasspath = append(kotlinClasspath, genDir)

kotlincArgs := []string{"-J-Dlibrary.jansi.path=" + os.Getenv("PREFIX") + "/lib/jansi",
"-d", objDir,
"-classpath", strings.Join(kotlinClasspath, ":"),
"-jvm-target", "1.8"}
kotlincArgs = append(kotlincArgs, kotlinSources...)

var kotlinStderr bytes.Buffer
cmd := exec.Command("kotlinc", kotlincArgs...)
cmd.Stdout = os.Stdout
cmd.Stderr = &kotlinStderr
if err := cmd.Run(); err != nil {
ui.Error("Errores de compilación Kotlin:")
fmt.Println(kotlinStderr.String())
return fmt.Errorf("kotlinc falló")
}
ui.Info("Kotlin compilado")
}

ui.Info("compilando Java...")
var sources []string
filepath.Walk(filepath.Join(cfg.AppDir, "src/main"), func(path string, info os.FileInfo, err error) error {
if strings.HasSuffix(path, ".java") {
sources = append(sources, path)
}
return nil
})
filepath.Walk(genDir, func(path string, info os.FileInfo, err error) error {
if strings.HasSuffix(path, ".java") {
sources = append(sources, path)
}
return nil
})

var classpath []string
classpath = append(classpath, cfg.AndroidJar)
classpath = append(classpath, objDir) // Incluir clases de Kotlin ya compiladas
libEntries, _ := os.ReadDir(cfg.LibsDir)
for _, e := range libEntries {
if strings.HasSuffix(e.Name(), ".jar") {
classpath = append(classpath, filepath.Join(cfg.LibsDir, e.Name()))
}
}

javacArgs := []string{"-d", objDir, "-source", "8", "-target", "8", "-Xlint:-options",
"-classpath", strings.Join(classpath, ":")}
javacArgs = append(javacArgs, sources...)

var stderrBuf bytes.Buffer
cmd := exec.Command("javac", javacArgs...)
cmd.Stdout = os.Stdout
cmd.Stderr = &stderrBuf
if err := cmd.Run(); err != nil {
ui.Error("Errores de compilación Java:")
fmt.Println(stderrBuf.String())
return fmt.Errorf("javac falló")
}

var classFiles []string
filepath.Walk(objDir, func(path string, info os.FileInfo, err error) error {
if strings.HasSuffix(path, ".class") {
classFiles = append(classFiles, path)
}
return nil
})

dexDir := filepath.Join(cfg.BuildDir, "dex")
os.MkdirAll(dexDir, 0755)

if cfg.Release {
ui.Info("generando DEX con R8 (release)...")
d8Jar := findD8Jar()
if d8Jar == "" {
return fmt.Errorf("no se encontró d8.jar para R8")
}
proguardFile := filepath.Join(cfg.BuildDir, "proguard.pro")
if !fileExists(proguardFile) {
os.WriteFile(proguardFile, []byte("-keep public class * extends android.app.Activity\n-dontwarn **\n"), 0644)
}
r8Args := []string{"-cp", d8Jar, "com.android.tools.r8.R8",
"--release", "--output", dexDir, "--lib", cfg.AndroidJar,
"--min-api", "21", "--pg-conf", proguardFile}
r8Args = append(r8Args, classFiles...)
cmd = exec.Command("java", r8Args...)
} else {
ui.Info("generando DEX con d8...")
d8Args := []string{"--output", dexDir, "--lib", cfg.AndroidJar, "--min-api", "21"}
d8Args = append(d8Args, classFiles...)
cmd = exec.Command("d8", d8Args...)
}
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
if err := cmd.Run(); err != nil {
return fmt.Errorf("dex: %w", err)
}
os.WriteFile(javaHashPath, []byte(currentJavaHash), 0644)
}

// --- Empaquetado ---
ui.Info("empaquetando...")
cmd := exec.Command("zip", "-j", baseApk, filepath.Join(cfg.BuildDir, "dex", "classes.dex"))
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
if err := cmd.Run(); err != nil {
return fmt.Errorf("zip: %w", err)
}

ui.Info("alineando...")
alignedApk := filepath.Join(cfg.BuildDir, "aligned.apk")
cmd = exec.Command("zipalign", "-f", "4", baseApk, alignedApk)
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
if err := cmd.Run(); err != nil {
return fmt.Errorf("zipalign: %w", err)
}

ui.Info("firmando...")
var keystore, ksPass, keyAlias, keyPass string

if cfg.Release && cfg.Signing != nil {
ui.Info("usando keystore de release: %s", cfg.Signing.StoreFile)
keystore = cfg.Signing.StoreFile
ksPass = "pass:" + cfg.Signing.StorePassword
keyAlias = cfg.Signing.KeyAlias
keyPass = "pass:" + cfg.Signing.KeyPassword
} else {
keystore = filepath.Join(cfg.BuildDir, "debug.keystore")
if !fileExists(keystore) {
cmd = exec.Command("keytool", "-genkey", "-v", "-keystore", keystore,
"-alias", "androiddebugkey", "-storepass", "android", "-keypass", "android",
"-keyalg", "RSA", "-keysize", "2048", "-validity", "10000",
"-dname", "CN=Android Debug,O=Android,C=US")
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
cmd.Run()
}
ksPass = "pass:android"
keyAlias = "androiddebugkey"
keyPass = "pass:android"
}

apkName := "app-debug.apk"
if cfg.Release {
apkName = "app-release.apk"
}
finalApk := filepath.Join(cfg.BuildDir, apkName)

apksignerArgs := []string{"sign", "--ks", keystore, "--ks-pass", ksPass}
if keyAlias != "" {
apksignerArgs = append(apksignerArgs, "--ks-key-alias", keyAlias)
}
if keyPass != "" {
apksignerArgs = append(apksignerArgs, "--key-pass", keyPass)
}
apksignerArgs = append(apksignerArgs, "--out", finalApk, alignedApk)
cmd = exec.Command("apksigner", apksignerArgs...)
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
if err := cmd.Run(); err != nil {
return fmt.Errorf("apksigner: %w", err)
}

elapsed := time.Since(start)
ui.Success("APK generado: %s (tardó %d segundos)", finalApk, int(elapsed.Seconds()))
return nil
}

func fileExists(path string) bool {
_, err := os.Stat(path)
return err == nil
}

func findD8Jar() string {
paths := []string{
"/data/data/com.termux/files/usr/share/java/d8.jar",
os.Getenv("PREFIX") + "/share/java/d8.jar",
}
for _, p := range paths {
if fileExists(p) {
return p
}
}
return ""
}
