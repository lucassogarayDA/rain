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
- **Detección automática del layout plano.** `rain build` y `rain check` sin
  argumentos ya no exigen `rain.toml`: si el directorio actual tiene un
  `AndroidManifest.xml` (en la raíz o en `src/main/`), compilan con el layout
  plano estilo android2apk, igual que `rain build <dir>`.
- Aviso cuando `libs/` contiene jars sueltos además de un conjunto de
  dependencias resuelto: pueden ser restos de versiones antiguas cuyas clases
  únicas acabarían en el APK. Se siguen incluyendo, pero ya no pasa
  desapercibido.
- **Ejemplo `examples/backdrop-music` completo.** El reproductor de cristal
  pasa de una demo de dos pestañas a una app real con Inicio (recientes,
  álbumes y acceso al reproductor), Biblioteca (canciones, artistas, álbumes,
  géneros y carpetas, con búsqueda y filtros), Reproductor (portada, progreso,
  controles, cola y ecualizador real) y Configuración (audio, apariencia,
  biblioteca, reproducción y almacenamiento). Añade aleatorio, repetición,
  mini-reproductor, ajustes persistidos en `SharedPreferences`, acento
  dinámico extraído de la carátula y transiciones animadas entre páginas y al
  cambiar de canción. El README incluye una checklist con lo implementado y lo
  pendiente (servicio en segundo plano, preajustes de ecualizador, letras,
  temporizador, radio por Internet, widgets y Android Auto).

### Corregido

- **Los alias de Compose Multiplatform vaciaban el classpath.** Al depender de
  una librería KMP como `io.github.kyant0:backdrop:2.0.1`, su metadata arrastra
  artefactos `org.jetbrains.compose.*` (`foundation-android`, `ui-android`…).
  En Android no son más que alias: publican un AAR casi vacío cuyo único
  contenido útil es la dependencia `androidx.compose.*` equivalente. Como
  comparten nombre de archivo con el AAR real, al descargarse en paralelo lo
  sobrescribían en `.rain-deps/aars`, y kotlinc fallaba con cientos de
  `unresolved reference` en `androidx.compose.*` de forma no determinista.
  Ahora `ResolveTransitive` sigue recorriendo sus POMs para descubrir AndroidX,
  pero los excluye del conjunto final y de la deduplicación
  (`isComposeMultiplatformAlias`, tests en `internal/pom/pom_test.go`).
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
- **Sobrevivían artefactos de builds anteriores.** Al reutilizar el APK base se
  descartan todos los `classes*.dex`, no solo los que este build va a escribir:
  si antes había más dependencias, los DEX sobrantes se quedaban con clases
  viejas. Además solo se empaquetan las `.so` del set actual, no todo
  `.rain-deps/jni`, y cada AAR extrae las suyas en su propio subdirectorio para
  que dos dependencias con el mismo nombre (p. ej. `libc++_shared.so`) no se
  pisen al prepararse en paralelo.
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

- **Icono propio en `examples/backdrop-music`.** La app deja el icono por
  defecto del sistema y estrena un icono adaptativo vectorial (degradado
  índigo, nota musical y halo) con capa monocroma para los iconos temáticos de
  Android 13+. El README añade capturas reales de Inicio, Biblioteca y
  Reproductor.
- **Etiquetas de acción de Configuración en blanco.** En
  `examples/backdrop-music`, “Abrir” y “Escanear” dejan de usar el acento
  dinámico y pasan a blanco con una cápsula algo más opaca, para asegurar el
  contraste del texto pequeño sobre el cristal. El acento se mantiene en
  iconos, selección, progreso y escena, y su extracción queda documentada en el
  README del ejemplo.
- **El build es mucho más rápido, sobre todo con un cambio de código.** Varias
  fases dejaron de repetir trabajo en cada ejecución:
  - El cierre transitivo de dependencias se cachea por coordenadas y repos
    (`cache/.rain-resolved-*.json`), sin recorrer cientos de POMs por build.
  - Los AAR/JAR se preparan en paralelo y el conjunto extraído se reutiliza
    (`.rain-deps/.rain-deps.json`).
  - El Java generado (`R.java`, `BuildConfig.java`) se compila una vez y se
    cachea en `build/genclasses/` (`.rain-gen-hash`).
  - En debug, el DEX de dependencias se cachea en `build/dexdeps/`
    (`.rain-dex-deps-hash`) y solo se regenera si cambian los jars, `min-api`,
    `android.jar`, d8 o el propio binario. La app se dexa aparte y ocupa
    `classes.dex`; las dependencias van después como `classes2.dex`,
    `classes3.dex`… En release se mantiene la pasada única de R8.
  - d8/R8 usan un heap de 2 GB por defecto (`RAIN_JAVA_MAX_HEAP`), que evita
    minutos de GC en proyectos Compose.
  - El empaquetado evita copias intermedias y ya no conserva `aligned.apk`.

  Medido en `examples/compose`: build con cambio de código ~199 s → ~60 s;
  build frío con la red ya cacheada ~735 s → ~160–210 s; sin cambios
  ~7,7 s → ~5–9 s.
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
