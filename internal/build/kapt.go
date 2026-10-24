package build

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/itskreisler/rain/internal/memenv"
	"github.com/itskreisler/rain/internal/ui"
)

// KaptConfig describe los annotation processors (kapt) que se ejecutan antes
// de compilar el código.
//
// Kapt es el mecanismo que usa Kotlin para ejecutar los annotation processors
// de Java (Dagger, Hilt, Room...) sobre el código Kotlin: se generan stubs
// Java de las clases Kotlin, se pasan por el processor y el Java que genera
// se compila junto al resto.
type KaptConfig struct {
	// ProcessorJars son los jars de los processors, ya descargados. Forman el
	// "processor path" de la ronda de annotation processing.
	ProcessorJars []string
	// Options son opciones para los processors, en formato "clave=valor" (se
	// pasan al processor como -Aclave=valor).
	Options []string
}

// hiltProcessorMarker es el fragmento del nombre del artifact que trae el
// processor de Hilt (hilt-compiler o hilt-android-compiler).
const hiltProcessorMarker = "hilt-compiler"

// isHiltProcessor indica si alguno de los processors es el de Hilt.
func (k *KaptConfig) isHiltProcessor() bool {
	if k == nil {
		return false
	}
	for _, jar := range k.ProcessorJars {
		if strings.Contains(filepath.Base(jar), hiltProcessorMarker) {
			return true
		}
	}
	return false
}

// processorOptions devuelve las opciones (-A) que se pasan a los processors.
//
// Para Hilt se añade la misma opción que pasa su plugin de Gradle:
//
//   - disableAndroidSuperclassValidation: sin ella el processor exige que la
//     clase ya extienda de Hilt_XXX, algo que solo puede conseguir el plugin
//     de Gradle con su transform de bytecode. Rain hace ese transform
//     después de compilar (ver internal/hilt), así que la opción es
//     necesaria para que la ronda de annotation processing tenga éxito.
//
// El resto de opciones que pasa el plugin de Gradle (dagger.fastInit,
// projectType...) son optimizaciones o validaciones, y aquí no aportan nada.
func (k *KaptConfig) processorOptions() []string {
	if k == nil {
		return nil
	}
	opts := append([]string{}, k.Options...)
	if k.isHiltProcessor() {
		opts = append(opts,
			"dagger.hilt.android.internal.disableAndroidSuperclassValidation=true",
		)
	}
	return opts
}

