package build

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/itskreisler/rain/internal/aar"
	"github.com/itskreisler/rain/internal/config"
	"github.com/itskreisler/rain/internal/hilt"
	"github.com/itskreisler/rain/internal/memenv"
	"github.com/itskreisler/rain/internal/ui"
)

type Config struct {
	AppDir      string
	BuildDir    string
	LibsDir     string
	AndroidJar  string
	Package     string
	Release     bool
	Signing     *config.Signing
	MinSdk      int
	TargetSdk   int
	VersionCode int
	VersionName string
	AppName     string
	// DepsDir contiene AAR/JAR preparados (classes.jar, res/, jni/) además de
	// los .jar sueltos de LibsDir. Viene de androidx/compose en modo completo.
	DepsDir string
	// Libs es el set de AAR/JAR preparados (si DepsDir está definido).
	Libs *aar.Set
	// ComposePlugin es la ruta a compose-compiler-plugin.jar; si está vacía se
	// autodetecta al compilar Kotlin.
	ComposePlugin string
	// JVMTarget es el target de bytecode (8 por defecto; 11 con Compose).
	JVMTarget int
	// ProguardFiles son archivos de reglas ProGuard/R8 adicionales (los del
	// usuario y los consumer-rules de los AAR). Se pasan a R8 en release.
	ProguardFiles []string
	// AllowDebugSigning permite firmar un build release con la keystore de
	// debug. Por defecto NO: un release sin keystore propia es un error.
	AllowDebugSigning bool
	// CheckOnly detiene el pipeline tras compilar los fuentes (Java + Kotlin):
	// no genera DEX, no empaqueta y no firma. Es el modo de "rain check",
	// que valida los tipos del proyecto sin pagar el empaquetado.
	CheckOnly bool
	// FastCheck hace el chequeo de tipos de forma incremental: solo
	// recompila los ficheros modificados (y los que dependen de ellos)
	// contra las clases ya compiladas. Es mucho más rápido, pero menos
	// exhaustivo: un error en un fichero que no se recompila puede no
	// aparecer.
	FastCheck bool
	// Kapt son los annotation processors que se ejecutan antes de compilar
	// (Dagger, Hilt...). Si está vacío, el proyecto no usa kapt.
	Kapt *KaptConfig
}

var manifestPackageRe = regexp.MustCompile(`package="([^"]+)"`)

// SourceRoot devuelve el directorio de fuentes de un módulo: el layout Gradle
// (app/src/main) o el layout plano de android2apk (java/ y res/ en la raíz).
// Se exporta para que otros comandos (check, lint) usen la misma resolución.
func SourceRoot(appDir string) string { return sourceRoot(appDir) }

// sourceRoot soporta el layout Gradle (app/src/main) y el layout plano que
// usan los ejemplos de android2apk (AndroidManifest.xml, java/ y res/ en la raíz).
func sourceRoot(appDir string) string {
	gradleRoot := filepath.Join(appDir, "src", "main")
	if fileExists(filepath.Join(gradleRoot, "AndroidManifest.xml")) {
		return gradleRoot
	}
	return appDir
}

func manifestPath(appDir string) string {
	return filepath.Join(sourceRoot(appDir), "AndroidManifest.xml")
}

func resPath(appDir string) string {
	return filepath.Join(sourceRoot(appDir), "res")
}

func manifestPackage(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	match := manifestPackageRe.FindSubmatch(data)
	if len(match) == 2 {
		return string(match[1])
	}
	return ""
}

// PackageFromManifest devuelve el package declarado en AndroidManifest.xml.
func PackageFromManifest(appDir string) string {
	return manifestPackage(manifestPath(appDir))
}

// reManifestPlaceholder busca los marcadores sin resolver del manifest.
var reManifestPlaceholder = regexp.MustCompile(`\$\{([^}]+)\}`)

// reManifestTag busca la etiqueta de apertura del <manifest>.
var reManifestTag = regexp.MustCompile(`<manifest(\s[^>]*)?>`)

// substituteManifestPlaceholders sustituye los marcadores del manifest que
// Gradle resuelve al fusionarlo ("${applicationId}", "${versionName}"...).
//
// Gradle lo hace porque los manifests de un proyecto pueden usar el id de la
// app sin conocerlo de antemano. aapt2 no entiende los marcadores: si se
// quedan sin sustituir, el enlazado falla.
//
// También añade el atributo "package" al <manifest> si falta: desde AGP 7 el
// namespace va en el build.gradle.kts y el manifest ya no lo lleva, pero aapt2
// lo exige para compilar los recursos.
//
// Devuelve la ruta del manifest procesado, que es la original si no había
// ningún marcador ni faltaba el package.
func substituteManifestPlaceholders(src, dst string, cfg Config) (string, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return src, err
	}
	content := string(data)
	needsPackage := false
	if m := reManifestTag.FindStringSubmatch(content); m != nil {
		needsPackage = !strings.Contains(m[1], "package=")
	}
	if !strings.Contains(content, "${") && !needsPackage {
		return src, nil
	}

	replacer := strings.NewReplacer(
		"${applicationId}", cfg.Package,
		"${applicationIdSuffix}", "",
		"${namespace}", cfg.Package,
		"${versionName}", cfg.VersionName,
		"${versionCode}", strconv.Itoa(cfg.VersionCode),
		"${minSdkVersion}", strconv.Itoa(cfg.MinSdk),
		"${targetSdkVersion}", strconv.Itoa(cfg.TargetSdk),
	)
	content = replacer.Replace(content)

	// Los marcadores que no se conocen se avisan: aapt2 fallaría con un
	// error poco descriptivo.
	if left := reManifestPlaceholder.FindAllStringSubmatch(content, -1); len(left) > 0 {
		names := make([]string, 0, len(left))
		for _, m := range left {
			names = append(names, m[1])
		}
		ui.Warn("marcadores sin sustituir en el manifest: %s", strings.Join(names, ", "))
	}

	if needsPackage {
		content = reManifestTag.ReplaceAllString(content, `<manifest package="`+cfg.Package+`"$1>`)
	}

	if err := os.WriteFile(dst, []byte(content), 0o644); err != nil {
		return src, err
	}
	return dst, nil
}

func hashFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

