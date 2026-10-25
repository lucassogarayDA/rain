# AGENTS.md

Rain es un binario Go que compila y firma APKs Android sin Gradle. Los comentarios, la salida del CLI, los tests y la documentación están en español; el `README.md` está en inglés. Mantén esa separación.

## Core Principles

Always make decisions that improve all four areas without breaking existing functionality.

1. **UI (User Interface):** Maintain a consistent, clear, and polished interface.
2. **UX (User Experience):** Optimize usability, navigation, feedback, and interaction flows.
3. **DX (Developer Experience):** Keep code maintainable, APIs predictable, and development workflows simple.
4. **AX (Agent Experience):** Keep project structure, instructions, and code conventions clear for AI agents.

## Strict Rules

- Prioritize improvements that benefit UI, UX, DX, and AX simultaneously.
- Preserve existing functionality, public APIs, and established conventions.
- Avoid unnecessary refactoring, dependencies, and architectural changes.
- Follow existing patterns before introducing new patterns.
- Keep implementations consistent with the project's architecture.
- Make interfaces predictable and interactions accessible.
- Write clear code that developers and AI agents can understand.
- Update relevant documentation when behavior, APIs, or workflows change.
- Validate changes with appropriate tests, builds, and static analysis.
- Never claim success without verifying the result.
- Identify potential regressions before modifying existing behavior.
- Prefer the smallest complete change that achieves the objective.
- If a requirement conflicts with existing behavior, investigate before proceeding.
- Never sacrifice one experience unnecessarily to improve another.
- When trade-offs are unavoidable, explain the impact and choose the safest solution.

## Decision Rule

Before implementing any change, evaluate its impact on UI, UX, DX, and AX.

Reject changes that introduce avoidable regressions, unnecessary complexity, or usability problems.

Prioritize solutions that improve the overall product while preserving stability.

## Comandos

- `make build` genera `./rain` con versión, commit y fecha. `go build ./...` solo comprueba que compila.
- `./rain` es un binario ignorado por git: si tocas Go, reconstruye con `make build` antes de probarlo.
- `make test` equivale a `go test ./...`. Un test concreto: `go test ./internal/build -run TestIntegracionAPKConLibreriaAndroidX -v`.
- Antes de subir: `go vet ./... && go test ./... && make lint`. El lint usa golangci-lint v2.5.0; `make tools` lo instala si falta.
- `make fmt` usa `gofmt`; el lint también exige `goimports`.
- Usa `rg` en vez de `grep`: es más rápido y el `grep` de BusyBox no soporta opciones habituales como `--include`.
- Siempre usa `./rain --devlog build` para builds de APK (también en ejemplos y pruebas manuales), no solo al depurar.
- Verifica un APK con `java -jar ~/.rain/toolchain/apksigner.jar verify --verbose <apk>`; es lo que hace CI.

## Entorno y toolchain

- JDK 17 recomendado: JDK 21 puede fallar bajo PRoot. `aapt2`, `zipalign` y `zip` son herramientas del sistema.
- `./rain doctor --fix --profile complex` descarga R8, apksigner, `android.jar` y Kotlin/Compose en `~/.rain`; no instala `aapt2` ni `zipalign`.
- El perfil por defecto de doctor es `simple` y **no** instala Kotlin. Para Kotlin/Compose usa siempre `--profile complex`.
- No ejecutes `rain build` en la raíz del repo: su `rain.toml` apunta a `app/`, que no existe. Usa `examples/compose`, `examples/liquid-glass`, `examples/backdrop-music` o `examples/hello-buddy`.
- Variables útiles: `RAIN_ANDROID_JAR` fuerza `android.jar`; `RAIN_JAVA_MAX_HEAP` limita la JVM de d8/kotlinc; `RAIN_GOMEMLIMIT` y `RAIN_GOGC` ajustan el runtime de Go sin pisar `GOMEMLIMIT`/`GOGC` ya definidos.

## Mapa del código

