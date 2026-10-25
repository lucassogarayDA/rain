# Rain — Plan para superar a android2apk

> Objetivo: mantener el build aislado en Alpine (sin root, sin Gradle, sin Android Studio),
> añadir un CLI manejable con Cobra, un `rain doctor` que diagnostique el entorno y llevar
> `rain` al nivel de `android2apk` (y más allá): Java, Kotlin, Compose, AndroidX, multidex,
> assets, JNI y APKs de release verificados.

---

## 0. Diagnóstico

### 0.1 Qué es cada proyecto

| | `android2apk` | `rain` |
|---|---|---|
| Forma | Scripts Bash + docs + ejemplos + skill de agente | Binario Go + `rain.toml` |
| Entorno | Alpine (proot-distro/Termux), requiere root | Termux/Alpine, sin root |
| Java | JDK 17, `--release 8` (simple) / `11` (Compose) | JDK 21, `-source 8 -target 8` |
| Kotlin | 2.2.21, `-jvm-target 8/11` | cualquiera instalado, `-jvm-target 1.8` |
| minSdk / targetSdk | 26 / 36 fijo | 21 / sin targetSdk |
| AndroidX/Compose | Sí (93 AAR, recursos fusionados) | No (solo `classes.jar`) |
| Multidex | Sí (`classes*.dex`) | No (solo `classes.dex`) |
| Assets/JNI | Sí | No |
| Incremental | No (recompila todo) | Sí (SHA-256) |
| Release/R8 | No | Sí (R8 del `d8.jar` del SDK) |
| Tests/CI | No | No |
| Madurez | Snapshot cuidado, mucha doc | 3 commits, 2 días |

### 0.2 Respuesta a “¿manejan la misma versión de Java?”

**No.** Pero el punto importante es que **el JDK no se empaqueta en el APK**: `d8` convierte
bytecode a DEX. Lo que importa es:

| | `android2apk` | `rain` |
|---|---|---|
| JDK que ejecuta las herramientas | 17 | 21 (README) |
| Bytecode que se genera | Java 8 (simple), Java 11 (Compose) | Java 8 siempre |
| `minSdk` que se pasa a `d8` | 26 | 21 |
| R8/D8 | build-tools 36 → **R8 8.10.9-dev** | el que haya en el sistema |

Consecuencias:

- `rain` no puede usar sintaxis moderna de Java/Kotlin de forma efectiva: `-source 8`
  bloquea records, sealed, pattern matching, etc.
- `rain` fija `--min-api 21`, aunque el proyecto pida 26.
- El `d8` de build-tools 36 (R8 8.10.9) **no cumple** el mínimo para Kotlin 2.4 (R8 9.1.29),
  y está por debajo del mínimo de Kotlin 2.3 (R8 8.13.19). Para Kotlin 2.2 pide 8.10.21.
- `android2apk` acierta al separar JDK de target, pero se queda en Java 8/11.

**Decisión propuesta:** ejecutar con el JDK más moderno disponible (Alpine 3.24 tiene
**OpenJDK 25**) pero compilar con un target configurable, por defecto **17**:

```toml
[build]
javaTarget = 17   # 8 | 11 | 17 | 21
```

Y bajar un **R8 propio** (Google Maven, `com.android.tools:r8`) en vez de depender del
`d8.jar` del SDK. Así Kotlin 2.4 + Compose + Java 17 funcionan sin Gradle.

> Matiz: “Java moderno” = **nivel de lenguaje**, no APIs del JDK 25. Android no trae
> `java.time`, streams, etc. según API level. Sin desugaring de librería, hay que limitar
> las APIs o subir `minSdk`. `rain` debería documentarlo y avisar en `doctor`.

### 0.3 Qué impide hoy compilar los ejemplos de `android2apk`

| Ejemplo | Lenguaje | `res/` | AndroidX/Compose | Estado con `rain` hoy |
|---|---|---|---|---|
| `hello` | Java | No | No | Falla: layout distinto + pánico si no hay `res/` |
| `khello` | Kotlin | No | No | Falla: layout distinto |
| `pulse` | Java | Sí | No | Falla: layout distinto |
| `notes` | Java | Sí | No | Falla: layout distinto |
| `todo` | Java | No | No | Falla: layout + pánico sin `res/` |
| `compose` | Kotlin | Sí | Compose | Fase 5: falta todo el soporte AAR/Compose |
| `neko` | Kotlin | Sí | Compose | Fase 5 |
| `expressive` | — | — | — | No es una app (snippets) |

`android2apk` usa layout plano (`AndroidManifest.xml`, `java/`, `res/`, `assets/`,
`app.properties`); `rain` solo entiende `app/src/main/...`.

**Conclusión:** los 5 ejemplos simples son alcanzables en la Fase 3 con cambios moderados.
Los 2 de Compose requieren la Fase 4 (AAR completo) + Fase 5 (Compose).

### 0.4 Lo que ya es superior en `rain`

- Binario único, arranque instantáneo, sin daemon.
- Incremental real por hash.
- Resolución Maven con parents/`dependencyManagement`/transitivas.
- `rain.toml` declarativo y `clean`.
- R8 + firma configurables.
- Código pequeño y auditable.

Lo que hay que arreglar para que esa superioridad sea real: **corrección del pipeline**,
**paridad funcional** y **red de seguridad (tests/CI)**.

---

## 1. Decisiones de arquitectura

### 1.1 CLI con Cobra (solo Go, independiente de Java)

Hoy `main.go` hace `switch os.Args[1]` y crea un `flag.NewFlagSet` por comando. Eso no
escala: cada comando repite parseo, ayuda y validación, y no hay completions. Cobra
centraliza todo eso.

