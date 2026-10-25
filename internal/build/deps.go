package build

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/itskreisler/rain/internal/aar"
	"github.com/itskreisler/rain/internal/ui"
)

// libraryJars devuelve los jars de clases: los classes.jar de los AAR y los
// JAR sueltos preparados en DepsDir, más los .jar de LibsDir. Cuando hay un set
// de dependencias (Libs) se usa su lista explícita, que incluye tanto los
// classes.jar (en jars/) como los .jar que van directamente en aars/.
//
// Cuando hay set, NO se recorre DepsDir: el caché conserva artefactos de
// resoluciones anteriores (versiones viejas que ya no están en el grafo) y
// metarlos en el classpath haría que el APK dependiera del historial de la
// caché y no solo de las dependencias resueltas.
func (c Config) libraryJars() []string {
	jars := make(map[string]bool)
	var out []string
	add := func(path string) {
		if path != "" && !jars[path] {
			jars[path] = true
			out = append(out, path)
		}
	}

	if c.Libs != nil {
		for _, jar := range c.Libs.ClassJars() {
			add(jar)
		}
	}

	if c.LibsDir != "" {
		if entries, err := os.ReadDir(c.LibsDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".jar") {
					add(filepath.Join(c.LibsDir, e.Name()))
				}
			}
		}
	}

	// Sin set explícito solo queda el directorio de dependencias como fuente
	// de jars (modo simple con un libs/ prepopulado).
	if c.Libs == nil && c.DepsDir != "" {
		jarDir := filepath.Join(c.DepsDir, "jars")
		if entries, err := os.ReadDir(jarDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".jar") {
					add(filepath.Join(jarDir, e.Name()))
				}
			}
		}
		// Los artefactos que son JAR plano (no AAR) se descargan directamente
		// en aars/ y también deben entrar en el classpath.
		aarDir := filepath.Join(c.DepsDir, "aars")
		if entries, err := os.ReadDir(aarDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".jar") {
					add(filepath.Join(aarDir, e.Name()))
				}
			}
		}
	}

	return out
}

// warnLibsWithDependencies avisa cuando hay jars sueltos en libs/ además de un
// conjunto de dependencias resuelto.
//
// libs/ se escanea siempre por compatibilidad con proyectos que meten jars
// locales a mano. En modo completo eso también incluye restos de versiones
// antiguas: el dedupe elimina las clases que chocan con el set actual, pero las
// clases que solo existen en un jar viejo sobreviven y acaban en el APK. Un
// aviso explícito evita que esa mezcla pase desapercibida.
func (c Config) warnLibsWithDependencies() {
	if c.Libs == nil || c.LibsDir == "" {
		return
	}
	entries, err := os.ReadDir(c.LibsDir)
	if err != nil {
		return
	}
	var jars int
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jar") {
			jars++
		}
	}
	if jars > 0 {
		ui.Warn("%s contiene %d jar(s) además de las dependencias de rain.toml; se incluirán en el APK. Bórralos si son restos de versiones antiguas.", c.LibsDir, jars)
	}
}

// prepareLibraryResources fusiona los res/ de las librerías (si hay) y devuelve
// el directorio fusionado y los packages para --extra-packages.
func (c Config) prepareLibraryResources(libs *aar.Set) (string, []string, error) {
	if libs == nil {
		return "", nil, nil
	}
	resDirs := libs.ResDirs()
	if len(resDirs) == 0 {
		return "", libs.ExtraPackages(), nil
	}
	merged := filepath.Join(c.BuildDir, "libres-merged")
	if err := aar.MergeResources(resDirs, merged); err != nil {
		return "", nil, err
	}
	return merged, libs.ExtraPackages(), nil
}

// libraryResourceInputs devuelve las rutas de los recursos y de los manifests
// de las librerías. Entran en el hash incremental de recursos: si cambian,
// el APK base (resources.arsc y manifest fusionado) debe rehacerse.
func (c Config) libraryResourceInputs() []string {
	if c.Libs == nil {
		return nil
	}
	out := c.Libs.ResDirs()
	out = append(out, c.Libs.Manifests()...)
	return out
}