- `main.go` → `internal/cli` (Cobra). `internal/cli/build.go` elige el layout (`rain.toml` o plano), resuelve dependencias y lanza el pipeline.
- `internal/build/pipeline.go` es el pipeline real: recursos, Java/Kotlin, DEX/R8, empaquetado, alineado y firma. Las regresiones de build están en `internal/build/*_test.go`.
- `internal/pom` + `internal/mavenrepo` resuelven y descargan POMs; `internal/aar` prepara AAR/JAR; `internal/toolchain` gestiona `~/.rain`; `internal/gradle` importa proyectos Gradle; `internal/lint`, `internal/hilt` (kapt) y `internal/memenv` son auxiliares.
- `rain.toml` se parsea en estricto: una clave desconocida falla y las claves raíz (`mode`, `dependencies`, `modules`) deben ir antes de cualquier tabla. `app.properties` es fallback y `rain.toml` tiene prioridad.
- No confundas el vocabulario: plantilla `simple|completo` (`rain init`), perfil `simple|complex` (`rain doctor`) y `mode = "simple"|"completo"`.
- `[build] debug` se parsea pero no se usa; la variante release se activa con `--release`.
- Sin `rain.toml`, `rain build` y `rain check` detectan el layout plano si hay `AndroidManifest.xml` en la raíz o en `src/main/`.

## Convenciones de código y producto

- En `internal/cli`, toda función nueva no-test debe empezar con `devlog("nombre", fmt.Sprintf(...), "descripción")`. Al tocar el paquete, audita que todas las funciones no-test tengan su `devlog`; `devlog` es local a `cli` y no existe en los demás paquetes.
- Al crear o modificar una app o un ejemplo, cuida la UI/UX desde el primer commit: jerarquía visual, estados vacío/carga/error, tamaños táctiles, contraste/tema y comportamiento en pantallas pequeñas. No entregues UI de relleno.

## Estado y cachés

- Ignorados por git: `libs/` (jars locales), `cache/` (POMs y `.rain-resolved-*.json`), `.rain-deps/` (AAR/JAR preparados y `.rain-deps.json`), `**/build/` (salvo `internal/build/`), `rain`, `*.apk` y `*.keystore`.
- Salidas estables que usa CI: `app/build/app-debug.apk` y `app/build/app-release.apk`. También se genera `<nombre>-<versión>-<abi>[-release].apk`.
- En debug el DEX va partido: la app ocupa `classes.dex` y las dependencias `classes2.dex`, `classes3.dex`…, cacheadas en `build/dexdeps/` con `.rain-dex-deps-hash`. En release hay una sola pasada de R8.
- El Java generado se cachea en `build/genclasses/` (`.rain-gen-hash`). Las marcas `.rain-java-hash` (las fuentes compilan) y `.rain-dex-hash` (el DEX corresponde a esas fuentes) son distintas: `rain check` solo escribe la primera; no las mezcles.
- Si cambias la lógica de resolución, sube `resolvedCacheVersion` en `internal/cli/resolvecache.go`; si cambias el DEX de dependencias, `dexDepsCacheVersion` en `internal/build/pipeline.go`; si cambian las entradas del Java generado, el prefijo `rain-gen-v1`.
- `rain clean` borra `libs/`, `cache/` y `app/build` (o los `build/` de los módulos), pero deja `.rain-deps/`. En layout plano no borra el `build/` de la raíz.
- No edites a mano artefactos de `build/`, `cache/` ni `.rain-deps/`; regenéralos con Rain.

## Tests, CI y release

- `go test ./...` no usa emulador ni adb. El test de integración construye y firma un APK real, pero se salta si falta `~/.rain/toolchain/android.jar` o alguna herramienta (`aapt2`, `javac`, `keytool`, `zipalign`, `d8`, `apksigner`). Un `go test` verde puede no haber ejercitado el pipeline: para cambios de build, compila un ejemplo.
- El test caro de Compose es opt-in: `RAIN_COMPOSE_IT=1 go test ./internal/build -run TestEjemploCompose -v`. Requiere toolchain y `examples/compose/.rain-deps` generado por un build previo; tarda minutos y modifica `MainScreen.kt`, que restaura al terminar. Si se interrumpe, deja el texto `Text("Presióname")`.
- CI (`ci.yml`): test → build de `examples/compose`, `examples/liquid-glass` y `examples/hello-buddy` → release. `examples/backdrop-music` se valida con `rain check` porque su dependencia KMP (`io.github.kyant0:backdrop`) es la que cubre la regresión de alias de Compose Multiplatform. El release se publica en cada push a `main` salvo que el **último** commit contenga `[skip release]`.
- La versión sale de Conventional Commits con `scripts/next-version.sh`: `feat` sube minor; `fix`/`perf`/etc. y cualquier commit sin prefijo suben patch. Un push a `main` puede crear tag y release.
- Si el commit no modifica código Go (`README`, `AGENTS.md`, docs, imágenes…), incluye `[skip release]` en el mensaje. El workflow solo mira el último commit del push, así que ese commit debe llevarlo.
- No commitees el binario `rain`, APKs ni directorios de caché generados. Mantén `CHANGELOG.md` ([No publicado]) al día para cambios visibles, en español.