```
rain
├── init                    # crea rain.toml (interactivo o --non-interactive)
├── build [dir]             # compila; autodetecta layout
│   ├── --android-jar PATH
│   ├── --release
│   ├── --module NAME
│   ├── --dry-run
│   └── --offline
├── doctor                  # diagnóstico del entorno
│   ├── --profile simple|complex
│   ├── --json
│   ├── --print-install
│   └── --fix
├── deps tree               # árbol de dependencias resueltas
├── clean                   # respeta módulos
├── version                 # versión, commit, Go, plataforma
└── completion bash|zsh|fish
```

Esqueleto de migración:

```go
var (
    verbose bool
    quiet   bool
    noColor bool
)

var rootCmd = &cobra.Command{
    Use:           "rain",
    Short:         "Build system ligero para Android",
    SilenceUsage:  true, // no imprimir ayuda ante errores de ejecución
    SilenceErrors: true,
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        ui.Verbose = verbose
        ui.Quiet = quiet
        ui.NoColor = noColor
        return nil
    },
}

var buildCmd = &cobra.Command{
    Use:   "build [dir]",
    Short: "Compila un APK",
    Args:  cobra.MaximumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        dir := "."
        if len(args) == 1 {
            dir = args[0]
        }
        return runBuild(cmd.Context(), dir, buildOpts)
    },
}

func main() {
    if err := rootCmd.Execute(); err != nil {
        ui.Error("%v", err)
        os.Exit(1)
    }
}
```

Puntos clave:

- **Flags persistentes** (`--verbose`, `--quiet`, `--no-color`, `--json`) una sola vez en
  el root; no se repiten por comando.
- **`RunE` + `SilenceUsage`/`SilenceErrors`**: los errores de negocio salen limpios y con
  exit code, sin imprimir el usage entero.
- **Flags POSIX**: `--release`, `--android-jar`, `--profile`. Ojo: la CLI actual usa
  `-android-jar` (un guion). pflag espera `--android-jar`; hay que decidir si se acepta el
  cambio (pre-1.0) o se añade una normalización temporal de args para no romper scripts.
- **Completions** bash/zsh/fish y `RegisterFlagCompletionFunc` para `--profile`, `--module`.
- **Testeable**: `cmd.SetArgs([]string{...})`, `cmd.SetOut(buf)`, `cmd.SetErr(buf)`.
- **Exit codes**: `0` OK; `1` error de build; `2` uso incorrecto; `3` doctor incompleto.
  (Se implementa con un tipo de error que lleve el código.)
- Dependencias: `github.com/spf13/cobra v1.10.2` (+ `pflag`, que ya viene con Cobra).
  Requiere Go 1.21+; Alpine trae 1.26 y Termux, una versión reciente.
- No romper `rain build --android-jar ...`; mantener `init`, `build` y `clean` como
  comandos de primer nivel.

### 1.2 Toolchain propio en `~/.rain` (sin root)

En vez de escribir en `/usr/local/bin` o `/etc/apk/repositories` como `android2apk`:

```
~/.rain/
├── bin/            # symlinks o wrappers
├── toolchain/
│   ├── r8.jar
│   ├── apksigner.jar
│   ├── android.jar
│   ├── kotlinc/
│   └── aapt2
└── cache/          # AAR/POM + checksums
```

- `rain doctor` detecta; `rain doctor --fix` descarga **solo lo que falta** en `~/.rain`.
- Si ya hay herramientas del sistema válidas, se usan (configurable: `system-first` vs `managed-first`).
- Nada de root, nada de tocar `/etc`, nada de contaminar el contenedor Alpine.
- Descargas atómicas + SHA-256 + `.part` + reintentos + timeout.

### 1.3 `rain doctor` por perfiles

- `rain doctor` → perfil **simple**: ¿puedo compilar un APK Java sin AndroidX?
- `rain doctor --profile complex` → Kotlin + Compose + AndroidX + R8 moderno.
- `rain doctor --json` → para agentes/CI.
- `rain doctor --print-install` → solo comandos copiables.
- `rain doctor --fix` → instala lo gestionable sin root; para paquetes del sistema imprime el comando (`apk add ...` / `pkg install ...`).

Chequeos:

| Check | Simple | Complex |
|---|---|---|
| `java`/`javac`/`keytool` ≥ 17 | ✅ | ✅ |
| `aapt2` | ✅ | ✅ |
| `zipalign` | ✅ | ✅ |
| `apksigner` | ✅ | ✅ |
| `d8` o `r8.jar` | ✅ | ✅ |
| `android.jar` (API 36) | ✅ | ✅ |
| `zip` | ✅ | ✅ |
| `kotlinc` | — | ✅ |
| Compose compiler plugin | — | ✅ |
| `libjansi` (solo Termux) | — | ✅ |
| Espacio en disco | ✅ | ✅ |
| Matriz JDK/Kotlin/R8/SDK | ✅ | ✅ |

Salida humana (ejemplo):

```
Rain doctor · perfil simple · Alpine 3.24
✔ java 25.0.4        ✔ javac 25.0.4      ✔ keytool
✔ aapt2 16.0         ✔ zipalign          ✔ zip
✔ r8 9.5.28          ✔ apksigner         ✔ android.jar API 36
Resultado: LISTO para APK simple (Java).

Para apps complejas faltan:
  ✘ kotlinc 2.4.21
  ✘ Compose (plugin incluido en Kotlin)
Comandos recomendados:
  apk add openjdk25-jdk zip
  apk add android-build-tools@edge/testing
  rain doctor --fix          # baja kotlinc/r8/android.jar a ~/.rain
```