// runKapt ejecuta kapt sobre las fuentes Kotlin: genera los stubs y corre la
// ronda de annotation processing.
//
// Devuelve el directorio con el Java generado por los processors (vacío si no
// generaron nada).
//
// Detalles que no son obvios:
//
//   - El launcher de kapt es el propio compilador de Kotlin con otro punto de
//     entrada (KaptCli). Son las variables KOTLIN_TOOL y KOTLIN_COMPILER que
//     usa el script bin/kapt de la distribución de kotlinc.
//   - El modo "stubsAndApt" es el único válido en Kotlin 2.x: genera los
//     stubs y corre la ronda de annotation processing. La compilación del
//     Kotlin la hace después el propio kotlinc (el modo "compile" ya no
//     existe).
//   - Los jars de los processors van SOLO en el processor path, y cada uno en
//     una entrada propia (-Kapt-classpath repetido). El motivo es doble: kapt
//     los descubre con un ServiceLoader sobre el processor path, y si además
//     estuvieran en el classpath de compilación, el processor vería sus propias
//     librerías (Guava) y Dagger generaría código que las usa (ImmutableMap),
//     obligando a meter Guava también en el APK. Con el processor path
//     separado, Dagger no ve Guava y el código generado es el mismo que
//     genera el plugin de Gradle.
func runKapt(cfg Config, kotlinFiles []string, objDir, kaptDir string) (string, error) {
	stubsDir := filepath.Join(kaptDir, "stubs")
	sourcesDir := filepath.Join(kaptDir, "sources")
	classesDir := filepath.Join(kaptDir, "classes")

	for _, dir := range []string{stubsDir, sourcesDir, classesDir} {
		if err := os.RemoveAll(dir); err != nil {
			return "", fmt.Errorf("limpiando %s: %w", dir, err)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("creando %s: %w", dir, err)
		}
	}

	classpath := cfg.kaptClasspath(objDir)
	processorPath := cfg.kaptProcessorPath()

	args := []string{
		"-d", objDir,
		"-classpath", strings.Join(classpath, ":"),
		"-jvm-target", kotlinJVMTarget(cfg.jvmTarget()),
		"-Kapt-mode=stubsAndApt",
		"-Kapt-sources=" + sourcesDir,
		"-Kapt-classes=" + classesDir,
		"-Kapt-stubs=" + stubsDir,
		"-Kapt-correct-error-types=true",
	}

	// El processor path se pasa jar a jar, con una opción por jar. El CLI de
	// kapt mete el valor de -Kapt-classpath en la lista del processor path
	// como un único path (no lo parte por el separador), así que un classpath
	// con varios jars no funcionaría: la classloader que usa kapt para
	// descubrir los processors se quedaría con un path inválido y no
	// encontraría ninguno ("No annotation processors available").
	for _, jar := range processorPath {
		args = append(args, "-Kapt-classpath="+jar)
	}

	// El plugin de kapt solo admite una opción de annotation processing por
	// invocation (la que más importa va primero).
	options := cfg.Kapt.processorOptions()
	for i, opt := range options {
		if i > 0 {
			ui.Warn("kapt: solo se pasa la primera opción de annotation processing (%s); se ignora %s", options[0], opt)
			break
		}
		args = append(args, "-Kapt-option:"+opt)
	}

	// La JVM que corre kapt lee el @Metadata de todas las clases del
	// classpath, así que necesita un heap parecido al de la compilación.
	if !memenv.UserHeapOverride() {
		heap := memenv.HeapLimit(memenv.DefaultKotlincHeap)
		ui.VerboseLog("heap de kapt: %s", heap)
		args = append(args, "-J-Xmx"+heap)
	}

	args = append(args, kotlinFiles...)

	ui.Info("ejecutando kapt (annotation processing)...")
	cmd := exec.Command("kotlinc", args...)
	cmd.Env = append(os.Environ(),
		"KOTLIN_TOOL=kotlin-annotation-processing-cli.jar",
		"KOTLIN_COMPILER=org.jetbrains.kotlin.kapt.cli.KaptCli",
	)
	var stderr bytes.Buffer
	cmd.Stdout = os.Stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		ui.Error("Errores de kapt:")
		fmt.Println(stderr.String())
		return "", fmt.Errorf("kapt falló")
	}

	generated, err := collectJavaSources(sourcesDir)
	if err != nil {
		return "", fmt.Errorf("buscando el Java generado por kapt: %w", err)
	}
	if len(generated) > 0 {
		ui.Info("kapt generó %d ficheros Java", len(generated))
	}
	return sourcesDir, nil
}

// kaptClasspath es el classpath de la ronda de annotation processing.
//
// Incluye el stdlib de Kotlin y las anotaciones de JetBrains: los stubs que
// genera kapt llevan @kotlin.Metadata y @NotNull, y sin ellas el processor
// no puede resolver los tipos.
//
// Los jars de los processors NO van aquí, solo en el processor path. Es
// fundamental que sea así: este classpath es el que ve el processor cuando
// genera código, y si Guava estuviera en él, Dagger decidiría usar
// ImmutableMap en el código que genera. Ese código se compila después contra
// el classpath de la app (sin Guava) y fallaría, y el APK necesitaría Guava
// en runtime. Con Guava solo en el processor path, el código generado es
// idéntico al que genera el plugin de Hilt de Gradle.
func (c Config) kaptClasspath(objDir string) []string {
	classpath := []string{c.AndroidJar}
	classpath = append(classpath, c.libraryJars()...)
	classpath = append(classpath, objDir)
	if stdlib := findKotlinStdlib(); stdlib != "" {
		classpath = append(classpath, stdlib)
	}
	if annotations := findJetbrainsAnnotations(); annotations != "" {
		classpath = append(classpath, annotations)
	}
	return classpath
}

// kaptProcessorPath es el "processor path": los jars de los processors.
//
// Es el classpath con el que kapt carga los processors, así que aquí sí van
// todas sus dependencias (Guava, javapoet, dagger-spi...). Por eso se pasan
// separados del classpath de compilación: el processor puede usarlos, pero el
// código que genera no debe verlos.
//
// El stdlib va aquí también: hay processors (como el de Hilt) que cargan
// clases de Kotlin (kotlin.jvm.functions.Function1) al instanciarse, y sin
// el stdlib en el processor path el processor se cuelga con un
// ClassNotFoundException que kapt reporta como "No annotation processors
// available".
func (c Config) kaptProcessorPath() []string {
	var path []string
	if c.Kapt != nil {
		path = append(path, c.Kapt.ProcessorJars...)
	}
	if stdlib := findKotlinStdlib(); stdlib != "" {
		path = append(path, stdlib)
	}
	return path
}
