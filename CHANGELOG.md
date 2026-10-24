# Changelog

Todos los cambios notables de Rain se documentan aquí. El formato sigue
[Keep a Changelog](https://keepachangelog.com/es-ES/1.1.0/) y el proyecto usa
[Versionado Semántico](https://semver.org/lang/es/).

Las versiones se calculan automáticamente a partir de los mensajes de commit
(Conventional Commits): `feat:` sube la *minor*, `fix:`/`perf:`/`refactor:` la
*patch*, y `!`/`BREAKING CHANGE` la *major*.

## [No publicado]

### Añadido

- **`rain check`: comprobación de tipos sin generar APK.** Compila el código
  (Kotlin y Java) con el mismo `android.jar`, el mismo classpath y el mismo
  plugin de Compose que `rain build`, y se detiene ahí: sin DEX, sin
  empaquetado y sin firma. Es la única forma de detectar errores como
  `unresolved reference 'setContentTitle'` sin pagar un build completo, porque
  esos errores son del compilador y ningún linter de estilo los ve.
  - `rain check --fast` hace el chequeo incremental: solo recompila los
    ficheros modificados desde la última compilación correcta (y los que
    dependen de ellos, detectados por las declaraciones que mencionan)
    contra las clases ya compiladas. Es más rápido, pero menos exhaustivo: un
    error en un fichero que no se recompila puede no aparecer.
- **`rain lint`: analizadores estáticos opcionales.** Ejecuta ktlint, detekt y
  pmd sobre las fuentes y agrupa los avisos por fichero, con un resumen por
  herramienta. Se configura con la sección `[lint]` de `rain.toml`
  (`tools`, `fail_on`, `type_resolution`) o con los flags `--tools`,
  `--fail-on` y `--type-resolution`. La resolución de tipos de detekt viene
  desactivada por defecto: con el compilador de Kotlin embebido provoca un
  `NullPointerException` interno (`PrimitiveTypeUtil.unsignedType`).
- **ktlint y detekt en `rain doctor`.** Aparecen como comprobaciones
  opcionales (no bloquean el estado `Ready`) y se descargan con
  `rain doctor --fix --lint` o con `rain lint --install`; quedan en
  `~/.rain/toolchain` (ktlint 1.8.0 y detekt 1.23.8).
- **Caché de dependencias preparadas.** El conjunto de AAR/JAR ya extraídos se
  guarda en `.rain-deps/.rain-deps.json` junto con el hash de sus
  coordenadas. Si las dependencias no han cambiado, la siguiente ejecución los
  reutiliza en lugar de volver a descargarlos y extraerlos: en un proyecto con
  Compose son decenas de artefactos y se ahorran entre 8 y 14 s por
  ejecución.
- Repositorios Maven por coordenada: JitPack se consulta solo para grupos
  `com.github.*`/`io.github.*`, y `[repositories].extra` en `rain.toml` permite
  declarar repos de terceros sin afectar al resto de builds.
- Test de integración que compila y firma un APK real con una librería AndroidX
  sintética y verifica el manifiesto fusionado, la tabla de recursos y el dex
  (`internal/build/integration_test.go`).

### Corregido

- **Un `rain check` envenenaba el `rain build` siguiente, que empaquetaba el
  DEX viejo.** El estado de compilación se guardaba en un único fichero de
  marca (`.rain-java-hash`) que servía para dos cosas distintas: saber si las
  fuentes compilan (lo necesita `check`) y decidir si el `classes.dex` se puede
  reutilizar (lo necesita `build`). Pero `check` no genera DEX: al terminar
  escribía la marca y el `build` siguiente veía "las fuentes no han cambiado",
  reutilizaba el `classes.dex` de la compilación anterior y generaba un APK
  **con código que ya no estaba en el disco**. Se hacía visible al compilar,
  instalar y ver en el dispositivo un comportamiento antiguo sin haber
  tocado el código. Ahora hay dos marcas: `.rain-java-hash` (las fuentes
  compilan) y `.rain-dex-hash` (el DEX corresponde a esas fuentes), y solo un
  build que llega a generar el DEX escribe la segunda
  (tests en `internal/build/dexcache_test.go`).
- **Las clases de los ficheros borrados se quedaban en el DEX.** La
  compilación completa no limpiaba el directorio de objetos, así que al
  borrar o renombrar un fichero, sus clases viejas seguían ahí y se
  empaquetaban en el APK. Ahora la compilación completa empieza de cero.
- **Un APK con `classes.dex` duplicado hacía fallar la firma.** El APK base se
  reutiliza cuando los recursos no cambian, y tras empaquetarlo ya contenía
  el `classes.dex` y las librerías nativas. Al empaquetar de nuevo, esas
  entradas se copiaban *y* se añadían otra vez, así que el APK acababa con
  dos `classes.dex` y `apksigner` lo rechazaba con `Multiple ZIP entries with
  the same name: classes.dex` (reproducible con `examples/neko` al recompilar
  sin tocar los recursos). Ahora `appendDex` omite del APK base las entradas
  que va a añadir, de modo que el empaquetado es idempotente
  (tests en `internal/build/packaging_test.go`).
- **El classpath podía contaminarse con artefactos rancios del caché.** Al
  resolver dependencias, `libraryJars()` metía en el classpath *todos* los jars
  encontrados en el caché local, no solo los del conjunto resuelto. Bastaba un
  jar antiguo con contenido distinto en el caché para que el APK cambiase entre
  builds supposedamente idénticos. Ahora, cuando hay dependencias resueltas, el
  classpath sale exclusivamente de ese conjunto. Con el fallo, el hash del APK
  de `JetIsland_Dynamic-Island-Jetpack-Compose` pasaba de
  `cac4405bc08276ded203550d1d7d3d027bec1810dc3637f011b7202109612323` a
  `5ea858fe…`; con el arreglo, el hash vuelve a ser estable.
- **La fusión de manifests dejaba prefijos sin enlazar.** Los fragmentos
  copiados de las librerías conservaban sus atributos con prefijo (por ejemplo
  `tools:node="merge"` de AndroidX Startup) pero no la declaración
  `xmlns:tools`, que solo vivía en el manifest de la librería. El manifest
  fusionado quedaba mal formado y **aapt2 rechazaba el build** con
  `unbound prefix` (reproducible con `examples/compose` y `examples/neko`).
  Ahora los namespaces que usan los fragmentos copiados se declaran en el
  elemento raíz del manifest fusionado.
- **Las variantes Android de los módulos KMP se perdían.** La búsqueda de la
  variante Android en el Gradle Module Metadata (`.module`) solo aceptaba el
  atributo `org.gradle.jvm.environment`, que no todos los módulos publican.
  `androidx.compose.material:material-icons-core:1.7.8`, por ejemplo, identifica
  su variante Android solo con `org.jetbrains.kotlin.platform.type=androidJvm`
  y con el módulo al que apunta (`material-icons-core-android`). Resultado: la
  raíz KMP se quedaba como AAR "cáscara" (sin clases) y kotlinc fallaba con
  `unresolved reference 'Icons'`. Ahora la variante Android se reconoce por
  cualquiera de los tres indicios, sin dejar de preferir la que declara el
  entorno (tests en `internal/pom/module_test.go`).
- `rain doctor` ya no mostraba la ruta de una versión como válida cuando el
  comando fallaba al ejecutarse (`runVersion`).

### Cambiado

- El hash de las fuentes compiladas se guarda **después** de compilar, no
  después de generar el DEX. Así un `rain check` posterior puede saltarse el
  trabajo si el código no ha cambiado, y un `rain build` reutiliza la
  compilación en lugar de repetirla.
- `javaTarget` por defecto 17 (el mismo valor que usa AGP); se admiten 8, 11,
  17 y 21.

### Eliminado

- Código muerto: el paquete `internal/resolver` (`DownloadAAR`/
  `DownloadArtifact`, sin ningún importador) y `Project.EffectiveProperties`
  (obsoleto frente a `MergeProperties`).

## [0.1.0] - 2026-10-21

Primera versión etiquetada. Núcleo funcional: compila, firma y verifica APKs
reales de Android desde Termux/Alpine, sin Gradle ni Android Studio.

### Añadido

- CLI basada en Cobra: `init`, `build`, `clean`, `doctor`, `version`.
- `rain doctor` con motor de recomendaciones y toolchain autoprovisionado en
  `~/.rain` (Kotlin 2.4.21, R8 9.5.28, android.jar, apksigner).
- Matriz de compatibilidad JDK/R8/Kotlin/SDK (`docs/compatibilidad.md`).
- Scaffolding real de proyectos: plantilla `simple` (Java) y `completo`
  (Kotlin + Compose con AndroidX).
- Resolución de dependencias leyendo POMs de Maven (transitivas, "highest wins",
  properties y `dependencyManagement`).
- Soporte AAR completo: `classes.jar`, `res/`, `jni/<abi>/*.so`, manifest,
  `R.txt` y reglas ProGuard; fusión de recursos tipo AGP.
- Multidex y build de release con R8 real (reglas base + de AAR, `mapping.txt`,
  `apksigner verify`).
- Modo release con firma PKCS12 y soporte de keystore; APK nombrado
  `<nombre>-<versionName>-<abi>.apk` al estilo android2apk.
- Configuración completa en `rain.toml` (`[build]` con `min_sdk`, `target_sdk`,
  `version_code`, `version_name`, `application_id`, `java_target`), con
  `app.properties` como fallback.
- `BuildConfig.java` real (`DEBUG`, `APPLICATION_ID`, `BUILD_TYPE`,
  `VERSION_CODE`, `VERSION_NAME`).
- Mensaje final del build con tamaño del APK y tiempo empleado.

### Cambiado

- Plugin de Compose aplicado solo si el proyecto importa `androidx.compose`.
- Orden de compilación: Java generado (R.java/BuildConfig.java) → Kotlin → Java
  de la app, con `objDir` en el classpath de `kotlinc`.
- Dedupe global de dependencias por `group:artifact` (una sola versión, la más
  alta) para evitar clases contradictorias en el classpath.
- Si un JAR es un stub de Kotlin Multiplatform (sin `.class`), se descarga la
  variante `-jvm`.

[No publicado]: https://github.com/itskreisler/rain/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/itskreisler/rain/releases/tag/v0.1.0