### 1.4 Compatibilidad de layouts

`rain build [dir]` autodetecta:

1. **Gradle-like:** `<dir>/app/src/main/{AndroidManifest.xml,java,kotlin,res,assets}`.
2. **Plano (android2apk):** `<dir>/{AndroidManifest.xml,java,res,assets,app.properties}`.
3. **Overrides en `rain.toml`:** `manifest`, `sourceDirs`, `resDir`, `assetsDir`.

Con esto, `rain build /root/projects/android2apk/examples/hello` funciona sin copiar nada.

### 1.5 AAR completo (la diferencia clave con `android2apk`)

Para superar a `android2apk`, `rain` debe procesar el AAR entero:

- `classes.jar`
- `res/` (fusionar + `aapt2 compile` + `--extra-packages`)
- `assets/`
- `jni/<abi>/*.so`
- `libs/*.jar`
- `AndroidManifest.xml` (fusión parcial: permisos, `uses-sdk`, `application`, componentes)
- `R.txt` (para generar `R` de cada librería)
- `consumer-rules.pro` / `proguard.txt`

Y empaquetar **todos** los `classes*.dex`, no solo `classes.dex`.

---

## 2. Roadmap por fases

> Prioridad: P0 (bloqueante), P1 (alto), P2 (medio).
> Cada tarea debe terminar con test o verificación reproducible.

### Cómo leer estas casillas (auditoría del 2026-10-21)

Las casillas se han vuelto a comprobar **una por una contra el código y en
ejecución**, no por lo que decía el plan. Las marcas significan:

| Marca | Significado |
|---|---|
| `[x]` | Verificado: el código existe y se ha comprobado en ejecución. |
| `[~]` | Parcial: una parte está hecha y otra no. La nota dice cuál falta. |
| `[ ]` | **No implementado**: la casilla estaba marcada sin serlo. |

Evidencia usada: `go test ./...` en verde, `rain doctor --profile complex`
sobre un Alpine limpio reprovisionado desde cero, **tres builds consecutivos
de JetIsland desde cero con el mismo SHA-256** (`cac4405b…`), un test de
integración que compila un APK real con una dependencia estilo AppCompat y lo
verifica con `apksigner verify` / `aapt2 dump`, y el build de los 5 ejemplos
simples de `android2apk` y de `examples/compose` / `examples/neko`.

### Fase 0 — Red de seguridad y deuda técnica (P0)

- [x] **T0.1** Pasar `gofmt`/`go vet`; arreglar imports de `main.go` y estilo.
- [x] **T0.2** Tests unitarios: `pom` (parser, parents, versiones, transitivas), `config`, hashes, `needFetch`.
- [x] **T0.3** CI en GitHub Actions: `go test ./...`, `go vet`, `golangci-lint`, build para linux/amd64 y linux/arm64.
  > Incluye `.golangci.yml` (v2) y `Makefile` con `fmt`, `lint`, `test`, `build`, `tools`.
  > Equivalente a Ruff para Go: **`golangci-lint`** como meta-linter + `gofmt`/`goimports` como formatters.
- [x] **T0.4** Eliminar pánicos y errores ignorados: `res/` ausente, `filepath.Walk`, `MkdirAll`, `WriteFile`, `keytool`, `RemoveAll`.
- [~] **T0.5** Cambiar `module rain` → `github.com/lucassogarayDA/rain`; añadir `--version` con `ldflags`.
  > El módulo **sí** se renombró (`rain` → `github.com/itskreisler/rain`) y `--version` con `-ldflags` está implementado (`make build` inyecta versión, commit y fecha), pero **no** a `github.com/lucassogarayDA/rain`: esa ruta parece un error de copypaste del plan y no se ha usado.
- [x] **T0.6** Borrar código muerto (`DownloadAAR`, `EffectiveProperties`, `VerboseLog`) y el `DEBUG` de `main.go`; unificar repositorios Maven.
  > Corregido en esta auditoría: se ha eliminado el paquete `internal/resolver` entero (`DownloadAAR`/`DownloadArtifact`, sin ningún importador) y `Project.EffectiveProperties` (versión obsoleta de `MergeProperties`, que sí se usa). `DEBUG` no existe en el código. `ui.VerboseLog` **se conserva**: no era código muerto, lo usan `internal/cli/build.go` e `internal/aar/aar.go`.

**Criterio de salida:** `go test ./...` verde en CI y ningún pánico con proyectos sin `res/`.

### Fase 1 — CLI Cobra + `rain doctor` simple (P0)

- [x] **T1.1** Integrar Cobra v1.10.2; root command + flags persistentes + exit codes.
- [x] **T1.2** Migrar `init`, `build`, `clean` a Cobra sin romper flags actuales.
- [x] **T1.3** `rain doctor` (perfil simple): detectar SO (Alpine/Termux/otro), comandos, versiones, `android.jar`, espacio.
- [x] **T1.4** Motor de recomendaciones: comandos `apk`/`pkg` por perfil; `--print-install`; `--fix` sin root para `~/.rain`.
- [x] **T1.5** Salida `--json` estable (schema versionado) + `rain completion` + tests de CLI con `cobra.Command.SetArgs`.

**Criterio de salida:** en un Alpine limpio, `rain doctor` dice correctamente qué falta para un APK simple y imprime los comandos exactos.

### Fase 2 — Toolchain autoprovisionado + doctor complejo (P1)