// computeResourceHash devuelve un hash con el estado de los recursos.
//
// Se incluyen los recursos y los manifests de las librerías (extraPaths): si
// una dependencia cambia de versión, sus recursos cambian, y el APK base debe
// rehacerse aunque los recursos de la app no se hayan tocado.
func computeResourceHash(appDir string, extraPaths ...string) (string, error) {
	var files []string
	resDir := resPath(appDir)
	manifest := manifestPath(appDir)

	if err := filepath.Walk(resDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info != nil && !info.IsDir() {
			files = append(files, path)
		}
		return nil
	}); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if _, err := os.Stat(manifest); err == nil {
		files = append(files, manifest)
	}
	for _, extra := range extraPaths {
		if extra == "" {
			continue
		}
		info, err := os.Stat(extra)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			files = append(files, extra)
			continue
		}
		if err := filepath.Walk(extra, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info != nil && !info.IsDir() {
				files = append(files, path)
			}
			return nil
		}); err != nil && !os.IsNotExist(err) {
			return "", err
		}
	}
	sort.Strings(files)

	global := sha256.New()
	for _, f := range files {
		h, err := hashFile(f)
		if err != nil {
			return "", err
		}
		global.Write(h)
		global.Write([]byte(filepath.Base(f)))
	}
	return fmt.Sprintf("%x", global.Sum(nil)), nil
}

// collectJavaSources devuelve todas las rutas .java bajo root (recursivo).
// Un root inexistente devuelve (nil, nil): no tener fuentes es válido.
func collectJavaSources(root string) ([]string, error) {
	var sources []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".java") {
			sources = append(sources, path)
		}
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return sources, nil
}

// Nombre de las marcas de compilación dentro del directorio de build.
//
// Son dos ficheros distintos a propósito: ver la nota de la fase de Java/DEX
// en el pipeline.
const (
	// javaHashFile marca que las fuentes actuales compilan sin errores.
	javaHashFile = ".rain-java-hash"
	// dexHashFile marca que el classes.dex corresponde a esas fuentes.
	dexHashFile = ".rain-dex-hash"
)

// dexIsUpToDate indica si el classes.dex que hay en el directorio de build
// se puede reutilizar.
//
// Solo se reutiliza si su marca coincide con el hash de las fuentes actuales
// y el fichero existe: un "rain check" no genera DEX, así que la marca de
// compilación no sirve aquí (si se usara, el APK llevaría código viejo).
func dexIsUpToDate(buildDir, currentHash string) bool {
	prev, err := os.ReadFile(filepath.Join(buildDir, dexHashFile))
	if err != nil {
		return false
	}
	if string(prev) != currentHash {
		return false
	}
	return fileExists(filepath.Join(buildDir, "dex", "classes.dex"))
}

func computeJavaHash(appDir, libsDir, genDir string, release bool) (string, error) {
	var files []string
	if err := filepath.Walk(sourceRoot(appDir), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, ".java") || strings.HasSuffix(path, ".kt") {
			files = append(files, path)
		}
		return nil
	}); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err := filepath.Walk(genDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, ".java") {
			files = append(files, path)
		}
		return nil
	}); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	entries, _ := os.ReadDir(libsDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".jar") {
			files = append(files, filepath.Join(libsDir, e.Name()))
		}
	}
	sort.Strings(files)

	global := sha256.New()
	if release {
		global.Write([]byte("release"))
	} else {
		global.Write([]byte("debug"))
	}
	for _, f := range files {
		h, err := hashFile(f)
		if err != nil {
			return "", err
		}
		global.Write(h)
		global.Write([]byte(filepath.Base(f)))
	}
	return fmt.Sprintf("%x", global.Sum(nil)), nil
}