// nativeLibs devuelve las .so que deben empaquetarse. Con un set de
// dependencias se usan solo las del set; recorrer jni/ entero empaquetaría las
// de dependencias que ya no están en rain.toml pero siguen en disco.
func (c Config) nativeLibs() (map[string]string, error) {
	if c.Libs == nil {
		// Modo simple: sin AARs no hay jni/ que empaquetar.
		return nil, nil
	}
	return c.Libs.NativeLibs(), nil
}

// dedupeJarsByClass resuelve definiciones duplicadas de clases entre jars. Los
// primeros jars de la lista tienen prioridad. Cuando un jar posterior comparte
// solo *algunas* clases con los anteriores (habitual en el grafo de AndroidX:
// variantes -android/-ktx, versiones solapadas), se reempaqueta sin esas clases
// en scratchDir y se usa el jar filtrado. Si todas sus clases están duplicadas
// se descarta por completo.
//
// d8/R8 no resuelven dependencias transitivas y fallan con "Type X is defined
// multiple times"; filtrar a nivel de clase evita repackaging manual del grafo.
func dedupeJarsByClass(jars []string, scratchDir string) []string {
	seen := make(map[string]bool)
	var out []string

	for _, jar := range jars {
		classes, err := jarClassNames(jar)
		if err != nil || len(classes) == 0 {
			// Si no se pueden leer las clases, se conserva el jar tal cual.
			out = append(out, jar)
			continue
		}
		unique := make([]string, 0, len(classes))
		for _, class := range classes {
			if !seen[class] {
				unique = append(unique, class)
			}
		}
		if len(unique) == len(classes) {
			// Sin solapes: se conserva el jar original.
			for _, class := range classes {
				seen[class] = true
			}
			out = append(out, jar)
			continue
		}
		if len(unique) == 0 {
			// Todas sus clases ya estaban: se descarta por completo.
			ui.VerboseLog("dedupe: %s descartado (clases duplicadas)", filepath.Base(jar))
			continue
		}
		// Solape parcial: reempaquetar conservando solo las clases únicas y el
		// resto de entradas (recursos, etc.).
		stripped, err := stripJarClasses(jar, scratchDir, seen)
		if err != nil {
			ui.Warn("dedupe: no se pudo filtrar %s: %v", filepath.Base(jar), err)
			out = append(out, jar)
			for _, class := range classes {
				seen[class] = true
			}
			continue
		}
		ui.VerboseLog("dedupe: %s filtrado (%d clases solapadas)", filepath.Base(jar), len(classes)-len(unique))
		for _, class := range classes {
			seen[class] = true
		}
		out = append(out, stripped)
	}
	return out
}

// jarClassNames devuelve los nombres de clase (con /) definidos en un jar.
func jarClassNames(jar string) ([]string, error) {
	zr, err := zip.OpenReader(jar)
	if err != nil {
		return nil, err
	}
	defer func() { _ = zr.Close() }()

	var classes []string
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, ".class") {
			classes = append(classes, strings.TrimSuffix(f.Name, ".class"))
		}
	}
	return classes, nil
}

// stripJarClasses escribe en scratchDir un jar que conserva todas las entradas
// no-.class (recursos embebidos, etc.) y solo las clases cuyo nombre no esté ya
// en seen. El nombre deriva del original para que sea estable entre builds.
func stripJarClasses(jar, scratchDir string, seen map[string]bool) (string, error) {
	zr, err := zip.OpenReader(jar)
	if err != nil {
		return "", err
	}
	defer func() { _ = zr.Close() }()

	if err := os.MkdirAll(scratchDir, 0o755); err != nil {
		return "", err
	}
	dest := filepath.Join(scratchDir, "dedupe-"+filepath.Base(jar))
	tmp := dest + ".tmp"

	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	zw := zip.NewWriter(f)
	for _, entry := range zr.File {
		if strings.HasSuffix(entry.Name, ".class") {
			name := strings.TrimSuffix(entry.Name, ".class")
			if seen[name] {
				continue // clase duplicada: se omite
			}
		}
		w, err := zw.Create(entry.Name)
		if err != nil {
			_ = zw.Close()
			_ = f.Close()
			return "", err
		}
		rc, err := entry.Open()
		if err != nil {
			_ = zw.Close()
			_ = f.Close()
			return "", err
		}
		_, err = io.Copy(w, rc)
		_ = rc.Close()
		if err != nil {
			_ = zw.Close()
			_ = f.Close()
			return "", err
		}
	}
	if err := zw.Close(); err != nil {
		_ = f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, dest); err != nil {
		return "", err
	}
	return dest, nil
}