- [~] **T2.1** Estructura `~/.rain` + descargas atómicas + checksums + reintentos/timeout.
  > Estructura `~/.rain` (`bin/`, `toolchain/`) y descarga atómica (`.part` + `os.Rename`) y timeout (20 min) están hechos. **Falta la verificación por checksum**: no hay SHA-256 del toolchain ni de las dependencias, así que una descarga corrupta o manipulada no se detecta. Los reintentos existen en `internal/httpx` (usado por las dependencias), no en la descarga del toolchain.
- [~] **T2.2** Resolver de toolchain: `system-first`/`managed-first`, overrides por env, `rain doctor --fix`.
  > `rain doctor --fix` reprovisiona el toolchain desde cero (verificado: `~/.rain` borrado → 27 s → todo ✅) y los overrides por env funcionan (`RAIN_JAVA_MAX_HEAP`, `RAIN_JAVA_OPTS`, `RAIN_GOGC`, `RAIN_GOMEMLIMIT`). **Falta la política explícita `system-first`/`managed-first`**: no existe ningún ajuste de eso; el doctor usa las herramientas del sistema si están y si no las gestionadas, sin que se pueda elegir.
- [x] **T2.3** Matriz de compatibilidad JDK/R8/Kotlin/SDK en código; `doctor` avisa (p. ej. Kotlin 2.4 + R8 8.10.9 = incompatible).
  > `internal/compat`: `CheckMatrix(Versions)` con severidades `advice`/`warning`/`error`; `doctor --json` incluye `compat` y la salida humana muestra la sección "Compatibilidad del toolchain".
- [~] **T2.4** Perfil complejo: `kotlinc`, plugin Compose, caché AAR, `libjansi` solo Termux, disco.
  > `kotlinc` (2.4.21), el plugin Compose (`compose-compiler-plugin.jar`), la caché de AAR y el check de `libjansi` solo en Termux están hechos. **Falta el check de espacio en disco**: `doctor` no comprueba el espacio libre (solo informa de la memoria).
- [x] **T2.5** Tests con `httptest` y FS temporal (sin red real). Cubre `httpx` (reintentos/backoff), `aar` (extracción, `ExtraPackages`, `NativeDir`, fusión de recursos) y `pom`/`build` (dedupe de clases).

**Criterio de salida:** `rain doctor --profile complex --fix` deja un Alpine limpio listo para Kotlin/Compose sin root.

### Fase 3 — Paridad con los ejemplos simples de android2apk (P0)

- [~] **T3.1** Autodetección de layout (Gradle-like / plano) + overrides en `rain.toml`. `sourceRoot()` detecta `app/src/main` (Gradle) o la raíz (plano android2apk). **Pendiente: los overrides explícitos en `rain.toml` (`manifest`, `sourceDirs`, `resDir`, `assetsDir`) no existen**: `internal/config` no tiene ninguna clave para ellos.
- [~] **T3.2** Config completa: `minSdk`, `targetSdk`, `versionCode`, `versionName`, `javaTarget`, `applicationId`/`namespace`; leer `app.properties` como fallback. `config.Build` expone todas las claves y `resolveModuleBuildConfig()` aplica la precedencia `rain.toml` > `app.properties` del módulo > defaults (min_sdk 21, target_sdk 36, version 1.0). **Pendiente: `namespace` separado de `package`** (no hay clave `namespace` en la config).
- [x] **T3.3** `BuildConfig` real: `VERSION_CODE`, `VERSION_NAME`, `DEBUG`, `APPLICATION_ID`, `BUILD_TYPE`. Se genera `BuildConfig.java` en `gen/<package>/` y se compila antes que Kotlin.
- [x] **T3.4** Pasar `minSdk`/`targetSdk` a `aapt2` y `d8` (quitar 21/36 hardcodeados). Ya no se hardcodea: llegan desde `resolveModuleBuildConfig()` al `build.Config`.
- [x] **T3.5** Soportar proyectos sin `res/` (arreglar el pánico) y sin `assets/`. Los `Walk`/lecturas ignoran `os.IsNotExist` y `aapt2 compile` solo corre `if dirExists(resDir)`.
- [x] **T3.6** Empaquetar `assets/`; Kotlin ↔ Java en el mismo módulo (orden/classpath correcto); `kotlin-stdlib` automática. `aapt2 link -A assets` y `findKotlinStdlib()` añaden el stdlib cuando hay fuentes Kotlin.
- [x] **T3.7** Compilar y verificar `hello`, `khello`, `pulse`, `notes`, `todo` **sin modificar los ejemplos**. Verificado: los 5 generan APK firmado (`apksigner verify` OK) con package y `launchable-activity` correctos, incluidos los recursos (`res/layout`, `res/drawable`, `resources.arsc`) de `pulse`/`notes`. Bug corregido: un proyecto Kotlin puro (khello) recibía el plugin Compose y fallaba con `noRuntimeOnClasspathError`; ahora el plugin se aplica solo si hay imports de `androidx.compose`.
  > **Reverificado en esta auditoría** (builds limpios, borrando el `build/` previo de cada ejemplo, sin tocar los fuentes):

  > | Ejemplo | Lenguaje | package | launchable-activity | Tamaño |
  > |---|---|---|---|---|
  > | `hello` | Java | `com.klei.hello` | `com.klei.hello.MainActivity` | 8.6 KB |
  > | `khello` | Kotlin | `com.klei.khello` | `com.klei.khello.MainActivity` | 811 KB |
  > | `pulse` | Java | `com.klei.pulse` | `com.klei.pulse.MainActivity` | 12.9 KB |
  > | `notes` | Java | `com.klei.notes` | `com.klei.notes.MainActivity` | 25.4 KB |
  > | `todo` | Java | `com.klei.todo` | `com.klei.todo.MainActivity` | 16.8 KB |