func Run(cfg Config) error {
	start := time.Now()
	if err := ValidateJVMTarget(cfg.JVMTarget); err != nil {
		return err
	}
	if cfg.MinSdk <= 0 {
		cfg.MinSdk = 21
	}
	if cfg.TargetSdk <= 0 {
		cfg.TargetSdk = 36
	}
	if cfg.VersionCode <= 0 {
		cfg.VersionCode = 1
	}
	if cfg.VersionName == "" {
		cfg.VersionName = "1.0"
	}
	if cfg.Package == "" {
		cfg.Package = manifestPackage(manifestPath(cfg.AppDir))
	}
	if cfg.Package == "" {
		return fmt.Errorf("no se pudo determinar el package desde AndroidManifest.xml")
	}

	if err := os.MkdirAll(cfg.BuildDir, 0755); err != nil {
		return fmt.Errorf("creando directorio de build: %w", err)
	}

	genDir := filepath.Join(cfg.BuildDir, "gen")
	if err := os.MkdirAll(genDir, 0755); err != nil {
		return fmt.Errorf("creando directorio gen: %w", err)
	}

	// --- Recursos ---
	resHashPath := filepath.Join(cfg.BuildDir, ".rain-res-hash")
	flatDir := filepath.Join(cfg.BuildDir, "res")
	baseApk := filepath.Join(cfg.BuildDir, "base.apk")

	currentResHash, err := computeResourceHash(cfg.AppDir, cfg.libraryResourceInputs()...)
	if err != nil {
		return fmt.Errorf("hash de recursos: %w", err)
	}
	prevResHash, _ := os.ReadFile(resHashPath)
	resUpToDate := string(prevResHash) == currentResHash && fileExists(baseApk)

	if resUpToDate {
		ui.Info("recursos sin cambios, reutilizando APK base")
	} else {
		ui.Info("compilando recursos...")
		resDir := resPath(cfg.AppDir)
		manifest := manifestPath(cfg.AppDir)
		if !fileExists(manifest) {
			return fmt.Errorf("no se encontró AndroidManifest.xml en %s", sourceRoot(cfg.AppDir))
		}

		var flatFiles []string
		if dirExists(resDir) {
			if err := os.RemoveAll(flatDir); err != nil {
				return fmt.Errorf("limpiando recursos planos: %w", err)
			}
			if err := os.MkdirAll(flatDir, 0755); err != nil {
				return fmt.Errorf("creando directorio de recursos planos: %w", err)
			}

			cmd := exec.Command("aapt2", "compile", "--dir", resDir, "-o", flatDir)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("aapt2 compile: %w", err)
			}

			entries, err := os.ReadDir(flatDir)
			if err != nil {
				return fmt.Errorf("leyendo recursos planos: %w", err)
			}
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".flat") {
					flatFiles = append(flatFiles, filepath.Join(flatDir, e.Name()))
				}
			}
		} else {
			ui.VerboseLog("sin directorio res/, se enlaza solo el manifest")
		}

		ui.Info("enlazando recursos...")

		// Se fusionan los manifests de las librerías con el de la app. Sin
		// esto, los componentes que declaran (los providers de AndroidX
		// Startup, las activities de AppCompat...) no quedan registrados y la
		// app revienta en tiempo de ejecución con el ClassNotFoundException del
		// provider que faltaba.
		linkManifest := manifest

		// Un manifest de proyecto Gradle puede llevar marcadores ("${applicationId}")
		// que Gradle sustituye al fusionar. Rain hace lo mismo.
		if processed, err := substituteManifestPlaceholders(manifest, filepath.Join(cfg.BuildDir, "manifest.xml"), cfg); err != nil {
			ui.Warn("no se pudieron sustituir los marcadores del manifest: %v", err)
		} else if processed != manifest {
			linkManifest = processed
		}

		if cfg.Libs != nil {
			if libManifests := cfg.Libs.Manifests(); len(libManifests) > 0 {
				mergedPath := filepath.Join(cfg.BuildDir, "merged-manifest.xml")
				if err := aar.MergeManifests(linkManifest, libManifests, mergedPath); err != nil {
					ui.Warn("no se pudieron fusionar los manifests de las librerías: %v", err)
				} else {
					linkManifest = mergedPath
					ui.VerboseLog("manifest fusionado con %d librerías", len(libManifests))
				}
			}
		}

		// AGP inyecta permisos según el targetSdk (hoy POST_NOTIFICATIONS a
		// partir de API 33). Rain los añade igual para que el APK se comporte
		// como el de Gradle: sin ellos el sistema descarta las notificaciones
		// en silencio y la app parece no hacer nada. El resultado se escribe en
		// un fichero nuevo: el manifest del proyecto no se toca nunca.
		if manifestText, err := os.ReadFile(linkManifest); err == nil {
			withPerms, err := aar.WithImplicitPermissions(string(manifestText), cfg.TargetSdk)
			if err != nil {
				ui.Warn("no se pudieron añadir los permisos implícitos: %v", err)
			} else if withPerms != string(manifestText) {
				permsPath := filepath.Join(cfg.BuildDir, "manifest-with-permissions.xml")
				if err := os.WriteFile(permsPath, []byte(withPerms), 0644); err != nil {
					ui.Warn("no se pudo escribir el manifest con permisos implícitos: %v", err)
				} else {
					linkManifest = permsPath
					ui.VerboseLog("permisos implícitos añadidos al manifest")
				}
			}
		}

		args := []string{
			"link", "-o", baseApk, "-I", cfg.AndroidJar,
			"--manifest", linkManifest, "--java", genDir,
			"--min-sdk-version", strconv.Itoa(cfg.MinSdk),
			"--target-sdk-version", strconv.Itoa(cfg.TargetSdk),
			"--version-code", strconv.Itoa(cfg.VersionCode),
			"--version-name", cfg.VersionName,
		}
		if assets := filepath.Join(sourceRoot(cfg.AppDir), "assets"); dirExists(assets) {
			args = append(args, "-A", assets)
		}

		libRes, extraPackages, err := cfg.prepareLibraryResources(cfg.Libs)
		if err != nil {
			return fmt.Errorf("fusionando recursos de librerías: %w", err)
		}
		if libRes != "" {
			ui.Info("compilando recursos de librerías...")
			libFlat := filepath.Join(cfg.BuildDir, "libres-flat")
			if err := os.RemoveAll(libFlat); err != nil {
				return err
			}
			if err := os.MkdirAll(libFlat, 0o755); err != nil {
				return err
			}
			cmd := exec.Command("aapt2", "compile", "--dir", libRes, "-o", libFlat)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("aapt2 compile (librerías): %w", err)
			}
			entries, err := os.ReadDir(libFlat)
			if err != nil {
				return err
			}
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".flat") {
					flatFiles = append(flatFiles, filepath.Join(libFlat, e.Name()))
				}
			}
		}
		for _, pkg := range extraPackages {
			args = append(args, "--extra-packages", pkg)
		}

		args = append(args, flatFiles...)

		cmd := exec.Command("aapt2", args...)
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
	if err := os.MkdirAll(buildConfigDir, 0755); err != nil {
		return fmt.Errorf("creando directorio BuildConfig: %w", err)
	}

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
    public static final int VERSION_CODE = %d;
    public static final String VERSION_NAME = "%s";
}
`, cfg.Package, debugValue, cfg.Package, buildType, cfg.VersionCode, cfg.VersionName)

	buildConfigPath := filepath.Join(buildConfigDir, "BuildConfig.java")
	if err := os.WriteFile(buildConfigPath, []byte(buildConfigContent), 0644); err != nil {
		return fmt.Errorf("generando BuildConfig: %w", err)
	}

	// --- Java / DEX ---
	//
	// Hay dos marcas distintas y no se pueden mezclar:
	//
	//   - `.rain-java-hash`: las fuentes actuales compilan sin errores. La
	//     escribe cualquier compilación, también la de `rain check`.
	//   - `.rain-dex-hash`: el `classes.dex` de `build/dex` corresponde a esas
	//     mismas fuentes. Solo la escribe un build que llegó a generar el DEX.
	//
	// Si una sola marca sirviera para las dos cosas, un `rain check` seguido
	// de `rain build` reutilizaría el DEX de la compilación anterior y el APK
	// llevaría código que ya no está en el disco.
	javaHashPath := filepath.Join(cfg.BuildDir, javaHashFile)
	dexHashPath := filepath.Join(cfg.BuildDir, dexHashFile)

	currentJavaHash, err := computeJavaHash(cfg.AppDir, cfg.LibsDir, genDir, cfg.Release)
	if err != nil {
		return fmt.Errorf("hash de java: %w", err)
	}
	prevJavaHash, _ := os.ReadFile(javaHashPath)
	sourcesUpToDate := string(prevJavaHash) == currentJavaHash

	// En modo "check" solo importa el código: si las fuentes no han cambiado
	// desde la última compilación correcta no hay nada que comprobar.
	if cfg.CheckOnly && sourcesUpToDate {
		ui.Info("código sin cambios desde la última compilación correcta")
		return nil
	}

	// Chequeo incremental: solo se recompilan los ficheros modificados (y los
	// que dependen de ellos) contra las clases ya compiladas. Es mucho más
	// rápido, pero puede no pillar errores en ficheros que no se han
	// recompilado.
	if cfg.CheckOnly && cfg.FastCheck {
		return fastCheck(cfg, genDir)
	}

	javaUpToDate := dexIsUpToDate(cfg.BuildDir, currentJavaHash)

	// Proveedores de java.util.ServiceLoader declarados por las dependencias.
	// Los ficheros META-INF/services van dentro del APK (el ServiceLoader los
	// busca ahí en tiempo de ejecución) y hay que mantener vivos a los
	// proveedores: R8 no reescribe esos ficheros y los eliminaría.
	serviceProviders, err := cfg.serviceProviders()
	if err != nil {
		return fmt.Errorf("leyendo los servicios de las dependencias: %w", err)
	}

	if javaUpToDate {
		ui.Info("java sin cambios, reutilizando classes.dex")
	} else {
		objDir := filepath.Join(cfg.BuildDir, "obj")
		if err := os.MkdirAll(objDir, 0755); err != nil {
			return fmt.Errorf("creando directorio de objetos: %w", err)
		}

		kotlinSources, err := compileSources(cfg, genDir, objDir)
		if err != nil {
			return err
		}

		// Marca de compilación correcta: a partir de aquí "rain check --fast"
		// sabe qué ficheros son nuevos y cuáles se han quedaado intactos.
		if err := writeSourcesStamp(cfg.BuildDir); err != nil {
			ui.VerboseLog("no se pudo guardar la marca de compilación: %v", err)
		}

		// Las fuentes ya son válidas: a partir de aquí un "rain check"
		// posterior puede saltarse el trabajo. Ojo: esta marca NO dice que el
		// DEX esté al día (eso lo lleva `.rain-dex-hash`), porque un "check"
		// no genera DEX.
		if err := os.WriteFile(javaHashPath, []byte(currentJavaHash), 0644); err != nil {
			return fmt.Errorf("guardando hash de java: %w", err)
		}

		if cfg.CheckOnly {
			ui.Info("chequeo de tipos completado: sin errores")
			return nil
		}

		var classFiles []string
		if err := filepath.Walk(objDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if strings.HasSuffix(path, ".class") && !info.IsDir() {
				classFiles = append(classFiles, path)
			}
			return nil
		}); err != nil {
			return fmt.Errorf("buscando clases compiladas: %w", err)
		}

		// Las librerías de AndroidX/Compose deben entrar en el DEX: d8/R8 no
		// resuelven dependencias transitivas por sí solos. Se aplican incluso
		// en proyectos Java puros con dependencias.
		deps := dedupeJarsByClass(cfg.libraryJars(), filepath.Join(cfg.BuildDir, "dedupe"))
		if len(deps) > 0 {
			ui.VerboseLog("dex: %d jars de dependencias", len(deps))
		}

		if len(kotlinSources) > 0 {
			stdlib := findKotlinStdlib()
			if stdlib == "" {
				return fmt.Errorf("se compiló Kotlin pero no se encontró kotlin-stdlib.jar (revisa la instalación de kotlinc)")
			}
			// El stdlib debe ser el primero: si alguna dependencia lo duplica,
			// dedupeJarsByClass conserva esta versión (la del toolchain).
			classFiles = append(classFiles, stdlib)
		}
		classFiles = append(classFiles, deps...)

		dexDir := filepath.Join(cfg.BuildDir, "dex")
		if err := os.MkdirAll(dexDir, 0755); err != nil {
			return fmt.Errorf("creando directorio dex: %w", err)
		}

		var cmd *exec.Cmd
		if cfg.Release {
			ui.Info("generando DEX con R8 (release)...")
			d8Jar := findD8Jar()
			if d8Jar == "" {
				return fmt.Errorf("no se encontró d8.jar para R8")
			}

			// Reglas base de Rain en un archivo generado (nunca pisa las del
			// usuario) + las reglas del usuario + los consumer-rules de los AAR.
			baseRules := filepath.Join(cfg.BuildDir, "r8-rain.pro")
			if err := os.WriteFile(baseRules, []byte(defaultProguardRules), 0644); err != nil {
				return fmt.Errorf("escribiendo reglas R8 base: %w", err)
			}

			r8Args := []string{"-cp", d8Jar, "com.android.tools.r8.R8",
				"--release", "--output", dexDir,
				"--min-api", strconv.Itoa(cfg.MinSdk), "--pg-conf", baseRules}

			for _, lib := range cfg.r8LibraryJars(classFiles) {
				r8Args = append(r8Args, "--lib", lib)
			}

			for _, pg := range cfg.proguardFiles() {
				ui.VerboseLog("reglas R8: %s", pg)
				r8Args = append(r8Args, "--pg-conf", pg)
			}

			// R8 no sabe que las clases de los proveedores de servicio están
			// vivas por el fichero META-INF/services: si no se le dice, las
			// elimina y la app falla al arrancar ("Module with the Main
			// dispatcher is missing").
			if len(serviceProviders) > 0 {
				svcRules := filepath.Join(cfg.BuildDir, "r8-services.pro")
				if err := os.WriteFile(svcRules, serviceKeepRules(serviceProviders), 0644); err != nil {
					return fmt.Errorf("escribiendo reglas R8 de servicios: %w", err)
				}
				ui.VerboseLog("reglas R8 de servicios: %s (%d servicios)", svcRules, len(serviceProviders))
				r8Args = append(r8Args, "--pg-conf", svcRules)
			}

			// mapping.txt para desofuscar crashes (nombre estable tras el build).
			mappingPath := filepath.Join(cfg.BuildDir, "mapping.txt")
			r8Args = append(r8Args, "--pg-map-output", mappingPath)

			r8Args = append(r8Args, classFiles...)
			cmd = exec.Command("java", r8Args...)
		} else {
			ui.Info("generando DEX con d8...")
			d8Args := []string{"--output", dexDir, "--lib", cfg.AndroidJar, "--min-api", strconv.Itoa(cfg.MinSdk)}
			d8Args = append(d8Args, classFiles...)
			cmd = exec.Command("d8", d8Args...)
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("dex: %w", err)
		}
		// Solo ahora el DEX corresponde a estas fuentes: se guarda su marca
		// para que el siguiente build pueda reutilizarlo.
		if err := os.WriteFile(dexHashPath, []byte(currentJavaHash), 0644); err != nil {
			return fmt.Errorf("guardando hash del DEX: %w", err)
		}
	}
	// --- Empaquetado ---
	ui.Info("empaquetando...")
	dexFiles, err := filepath.Glob(filepath.Join(cfg.BuildDir, "dex", "classes*.dex"))
	if err != nil {
		return fmt.Errorf("buscando DEX: %w", err)
	}
	if len(dexFiles) == 0 {
		return fmt.Errorf("no se generó ningún classes*.dex")
	}
	sort.Slice(dexFiles, func(i, j int) bool {
		return dexNumber(dexFiles[i]) < dexNumber(dexFiles[j])
	})

	nativeLibs, err := collectNativeLibs(cfg.nativeDir(cfg.Libs))
	if err != nil {
		return err
	}
	if len(nativeLibs) > 0 {
		ui.Info("empaquetando %d bibliotecas nativas...", len(nativeLibs))
	}

	// Los recursos Java de las dependencias (META-INF/services) van dentro del
	// APK: el ServiceLoader los busca en el classpath y, sin ellos, la app no
	// encuentra al proveedor y revienta al arrancar.
	javaResources, err := writeServiceFiles(filepath.Join(cfg.BuildDir, "javares"), serviceProviders)
	if err != nil {
		return err
	}
	if len(javaResources) > 0 {
		ui.Info("empaquetando %d ficheros de servicio...", len(javaResources))
	}

	if err := appendDex(baseApk, dexFiles, nativeLibs, javaResources); err != nil {
		return err
	}

	ui.Info("alineando...")
	alignedApk := filepath.Join(cfg.BuildDir, "aligned.apk")
	cmd := exec.Command("zipalign", "-f", "4", baseApk, alignedApk)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("zipalign: %w", err)
	}

	ui.Info("firmando...")
	var keystore, ksPass, keyAlias, keyPass string

	if cfg.Release && cfg.Signing != nil {
		// Una keystore mal configurada debe fallar aquí con un mensaje claro,
		// no más tarde con un error críptico de apksigner.
		if cfg.Signing.StoreFile == "" {
			return fmt.Errorf("signing.store_file vacío en rain.toml")
		}
		if !fileExists(cfg.Signing.StoreFile) {
			return fmt.Errorf("signing.store_file no existe: %s", cfg.Signing.StoreFile)
		}
		if cfg.Signing.KeyAlias == "" {
			return fmt.Errorf("signing.key_alias vacío en rain.toml")
		}
		ui.Info("usando keystore de release: %s", cfg.Signing.StoreFile)
		keystore = cfg.Signing.StoreFile
		ksPass = "pass:" + cfg.Signing.StorePassword
		keyAlias = cfg.Signing.KeyAlias
		keyPass = "pass:" + cfg.Signing.KeyPassword
		// Muchos keystores son PKCS12 y no admiten una clave distinta a la del
		// almacén: si no se define key_password, se reutiliza store_password.
		if cfg.Signing.KeyPassword == "" {
			keyPass = ksPass
		}
	} else {
		// Ni debug ni release deberían firmarse en silencio con la keystore de
		// debug: un release sin keystore propia se avisa explícitamente.
		if cfg.Release && !cfg.AllowDebugSigning {
			ui.Warn("release sin keystore configurada: se firma con la de DEBUG (--allow-debug-signing para confirmarlo)")
		} else if cfg.Release {
			ui.Warn("release firmado con la keystore de DEBUG (--allow-debug-signing)")
		}
		keystore = debugKeystorePath(cfg.BuildDir)
		if !fileExists(keystore) {
			cmd = exec.Command("keytool", "-genkey", "-v", "-keystore", keystore,
				"-alias", "androiddebugkey", "-storepass", "android", "-keypass", "android",
				"-keyalg", "RSA", "-keysize", "2048", "-validity", "10000",
				"-dname", "CN=Android Debug,O=Android,C=US")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("generando keystore de debug: %w", err)
			}
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

	// Verifica la firma para no entregar un APK silenciosamente inválido.
	verify := exec.Command("apksigner", "verify", "--print-certs", finalApk)
	var verifyOut bytes.Buffer
	verify.Stdout = &verifyOut
	verify.Stderr = &verifyOut
	if err := verify.Run(); err != nil {
		fmt.Println(verifyOut.String())
		return fmt.Errorf("el APK firmado no pasa apksigner verify")
	}
	if cfg.Release {
		ui.Info("firma verificada (release); mapping en %s", filepath.Join(cfg.BuildDir, "mapping.txt"))
	}

	// Copia descriptiva estilo android2apk: <nombre>-<version>-<abi>.apk
	namedApk := filepath.Join(cfg.BuildDir, apkNameFor(cfg))
	if err := copyFile(finalApk, namedApk); err != nil {
		return fmt.Errorf("creando %s: %w", namedApk, err)
	}

	elapsed := time.Since(start)
	ui.Success("APK generado: %s (%s, tardó %d segundos)", namedApk, humanSize(namedApk), int(elapsed.Seconds()))
	ui.Info("ruta estable: %s", finalApk)
	return nil
}

// humanSize devuelve el tamaño de un archivo en un formato legible (B, KiB,
// MiB). Si no se puede leer, devuelve "?".
// compileSources compila todas las fuentes del proyecto (Java generado,
// Kotlin y Java de la app) dentro de objDir.
func compileSources(cfg Config, genDir, objDir string) ([]string, error) {
	// La compilación completa empieza de cero: si se ha borrado o renombrado
	// un fichero, sus clases viejas no deben quedarse en el DEX.
	if err := os.RemoveAll(objDir); err != nil {
		return nil, fmt.Errorf("limpiando el directorio de objetos: %w", err)
	}
	if err := os.MkdirAll(objDir, 0o755); err != nil {
		return nil, fmt.Errorf("creando el directorio de objetos: %w", err)
	}

	kotlinFiles, err := collectKotlinFiles(sourceRoot(cfg.AppDir))
	if err != nil {
		return nil, err
	}
	javaFiles, err := collectJavaSources(sourceRoot(cfg.AppDir))
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("buscando fuentes Java: %w", err)
	}
	return compileSourceSet(cfg, genDir, objDir, kotlinFiles, javaFiles)
}

// collectKotlinFiles devuelve las rutas .kt bajo root (recursivo).
func collectKotlinFiles(root string) ([]string, error) {
	var sources []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info != nil && !info.IsDir() && strings.HasSuffix(path, ".kt") {
			sources = append(sources, path)
		}
		return nil
	})
	if err != nil && os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("buscando fuentes Kotlin: %w", err)
	}
	return sources, nil
}

// compileSourceSet compila el conjunto de fuentes indicado dentro de objDir.
// El Java generado (R.java, BuildConfig.java) se compila siempre: es rápido y
// sus clases hacen falta en el classpath del resto.
//
// Devuelve las fuentes Kotlin compiladas (el DEX necesita el stdlib si las hay)
// o el error del compilador, con el diagnóstico ya impreso en pantalla.
func compileSourceSet(cfg Config, genDir, objDir string, kotlinFiles, javaFiles []string) ([]string, error) {
	// --- Compilar el Java generado (R.java, BuildConfig.java) primero ---
	// Así las clases generadas quedan en objDir y son visibles tanto desde
	// el código Kotlin (que se compila después con objDir en el classpath)
	// como desde el Java de la app.
	generatedSources, err := collectJavaSources(genDir)
	if err != nil {
		return nil, fmt.Errorf("buscando fuentes generadas: %w", err)
	}
	if len(generatedSources) > 0 {
		var genClasspath []string
		genClasspath = append(genClasspath, cfg.AndroidJar)
		genClasspath = append(genClasspath, cfg.libraryJars()...)
		genArgs := []string{"-d", objDir, "--release", strconv.Itoa(cfg.jvmTarget()), "-encoding", "UTF-8", "-Xlint:-options",
			"-classpath", strings.Join(genClasspath, ":")}
		genArgs = append(genArgs, generatedSources...)

		var genErr bytes.Buffer
		genCmd := exec.Command("javac", genArgs...)
		genCmd.Stdout = os.Stdout
		genCmd.Stderr = &genErr
		if err := genCmd.Run(); err != nil {
			ui.Error("Errores de compilación Java (generado):")
			fmt.Println(genErr.String())
			return nil, fmt.Errorf("javac falló")
		}
	}

	// Con kapt hay una ronda de annotation processing antes de compilar: se
	// generan los stubs de las clases Kotlin y se pasan por los processors
	// (Dagger, Hilt...), que generan código Java. Ese Java se compila
	// después, con las clases Kotlin ya en el classpath.
	var kaptSources []string
	if len(kotlinFiles) > 0 && cfg.Kapt != nil && len(cfg.Kapt.ProcessorJars) > 0 {
		kaptDir := filepath.Join(cfg.BuildDir, "kapt")
		generated, err := runKapt(cfg, kotlinFiles, objDir, kaptDir)
		if err != nil {
			return nil, err
		}
		kaptSources, err = collectJavaSources(generated)
		if err != nil {
			return nil, fmt.Errorf("buscando el Java generado por kapt: %w", err)
		}
	}

	if len(kotlinFiles) > 0 {
		if err := compileKotlin(cfg, objDir, kotlinFiles); err != nil {
			return nil, err
		}
	}

	if len(kaptSources) > 0 {
		if err := compileJavaSources(cfg, objDir, kaptSources, "generado por kapt"); err != nil {
			return nil, err
		}
	}

	if len(javaFiles) > 0 {
		ui.Info("compilando Java...")
	}

	// Fuentes de la app. objDir ya contiene las clases generadas y las de
	// Kotlin, así que Java puede referenciarlas (y viceversa).
	if len(javaFiles) > 0 {
		if err := compileJavaSources(cfg, objDir, javaFiles, "Java"); err != nil {
			return nil, err
		}
	}

	// El transform de Hilt: las clases anotadas con @AndroidEntryPoint o
	// @HiltAndroidApp tienen que heredar de la clase base que genera el
	// processor (Hilt_MainActivity, Hilt_MyApp...). El plugin de Gradle lo
	// hace con ASM después de compilar; aquí se hace con el mismo efecto.
	if cfg.Kapt != nil && cfg.Kapt.isHiltProcessor() {
		count, err := hilt.TransformDir(objDir)
		if err != nil {
			return nil, fmt.Errorf("transformando las clases de Hilt: %w", err)
		}
		if count > 0 {
			ui.Info("Hilt: %d clases reescritas para heredar de su clase base", count)
		}
	}

	return kotlinFiles, nil
}

// compileKotlin compila las fuentes Kotlin del proyecto dentro de objDir.
func compileKotlin(cfg Config, objDir string, kotlinFiles []string) error {
	ui.Info("compilando Kotlin (%d archivos)...", len(kotlinFiles))

	jvmTarget := cfg.jvmTarget()

	var kotlinClasspath []string
	kotlinClasspath = append(kotlinClasspath, cfg.AndroidJar)
	kotlinClasspath = append(kotlinClasspath, cfg.libraryJars()...)
	// objDir contiene R.class/BuildConfig.class ya compilados, para que
	// el código Kotlin pueda referenciar R y BuildConfig.
	kotlinClasspath = append(kotlinClasspath, objDir)

	kotlincArgs := []string{"-J-Dlibrary.jansi.path=" + os.Getenv("PREFIX") + "/lib/jansi",
		"-d", objDir,
		"-classpath", strings.Join(kotlinClasspath, ":"),
		"-jvm-target", kotlinJVMTarget(jvmTarget)}

	// La JVM de kotlinc reserva por defecto 1/4 de la RAM física. En
	// un contenedor con poca memoria libre eso es un OOM asegurado,
	// así que se acota al presupuesto real (detectado o indicado por
	// RAIN_JAVA_MAX_HEAP). Si el usuario ya fijó un heap con las
	// variables estándar, no se toca nada.
	if !memenv.UserHeapOverride() {
		heap := memenv.HeapLimit(memenv.DefaultKotlincHeap)
		ui.VerboseLog("heap de kotlinc: %s", heap)
		kotlincArgs = append(kotlincArgs, "-J-Xmx"+heap)
	}

	if cfg.appliesComposePlugin() {
		if plugin := cfg.composePlugin(); plugin != "" {
			ui.VerboseLog("plugin Compose: %s", plugin)
			kotlincArgs = append(kotlincArgs, "-Xplugin="+plugin)
		}
	}

	kotlincArgs = append(kotlincArgs, kotlinFiles...)

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
	return nil
}

// compileJavaSources compila un conjunto de fuentes Java dentro de objDir.
// El label solo se usa en los mensajes de error.
func compileJavaSources(cfg Config, objDir string, sources []string, label string) error {
	if len(sources) == 0 {
		return nil
	}
	var classpath []string
	classpath = append(classpath, cfg.AndroidJar)
	classpath = append(classpath, objDir)
	classpath = append(classpath, cfg.libraryJars()...)
	args := []string{"-d", objDir, "--release", strconv.Itoa(cfg.jvmTarget()), "-encoding", "UTF-8", "-Xlint:-options",
		"-classpath", strings.Join(classpath, ":")}
	args = append(args, sources...)

	var stderrBuf bytes.Buffer
	cmd := exec.Command("javac", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = &stderrBuf
	if err := cmd.Run(); err != nil {
		ui.Error("Errores de compilación Java (%s):", label)
		fmt.Println(stderrBuf.String())
		return fmt.Errorf("javac falló")
	}
	return nil
}

func humanSize(path string) string {
	st, err := os.Stat(path)
	if err != nil {
		return "?"
	}
	return formatBytes(st.Size())
}

// formatBytes formatea un número de bytes con una unidad binaria y un decimal
// (p. ej. 12.3 MiB).
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit && exp < 4; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGT"[exp])
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	st, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, st.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

// appendDex añade todos los classes*.dex al APK base (multidex) preservando
// el resto de entradas (manifest, resources.arsc...), igual que hace
// android2apk. No usamos "zip" porque machacaría resources.arsc y su alineado.
// nativeLibs mapea la ruta dentro del APK (lib/<abi>/x.so) al fichero origen.
// javaResources mapea la ruta dentro del APK (META-INF/services/...) al
// fichero origen: son los recursos Java de las dependencias, que el
// ServiceLoader busca en el APK en tiempo de ejecución.
func appendDex(baseApk string, dexFiles []string, nativeLibs map[string]string, javaResources map[string]string) error {
	src, err := zip.OpenReader(baseApk)
	if err != nil {
		return fmt.Errorf("abriendo APK base: %w", err)
	}
	defer func() { _ = src.Close() }()

	tmp := baseApk + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("creando APK temporal: %w", err)
	}

	writer := zip.NewWriter(out)
	fail := func(err error) error {
		_ = writer.Close()
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}

	// El APK base puede venir de un build anterior (se reutiliza si los
	// recursos no han cambiado) y entonces ya contiene los DEX y las
	// librerías nativas que se van a añadir ahora. Si se copiaran tal cual,
	// el APK tendría entradas duplicadas ("Multiple ZIP entries with the
	// same name: classes.dex") y apksigner lo rechazaría.
	skip := map[string]bool{}
	for _, dex := range dexFiles {
		skip[filepath.Base(dex)] = true
	}
	for name := range nativeLibs {
		skip[name] = true
	}
	for name := range javaResources {
		skip[name] = true
	}

	for _, file := range src.File {
		if skip[file.Name] {
			continue
		}
		if err := copyZipEntry(writer, file); err != nil {
			return fail(fmt.Errorf("copiando %s: %w", file.Name, err))
		}
	}

	for _, dex := range dexFiles {
		if err := addFileEntry(writer, filepath.Base(dex), dex); err != nil {
			return fail(fmt.Errorf("añadiendo %s: %w", dex, err))
		}
	}

	// Bibliotecas nativas de los AAR: lib/<abi>/x.so (orden determinista).
	names := make([]string, 0, len(nativeLibs))
	for name := range nativeLibs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := addFileEntry(writer, name, nativeLibs[name]); err != nil {
			return fail(fmt.Errorf("añadiendo %s: %w", name, err))
		}
	}

	// Recursos Java de las dependencias: META-INF/services/... (orden
	// determinista).
	resNames := make([]string, 0, len(javaResources))
	for name := range javaResources {
		resNames = append(resNames, name)
	}
	sort.Strings(resNames)
	for _, name := range resNames {
		if err := addFileEntry(writer, name, javaResources[name]); err != nil {
			return fail(fmt.Errorf("añadiendo %s: %w", name, err))
		}
	}

	if err := writer.Close(); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("cerrando APK: %w", err)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("cerrando archivo APK: %w", err)
	}
	return os.Rename(tmp, baseApk)
}

// zipEpoch es la fecha fija que se escribe en las entradas del ZIP. Un APK
// reproducible necesita timestamps constantes: con time.Now() dos builds del
// mismo commit producen binarios distintos. 1980-01-01 es la fecha mínima que
// representa el formato ZIP y la que usa AGP para builds reproducibles.
var zipEpoch = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)

// zipTimestamp devuelve el timestamp fijo de las entradas del ZIP. Si se
// define SOURCE_DATE_EPOCH (convención estándar de builds reproducibles) se
// usa esa fecha; si no, la época del formato ZIP.
func zipTimestamp() time.Time {
	if raw := strings.TrimSpace(os.Getenv("SOURCE_DATE_EPOCH")); raw != "" {
		if secs, err := strconv.ParseInt(raw, 10, 64); err == nil && secs > 0 {
			moment := time.Unix(secs, 0).UTC()
			if moment.Year() >= 1980 {
				return moment
			}
		}
	}
	return zipEpoch
}

// debugKeystorePath devuelve la keystore de debug. Vive fuera del directorio
// de build (como la de AGP en ~/.android) porque, si se regenerara en cada
// build, la firma del APK cambiaría y el build dejaría de ser reproducible.
func debugKeystorePath(buildDir string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		path := filepath.Join(home, ".rain", "debug.keystore")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
			return path
		}
	}
	return filepath.Join(buildDir, "debug.keystore")
}

// addFileEntry escribe un fichero del disco dentro del ZIP con la ruta dada.
func addFileEntry(writer *zip.Writer, name, src string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	header := &zip.FileHeader{
		Name:     name,
		Method:   zip.Deflate,
		Modified: zipTimestamp(),
	}
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = entry.Write(data)
	return err
}

// collectNativeLibs recorre jni/<abi>/*.so y devuelve un mapa
// lib/<abi>/<file>.so -> ruta de origen.
func collectNativeLibs(nativeDir string) (map[string]string, error) {
	libs := make(map[string]string)
	if nativeDir == "" {
		return libs, nil
	}
	abis, _ := os.ReadDir(nativeDir)
	for _, abi := range abis {
		if !abi.IsDir() {
			continue
		}
		abiDir := filepath.Join(nativeDir, abi.Name())
		files, err := os.ReadDir(abiDir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".so") {
				continue
			}
			apkPath := filepath.ToSlash(filepath.Join("lib", abi.Name(), f.Name()))
			libs[apkPath] = filepath.Join(abiDir, f.Name())
		}
	}
	return libs, nil
}

func copyZipEntry(writer *zip.Writer, file *zip.File) error {
	header := file.FileHeader
	// aapt2 pone la hora de compilación en sus entradas: se sustituye por el
	// timestamp fijo para que el APK sea reproducible.
	header.Modified = zipTimestamp()
	entry, err := writer.CreateHeader(&header)
	if err != nil {
		return err
	}
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()
	_, err = io.Copy(entry, reader)
	return err
}

// apkNameFor construye el nombre descriptivo estilo android2apk:
// <nombre>-<versionName>-<abi>.apk, y deja también la ruta estable app-*.apk.
func apkNameFor(cfg Config) string {
	name := sanitizeTag(cfg.AppName)
	if name == "" {
		name = sanitizeTag(filepath.Base(filepath.Clean(cfg.AppDir)))
	}
	if name == "" {
		name = "app"
	}
	version := sanitizeTag(cfg.VersionName)
	if version == "" {
		version = "1.0"
	}
	apk := fmt.Sprintf("%s-%s-%s.apk", name, version, detectABI(cfg.AppDir))
	if cfg.Release {
		// El release lleva sufijo: si no, el nombre descriptivo del release
		// pisaría al del debug y solo quedaría una de las dos variantes.
		apk = strings.TrimSuffix(apk, ".apk") + "-release.apk"
	}
	return apk
}

func sanitizeTag(value string) string {
	var builder strings.Builder
	for _, r := range strings.TrimSpace(value) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '.', r == '_', r == '-':
			builder.WriteRune(r)
		default:
			builder.WriteByte('-')
		}
	}
	return strings.Trim(builder.String(), "-")
}

// detectABI mira lib/<abi>/ dentro del proyecto; sin código nativo es "universal".
func detectABI(appDir string) string {
	libDir := filepath.Join(appDir, "lib")
	entries, err := os.ReadDir(libDir)
	if err != nil {
		return "universal"
	}

	var abis []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		hasSo := false
		files, err := os.ReadDir(filepath.Join(libDir, entry.Name()))
		if err != nil {
			continue
		}
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".so") {
				hasSo = true
				break
			}
		}
		if hasSo {
			abis = append(abis, entry.Name())
		}
	}
	if len(abis) == 0 {
		return "universal"
	}
	sort.Strings(abis)
	if strings.Join(abis, "_") == "arm64-v8a_armeabi-v7a_x86_x86_64" {
		return "universal"
	}
	return strings.Join(abis, "_")
}

func dexNumber(path string) int {
	name := strings.TrimSuffix(filepath.Base(path), ".dex")
	name = strings.TrimPrefix(name, "classes")
	if name == "" {
		return 1
	}
	number, err := strconv.Atoi(name)
	if err != nil {
		return 1 << 30
	}
	return number
}

func findD8Jar() string {
	home, _ := os.UserHomeDir()
	paths := []string{
		filepath.Join(home, ".rain", "toolchain", "r8.jar"),
		filepath.Join(home, ".rain", "toolchain", "d8.jar"),
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

func findKotlinStdlib() string {
	home, _ := os.UserHomeDir()
	paths := []string{
		os.Getenv("KOTLIN_STDLIB"),
		filepath.Join(home, ".rain", "toolchain", "kotlinc", "lib", "kotlin-stdlib.jar"),
		filepath.Join(home, "kotlin", "kotlinc", "lib", "kotlin-stdlib.jar"),
		"/usr/share/kotlin/lib/kotlin-stdlib.jar",
		os.Getenv("PREFIX") + "/lib/kotlin-stdlib.jar",
	}
	if kotlinc, err := exec.LookPath("kotlinc"); err == nil {
		if real, err := filepath.EvalSymlinks(kotlinc); err == nil {
			paths = append(paths, filepath.Join(filepath.Dir(real), "..", "lib", "kotlin-stdlib.jar"))
		}
	}
	for _, p := range paths {
		if p != "" && fileExists(p) {
			return p
		}
	}
	return ""
}

// r8LibraryJars devuelve los jars que R8 debe tratar como librerías (--lib)
// además de android.jar. R8 valida las referencias del programa contra estas
// librerías y aborta con "Missing class" si no las encuentra (d8 no valida).
// Un proyecto Java con AAR de AndroidX que usan Kotlin necesita el stdlib del
// toolchain, aunque el propio proyecto no tenga fuentes Kotlin.
//
// programJars son los jars que ya entran como programa: si el stdlib o las
// anotaciones ya están ahí (proyecto con fuentes Kotlin), no se repiten como
// librería para evitar el aviso "defined by both the program and the library".
func (c Config) r8LibraryJars(programJars []string) []string {
	inProgram := make(map[string]bool, len(programJars))
	for _, j := range programJars {
		inProgram[j] = true
	}

	jars := []string{c.AndroidJar}
	if stdlib := findKotlinStdlib(); stdlib != "" && !inProgram[stdlib] {
		jars = append(jars, stdlib)
	}
	if annotations := findJetbrainsAnnotations(); annotations != "" && !inProgram[annotations] {
		jars = append(jars, annotations)
	}
	return jars
}

// findJetbrainsAnnotations localiza annotations.jar (org.jetbrains.annotations.*),
// que AndroidX y Kotlin referencian desde sus APIs públicas.
func findJetbrainsAnnotations() string {
	home, _ := os.UserHomeDir()
	paths := []string{
		os.Getenv("JB_ANNOTATIONS"),
		filepath.Join(home, ".rain", "toolchain", "kotlinc", "lib", "annotations-13.0.jar"),
		filepath.Join(home, "kotlin", "kotlinc", "lib", "annotations-13.0.jar"),
		"/usr/share/kotlin/lib/annotations-13.0.jar",
		os.Getenv("PREFIX") + "/lib/annotations-13.0.jar",
	}
	if kotlinc, err := exec.LookPath("kotlinc"); err == nil {
		if real, err := filepath.EvalSymlinks(kotlinc); err == nil {
			dir := filepath.Join(filepath.Dir(real), "..", "lib")
			if matches, _ := filepath.Glob(filepath.Join(dir, "annotations-*.jar")); len(matches) > 0 {
				paths = append(paths, matches...)
			}
		}
	}
	for _, p := range paths {
		if p != "" && fileExists(p) {
			return p
		}
	}
	return ""
}

// defaultJVMTarget es el nivel de bytecode por defecto. AGP moderno compila a
// Java 17 y AndroidX/Compose actuales ya se publican con ese nivel, así que
// bajar a 8 solo tiene sentido para código heredado.
const defaultJVMTarget = 17

// validJVMTargets son los niveles de bytecode que Rain sabe emitir.
var validJVMTargets = []int{8, 11, 17, 21}

// ValidateJVMTarget comprueba que el nivel de bytecode sea uno de los
// soportados. 0 significa "automático" y siempre es válido.
func ValidateJVMTarget(target int) error {
	if target == 0 {
		return nil
	}
	for _, valid := range validJVMTargets {
		if target == valid {
			return nil
		}
	}
	return fmt.Errorf("java_target %d no válido: usa uno de %s",
		target, strings.Trim(strings.Repeat("%d ", len(validJVMTargets)), " "))
}

// jvmTarget devuelve el target de bytecode: 17 por defecto (como AGP) y el
// valor de java_target si el proyecto lo fija.
func (c Config) jvmTarget() int {
	if c.JVMTarget > 0 {
		return c.JVMTarget
	}
	return defaultJVMTarget
}

// appliesComposePlugin indica si el proyecto realmente usa Compose y por tanto
// hay que aplicar el plugin del compilador y subir el target a JVM 11. Un
// proyecto Kotlin sin Compose (p. ej. khello) NO debe recibir el plugin: exige
// androidx.compose.runtime en el classpath y fallaría con
// "noRuntimeOnClasspathError".
func (c Config) appliesComposePlugin() bool {
	return c.composePlugin() != "" && c.usesCompose()
}

// usesCompose detecta si alguna fuente Kotlin importa algo de androidx.compose.
func (c Config) usesCompose() bool {
	for _, path := range c.findKotlinSources() {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), "androidx.compose") {
			return true
		}
	}
	return false
}

// composePlugin localiza compose-compiler-plugin.jar (explícito, ~/.rain o la
// instalación de kotlinc).
func (c Config) composePlugin() string {
	if c.ComposePlugin != "" {
		if fileExists(c.ComposePlugin) {
			return c.ComposePlugin
		}
		return ""
	}
	home, _ := os.UserHomeDir()
	paths := []string{
		os.Getenv("COMPOSE_PLUGIN"),
		filepath.Join(home, ".rain", "toolchain", "kotlinc", "lib", "compose-compiler-plugin.jar"),
		filepath.Join(home, "kotlin", "kotlinc", "lib", "compose-compiler-plugin.jar"),
	}
	if kotlinc, err := exec.LookPath("kotlinc"); err == nil {
		if real, err := filepath.EvalSymlinks(kotlinc); err == nil {
			paths = append(paths, filepath.Join(filepath.Dir(real), "..", "lib", "compose-compiler-plugin.jar"))
		}
	}
	for _, p := range paths {
		if p != "" && fileExists(p) {
			return p
		}
	}
	return ""
}

// findKotlinSources lista las fuentes .kt del proyecto.
func (c Config) findKotlinSources() []string {
	var sources []string
	_ = filepath.Walk(sourceRoot(c.AppDir), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info != nil && !info.IsDir() && strings.HasSuffix(path, ".kt") {
			sources = append(sources, path)
		}
		return nil
	})
	return sources
}

// kotlinJVMTarget traduce el target numérico al formato de kotlinc: Kotlin 2.x
// ya no acepta "8" (solo "1.8" y de "9" en adelante).
func kotlinJVMTarget(target int) string {
	if target <= 8 {
		return "1.8"
	}
	return strconv.Itoa(target)
}