// defaultProguardRules son las reglas base de Rain para release. Se escriben en
// un archivo generado (nunca pisan el proguard.pro del usuario). Mantienen lo
// esencial que Android necesita en runtime: componentes declarados en el
// manifest, clases de entrada y metadatos que la reflexión usa.
const defaultProguardRules = `# Reglas base de Rain (generadas; no editar).
# Rain no conoce aún el manifest final, así que conserva las clases que el
# framework instancia por reflexión.
-keep public class * extends android.app.Activity
-keep public class * extends android.app.Application
-keep public class * extends android.app.Service
-keep public class * extends android.content.BroadcastReceiver
-keep public class * extends android.content.ContentProvider
-keep public class * extends android.view.View {
    public <init>(android.content.Context);
    public <init>(android.content.Context, android.util.AttributeSet);
    public <init>(android.content.Context, android.util.AttributeSet, int);
}
# Views infladas desde XML.
-keepclasseswithmembers class * {
    public <init>(android.content.Context, android.util.AttributeSet);
}
-keepattributes *Annotation*, Signature, InnerClasses, EnclosingMethod
# Conserva la metadata de Kotlin (necesaria para reflexión/Compose en release).
-keep class kotlin.Metadata { *; }
-keepclassmembers class **$WhenMappings { <fields>; }
# AndroidX referencia clases que solo existen en versiones nuevas de la
# plataforma o en artefactos opcionales (no están en android.jar ni en el
# classpath). D8 las ignora; R8 las valida y aborta con "Missing class".
# AGP emite estos mismos -dontwarn por defecto para sus dependencias.
-dontwarn androidx.emoji2.**
-dontwarn androidx.window.**
-dontwarn androidx.profileinstaller.**
-dontwarn androidx.tracing.**
-dontwarn androidx.startup.**
-dontwarn kotlinx.coroutines.debug.**
# Anotaciones de JetBrains: solo metadatos de compilación, no van al runtime.
-dontwarn org.jetbrains.annotations.**
`

// proguardFiles reúne las reglas que R8 debe aplicar además de las base: las
// del proyecto (si existen), las que se indiquen en la configuración y los
// consumer-rules de los AAR.
//
// Las reglas se buscan en dos sitios: en el directorio del módulo y en su
// src/main. En un proyecto Gradle viven en el directorio del módulo
// (app/proguard-rules.pro), no en src/main, así que buscar solo en src/main
// las dejaría fuera. Sin ellas, un release construido con rain se quedaría
// sin los -keep de los accesos por reflexión ni con el -dontoptimize que
// algunos proyectos necesitan (R8 optimiza de forma distinta a AGP).
func (c Config) proguardFiles() []string {
	var files []string

	for _, dir := range []string{sourceRoot(c.AppDir), c.AppDir} {
		for _, name := range []string{"proguard-rules.pro", "proguard.pro"} {
			path := filepath.Join(dir, name)
			if fileExists(path) {
				files = append(files, path)
			}
		}
	}
	files = append(files, c.ProguardFiles...)
	if c.Libs != nil {
		files = append(files, c.Libs.ProguardFiles()...)
	}

	// Deduplica conservando el orden.
	seen := make(map[string]bool)
	var out []string
	for _, f := range files {
		if f != "" && !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	return out
}