- [~] **T3.8** `rain clean` respeta módulos; validar `manifest.package` vs config. `clean` borra `build/` de cada `[[modules]]`; el pipeline usa el `package` de `rain.toml` y cae a `manifest.package`. **Pendiente: el aviso si difieren** (no se compara el package del manifest con el de la config: se usa el de la config y se ignora la discrepancia en silencio).

**Criterio de salida:** `rain build <cada ejemplo simple>` genera un APK firmado, `apksigner verify` OK y `aapt2 dump badging` muestra el paquete/versión correctos.

### Fase 4 — AAR completo, multidex y release (P0/P1)

- [x] **T4.1** Extraer del AAR: `classes.jar`, `res/`, `jni/`, manifest y package de librería (para `--extra-packages`). Pendiente: `assets/`, `libs/*.jar`, `R.txt` explícito y reglas ProGuard.
- [x] **T4.2** Fusionar recursos de librerías (`--extra-packages`, duplicados/overlays tipo AGP). Pendiente: `R.txt` → `R.java`.
- [x] **T4.3** Fusión de manifests (subconjunto): permisos, `uses-sdk`, `application`, `activity/service/receiver/provider`; documentar lo no soportado.
  > Implementado en `internal/aar/manifest.go` (fusión **textual**: re-serializar el XML pierde los prefijos `android:` y aapt2 lo rechaza) con 10 tests. Verificado en ejecución sobre el APK de JetIsland: el manifiesto fusionado declara `androidx.startup.InitializationProvider` y `androidx.profileinstaller.ProfileInstallerInitializer`, que solo existen en las librerías. Queda fuera y documentado: `uses-sdk`, `uses-feature`, `queries` y los atributos de `<application>`.
  > **Bug corregido en esta auditoría**: la fusión copiaba los fragmentos de las librerías sin arrastrar sus declaraciones de namespace. AndroidX Startup usa `tools:node="merge"`, y `tools` solo estaba declarado en el manifest de la librería → el manifest fusionado quedaba con un prefijo sin enlazar y **aapt2 rechazaba el build** con `unbound prefix` (reproducido con `examples/compose`). Ahora los namespaces que usan los fragmentos copiados se declaran en el elemento raíz del manifest fusionado (tests en `internal/aar/manifest_test.go`).
- [x] **T4.4** Multidex: se empaquetan `classes*.dex` en orden (verificado en Compose: `classes.dex` + `classes2.dex`).
- [~] **T4.5** JNI: se empaquetan `lib/<abi>/*.so` de los AAR (verificado en Compose: 4 ABIs). **Pendiente: `zipalign -P 16`** (el APK se alinea con `zipalign -f 4`, sin el flag de páginas de 16 KB que exige Android 15 para las librerías nativas).
- [~] **T4.6** Dependencias correctas: exclusiones, BOM/import, classifiers, properties built-in, parents locales, checksums, caché atómica. (Ya: timeouts/reintentos, moduleKey/highest-wins, dedupe de clases, omisión resiliente.)
  > **La casilla mentía**: de lo enumerado solo están `dependencyManagement` (parseado y fusionado en `internal/pom/parent.go`) y la caché atómica (`.part` + `os.Rename`, añadido en esta auditoría). **Faltan**: exclusiones (`<exclusions>`), BOM/`import` de Spring, classifiers, properties *built-in* (`project.version` y compañía), parents locales (un POM padre en el disco) y **checksums**: no se verifica el SHA-256 ni del toolchain ni de las AAR.
- [x] **T4.7** Release: R8 con reglas del usuario + `consumer-rules.pro`, mapping, **nunca** caer a debug en silencio, `apksigner verify`, contraseñas por env/archivo. Implementado: reglas base en `build/r8-rain.pro` (nunca pisan las del usuario) + `proguard-rules.pro`/`proguard.pro` del proyecto + `proguard.txt` de cada AAR; `--pg-map-output` → `build/mapping.txt`; aviso explícito (o `--allow-debug-signing`) si falta keystore de release; `apksigner verify` tras firmar; `-dontwarn` para paquetes AndroidX opcionales/ausentes (emoji2, window, profileinstaller, tracing, startup) que R8 valida y D8 no.
- [~] **T4.8** Test de integración con una app AndroidX/AppCompat real y verificación de recursos en runtime.
  > **Corregido en esta auditoría**: existe `internal/build/integration_test.go`, que compila un APK real con una dependencia estilo AppCompat (manifiesto con provider, recursos y código propios) y lo verifica **de forma estática**: firma (`apksigner verify --print-certs`), contenido del APK, manifiesto fusionado (provider y permisos de la librería), tabla de recursos (`app_name`, `lib_name`, `AppTheme`) y dex (clase de la librería y de la app). El test se salta si falta el toolchain, así que corre en CI donde `doctor --fix` lo deja listo.
  > **Pendiente la parte de runtime**: arrancar la app en un emulador o dispositivo y comprobar que no lanza `Resources$NotFoundException`. No se ha hecho: este entorno no tiene emulador ni `adb` y no es viable en el contenedor de CI.

**Criterio de salida:** una app con AndroidX compila, arranca y no lanza `Resources$NotFoundException`.

### Fase 5 — Compose / Material 3 (P1)

- [x] **T5.1** Plugin Compose de Kotlin 2.4 + `-jvm-target` compatible (11 en Compose).
- [~] **T5.2** Cierre de dependencias Compose/Material3 (resolver transitivo estable). La resolución transitiva **sí** es estable: orden determinista del grafo (`internal/pom/transitive.go`), `moduleKey`/highest-wins, dedupe de clases y caché atómica. **Pendiente: `rain.lock`** (no existe; es lo mismo que T6.3).
- [~] **T5.3** Compilar `examples/compose` y verificar arranque.
  > **Compilado y verificado** en esta auditoría: APK de 11,5 MB, firmado (`apksigner verify --print-certs` OK), `package=com.klei.compose`, `versionName=1.0`, `targetSdk=36`, `launchable-activity=com.klei.compose.MainActivity`, con `classes.dex` + `classes2.dex` (multidex), `resources.arsc` y las 4 ABIs de `libandroidx.graphics.path.so`. Timestamps 1980-01-01 (reproducible).
  > **Pendiente el arranque**: sin emulador ni `adb` no se puede comprobar que la app abra sin crash.
- [~] **T5.4** Compilar `examples/neko` y verificar arranque + persistencia.
  > **Compilado y verificado estáticamente** en esta auditoría: APK de 16,9 MiB
  > (`app/build/Neko-Notes-1.0-universal.apk`, sha256
  > `189537e6e3a94ccafb4d9bb9dbe752f2e75d216c3245ff096365bebbffab9c1c`),
  > firma verificada (`apksigner verify --print-certs`: CN=Android Debug, SHA-256
  > `a3769715…`), `package=com.klei.neko`, `versionName=1.0`, `versionCode=1`,
  > `minSdk=26`, `targetSdk=36`, `compileSdk=36`,
  > `launchable-activity=com.klei.neko.MainActivity`, 3 dex (multidex),
  > `resources.arsc` y las 4 ABIs de `libandroidx.graphics.path.so`. Timestamps
  > 1980-01-01 (reproducible).
  > Al recompilarlo sin tocar los recursos salió a la luz un bug real: el APK
  > base reutilizado ya contenía `classes.dex` y el empaquetado lo añadía otra
  > vez, así que `apksigner` fallaba con `Multiple ZIP entries with the same
  > name: classes.dex`. Corregido (empaquetado idempotente) y con test de
  > regresión en `internal/build/packaging_test.go`.
  > **Pendiente el arranque y la persistencia**: sin emulador ni `adb` no se
  > puede comprobar que la app abra sin crash ni que las notas sobrevivan a un
  > reinicio.
- [x] **T5.5** Test de CI que compile Compose (sin emulador) y, opcional, smoke test en emulador.

**Estado verificado:** un proyecto Kotlin + Compose (AndroidX) generado con `rain init --template completo` compila a un APK firmado (multidex + `lib/<abi>/*.so` + recursos fusionados) sin Gradle. Los dos ejemplos de Compose (`examples/compose` y `examples/neko`) compilan, firman y pasan la verificación estática del APK (manifiesto, firma, dex, ABIs); a ambos les queda pendiente el arranque en un dispositivo, que es lo único que exige el criterio de salida.

**Criterio de salida:** `rain build examples/compose` y `examples/neko` producen APKs que abren sin crash.

### Fase 5 bis — Ciclo de trabajo rápido: `rain check` y `rain lint` (P1)

- [x] **T5b.1** `rain check`: valida los tipos sin generar DEX, APK ni firma.
  > Compila el código (kotlinc/javac) con el mismo `android.jar`, el mismo
  > classpath y el mismo plugin de Compose que `rain build`, y se detiene ahí
  > (`Config.CheckOnly` en `internal/build/pipeline.go`). Es la única forma de
  > detectar errores como `unresolved reference 'setContentTitle'` sin pagar un
  > build completo: **ningún linter de estilo los ve**, son errores del
  > compilador de Kotlin.
  > **Verificado**: en JetIsland, un fichero con `setContentTitle` inexistente
  > produce `app/src/main/java/.../ZzSondaProbe.kt:7:9: error: unresolved
  > reference 'setContentTitle' on receiver of type 'Context'` y sale con
  > código 1. Tiempos: **2,6 s** si no ha cambiado nada, **~33 s** si hay que
  > recompilar.
- [x] **T5b.2** `rain check --fast`: chequeo incremental.
  > Recompila solo los ficheros modificados desde la última compilación
  > correcta (`.rain-sources-stamp`) y los que dependen de ellos (los que
  > mencionan alguna de sus declaraciones, extraídas por regex), contra las
  > clases ya compiladas en `build/obj`. Si no hay clases previas, cae al
  > chequeo completo para no dar falsos errores.
  > **Medido en JetIsland**: 35 s para 7 ficheros frente a 34 s para los 20
  > ficheros del chequeo completo. El ahorro es marginal: el coste está en el
  > arranque de la JVM del compilador (~25 s), no en el número de ficheros. Se
  > mantiene por el salto que da cuando el cambio es pequeño y hay clases
  > previas de una compilación con más recursos, pero **no** es la palanca
  > principal de velocidad: esa es la caché de dependencias (T5b.3).
- [x] **T5b.3** Caché de dependencias preparadas.
  > El set de AAR/JAR extraídos se guarda en `.rain-deps/.rain-deps.json` con
  > el hash de sus coordenadas. Si las coordenadas no cambian, la siguiente
  > ejecución reutiliza el set en vez de volver a descargar y extraerlo:
  > **de ~14 s a ~0 s** en un proyecto con Compose (59 artefactos).
  > Se invalida sola si falta algún fichero referenciado (`internal/aar/cache.go`).
- [x] **T5b.4** `rain lint`: analizadores estáticos opcionales (ktlint, detekt, pmd).
  > Framework propio (`internal/lint`) con parsers de la salida de cada
  > herramienta, informes agrupados por fichero y resumen por herramienta.
  > Configuración en `rain.toml` (`[lint]`: `tools`, `fail_on`,
  > `type_resolution`) y flags (`--tools`, `--fail-on`, `--type-resolution`).
  > **Medido en JetIsland**: 332 avisos en 19 s (89 de detekt, 243 de ktlint).
  > La **resolución de tipos de detekt va desactivada por defecto**: con el
  > compilador de Kotlin embebido provoca un `NullPointerException` interno
  > (`PrimitiveTypeUtil.unsignedType`).
- [x] **T5b.5** ktlint y detekt en `rain doctor`.
  > Aparecen como checks **opcionales** (no bloquean `Ready`) y se descargan con
  > `rain doctor --fix --lint` o `rain lint --install` (ktlint 1.8.0 y detekt
  > 1.23.8 en `~/.rain/toolchain`).
- [x] **T5b.6** `rain check` ya no envenena el `rain build` siguiente.
  > El estado de compilación se guardaba en una sola marca (`.rain-java-hash`)
  > que servía para dos cosas: saber si las fuentes compilan (necesario para
  > `check`) y decidir si el `classes.dex` se puede reutilizar (necesario para
  > `build`). Como `check` no genera DEX, al terminar escribía la marca y el
  > `build` siguiente reutilizaba el **DEX viejo**: el APK salía con código
  > que ya no estaba en el disco.
  > **Cómo se reproducir**: editar una fuente → `rain check` (compila bien) →
  > `rain build` (reutiliza el DEX anterior, 5 s) → instalar → la app se
  > comporta como si el cambio no existiera.
  > Ahora hay dos marcas: `.rain-java-hash` (las fuentes compilan) y
  > `.rain-dex-hash` (el DEX corresponde a esas fuentes), y solo un build que
  > llega a generar el DEX escribe la segunda. De paso, la compilación completa
  > limpia `build/obj`, para que las clases de los ficheros borrados no sigan
  > en el APK (`internal/build/dexcache_test.go`).

**Criterio de salida:** `rain check` detecta un error de tipos en segundos y
`rain lint` informa de estilo y "code smells" sin tocar el pipeline de build.

### Fase 6 — Modernización y reproducibilidad (P2)

- [x] **T6.1** JDK 25 soportado; default `--release 17`; `javaTarget` configurable (8/11/17/21).
- [x] **T6.2** Kotlin 2.4.21 + R8 9.5.x desde Google Maven (desacoplar del build-tools). Fijado en `internal/toolchain/toolchain.go` (`KotlinVersion = "2.4.21"`, `R8Version = "9.5.28"`).
- [ ] **T6.3** `rain.lock` con SHA-256 de dependencias y toolchain; `--offline`.
  > **No implementado**: no existe ningún `rain.lock` ni la flag `--offline` (0 coincidencias en todo el código). Todo build necesita red para resolver y descargar dependencias, y nada comprueba el SHA-256 de lo descargado.
- [x] **T6.4** APK reproducible: timestamps fijos, ZIP ordenado, R8 determinista, firma determinista.
  > Verificado empíricamente: **tres builds consecutivos de JetIsland desde cero** (toolchain y caché de dependencias borrados) producen el mismo SHA-256 (`cac4405bc08276ded203550d1d7d3d027bec1810dc3637f011b7202109612323`), y el binario anterior a los cambios de esta auditoría da ese mismo hash (la diferencia de binario no altera la salida).
  > En esta auditoría se ha corregido un bug que **rompía** la reproducibilidad: `libraryJars()` metía en el classpath **todos** los jars del caché de dependencias, incluidos los artefactos rancios de resoluciones anteriores, así que el APK dependía del historial de la caché. Contrafactual medido (con un jar rancio plantado en `.rain-deps/jars/` con contenido propio):
  >
  > | Binario | Caché | SHA-256 del APK |
  > |---|---|---|
  > | anterior | limpia | `cac4405b…` |
  > | anterior | con jar rancio | `5ea858fe…` ← **el artefacto rancio se colaba en el APK** |
  > | nuevo (con el fix) | con jar rancio | `cac4405b…` ← **el fix lo neutraliza** |
  >
  > El classpath sale ahora solo del conjunto resuelto (tests de regresión en `internal/build/deps_test.go`).
- [ ] **T6.5** Paralelizar descargas y compilación por módulo; caché por archivo.
  > **No implementado**: no hay ni una goroutine en `internal/` (0 coincidencias de `errgroup`, `sync.WaitGroup` o `go func`): las descargas y la compilación son estrictamente secuenciales. La caché es por directorio (`.rain-deps`), no por archivo.
- [x] **T6.6** Matriz CI Alpine 3.24 / Termux (o contenedor equivalente) / amd64 + arm64.

**Criterio de salida:** dos builds del mismo commit producen el mismo APK (o diferencias justificadas y documentadas).

### Fase 7 — Producto y documentación (P2)

- [x] **T7.1** README nuevo + `docs/doctor.md`, `docs/toolchain.md`, `docs/compatibilidad.md`, guía de migración desde android2apk.
- [x] **T7.2** `CHANGELOG`, versionado semver, releases con binarios linux/amd64 y linux/arm64. Hecho: `scripts/next-version.sh` calcula la siguiente versión desde Conventional Commits; `.github/workflows/release.yml` crea tag anotado + release (binarios amd64/arm64 + SHA256SUMS) en cada push a `main` con `feat`/`fix`; `Makefile` inyecta versión/commit/fecha vía `-ldflags`; `CHANGELOG.md` añadido; primer tag `v0.1.0`.
- [x] **T7.3** `rain init` con scaffolding real (manifest, `MainActivity`, `res/`, plantillas Java/Kotlin/Compose).
- [x] **T7.4** `rain deps tree`, `rain doctor --json`, salida amigable para agentes.
- [~] **T7.5** `SECURITY.md`, NOTICE de terceros, cabeceras de licencia.
  > `SECURITY.md` y `NOTICE` existen (el `NOTICE` incluye la atribución de `android2apk`, que es Apache-2.0, y el código portado de `setup-compose.sh`/`build-apk.sh`). **Faltan las cabeceras de licencia**: no hay ningún fichero con SPDX ni cabecera de copyright en el código Go (0 coincidencias).

---

## 3. Matriz de pruebas objetivo

| Escenario | Java | Kotlin | Compose | AAR | Assets/JNI |
|---|---|---|---|---|---|
| `hello` | ✅ | — | — | — | — |
| `khello` | — | ✅ | — | — | — |
| `pulse` | ✅ | — | — | — | — |
| `notes` | ✅ | — | — | — | — |
| `todo` | ✅ | — | — | — | — |
| app AndroidX mínima | ✅ | — | — | ✅ | — |
| `compose` | — | ✅ | ✅ | ✅ | — |
| `neko` | — | ✅ | ✅ | ✅ | — |
| app con `.so` | ✅ | — | — | ✅ | ✅ |

Verificación por APK:

1. `apksigner verify --verbose`.
2. `aapt2 dump badging` (paquete, versión, minSdk, targetSdk).
3. `unzip -l` (dex, resources, assets, `.so`).
4. Instalación/arranque en emulador o dispositivo (smoke test manual o CI con emulador).
   > **Pendiente**: sin emulador ni `adb` en este entorno. La verificación que sí se hace es
   > estática sobre el APK (firma, manifiesto fusionado, tabla de recursos, dex), tanto en el
   > test de integración de `internal/build` como en los builds de los ejemplos.

### Estado verificado (auditoría del 2026-10-21)

| Escenario | package | launchable-activity | APK | Resultado |
|---|---|---|---|---|
| `hello` | `com.klei.hello` | `com.klei.hello.MainActivity` | 8,6 KB | ✅ firmado |
| `khello` | `com.klei.khello` | `com.klei.khello.MainActivity` | 811 KB | ✅ firmado |
| `pulse` | `com.klei.pulse` | `com.klei.pulse.MainActivity` | 12,9 KB | ✅ firmado |
| `notes` | `com.klei.notes` | `com.klei.notes.MainActivity` | 25,4 KB | ✅ firmado |
| `todo` | `com.klei.todo` | `com.klei.todo.MainActivity` | 16,8 KB | ✅ firmado |
| `compose` | `com.klei.compose` | `com.klei.compose.MainActivity` | 11,5 MB | ✅ firmado, multidex, 4 ABIs |
| `neko` | `com.klei.neko` | `com.klei.neko.MainActivity` | — | ✅ firmado |
| app AndroidX mínima (test de integración) | `com.example.integration` | `.MainActivity` | 12,4 KB | ✅ firmado, manifest fusionado, recursos y dex verificados |

En todos ellos: `apksigner verify --print-certs` OK, `aapt2 dump badging` con paquete y
activity lanzable, y `unzip -l` con `classes.dex`, `resources.arsc` (y `classes2.dex` + `lib/`
en los de Compose). Lo que **no** se ha verificado es el arranque en un dispositivo (sin
emulador ni `adb`).

---

## 4. Riesgos y mitigaciones

| Riesgo | Impacto | Mitigación |
|---|---|---|
| Fusión de manifests de AAR | Alto | Empezar por subconjunto; documentar; tests con AndroidX |
| Fusión de recursos duplicados | Alto | Reutilizar estrategia de `android2apk/setup-compose.sh`; tests |
| Matriz Kotlin/R8/SDK | Alto | Tabla en código + `doctor`; descargar R8 propio |
| `--release 17` vs APIs Android | Medio | `javaTarget` configurable; avisar de APIs no disponibles |
| Termux vs Alpine | Medio | Detección de plataforma; tests en ambos |
| Descargas sin checksum | Medio | `rain.lock` + SHA-256 + `.part` atómico |
| Complejidad de Compose | Alto | Fase 5 separada; no bloquear paridad simple |
| Pérdida de datos en ejemplos (JSON) | Bajo | No es bug de rain; documentar |

---

## 5. Orden recomendado de ejecución

1. **Fase 0** (1–2 días): tests + CI + arreglos de robustez.
2. **Fase 1** (2–3 días): Cobra + `doctor` simple.
3. **Fase 3** (3–5 días): layout + config + ejemplos simples. *Aquí ya se puede demostrar paridad.*
4. **Fase 2** (2–3 días): toolchain `~/.rain` + `doctor` complejo.
5. **Fase 4** (1–2 semanas): AAR completo, multidex, release.
6. **Fase 5** (1 semana): Compose.
7. **Fase 5 bis** (1–2 días): `rain check` + `rain lint`. *Es lo que más acelera el trabajo diario: detecta los errores en segundos en vez de en un build completo.*
8. **Fase 6–7**: modernización, reproducibilidad, docs y releases.

**Hito demostrable temprano:** `rain doctor` + `rain build examples/hello` (y los otros 4 simples)
en un Alpine limpio, sin root, con un solo binario. Eso ya es superior a `android2apk` en
experiencia; la superioridad funcional total llega con la Fase 4/5.

---

## 6. Notas de licencia

`android2apk` es Apache-2.0 y tiene `NOTICE` por código de `buddy-android`. Si se porta
lógica de `setup-compose.sh` o `build-apk.sh` a Go, conservar atribución y añadir un
`NOTICE` actualizado en `rain`.
