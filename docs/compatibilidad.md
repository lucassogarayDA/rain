# Compatibilidad del toolchain

`rain doctor` aplica una **matriz de compatibilidad** entre las piezas del
toolchain (JDK, R8, Kotlin, build-tools/SDK) para avisarte antes de que te
choques con un error críptico de R8 o Kotlin. La misma información se expone en
`rain doctor --json` bajo la clave `compat`.

## Reglas y por qué existen

| Tema | Regla | Severidad | Motivo |
|------|-------|-----------|--------|
| JDK | `>= 17` | error si es menor | `r8`/`apksigner` y la metadata de Kotlin 2.x necesitan Java 17+ |
| R8 + Kotlin | R8 `>= 8.3` con Kotlin `>= 2.0` | error si no | R8 &lt; 8.3 no entiende la metadata de Kotlin 2.x |
| R8 + Kotlin | R8 moderno + Kotlin 2.x | advice | combinación soportada y recomendada |
| compileSdk | `>= 33` (ideal 34+) | warning | las librerías AndroidX/Compose actuales piden SDK reciente |
| build-tools | `>= compileSdk - 1` | warning | evita desajustes entre `aapt2`/`zipalign` y la plataforma |

Los mínimos viven en código (`internal/compat`) para que doctor y el `--json`
nunca se desincronicen de lo que realmente comprueba el build.

## Bytecode objetivo

Rain compila a un bytecode conservador para maximizar compatibilidad con el
dispositivo:

- **Java**: `javac --release 8`.
- **Kotlin**: `-jvm-target 1.8`.
- **Kotlin + Compose**: `-jvm-target 11` (el compilador de Compose no acepta
  objetivos inferiores; Kotlin 2.x ya no acepta `8` en ese contexto).

Por eso el JDK solo necesita ser **17+**, no necesitas el JDK exactamente
alineado con el `targetSdk`.

## Perfiles de doctor

`rain doctor` tiene dos perfiles:

- **simple** (por defecto): comprueba lo mínimo para un APK Java sin
  dependencias — JDK, `android.jar`, `aapt2`, `zipalign`, `d8`, `apksigner`,
  espacio en disco y permisos de escritura en `~/.rain`.
- **completo** (`rain doctor --profile complex`): añade `kotlinc`, plugin de
  Compose, caché de AAR/POM y espacio para el cierre de AndroidX/Compose.

```sh
rain doctor                     # perfil simple
rain doctor --profile complex   # perfil complejo (Kotlin/Compose)
rain doctor --json              # salida estable para agentes/CI
rain doctor --fix               # descarga lo que falte en ~/.rain (sin root)
rain doctor --print-install     # imprime los comandos de instalación sugeridos
```

Códigos de salida relevantes:

- `0` — todo listo.
- `3` — falta algo para compilar (doctor sugiere cómo arreglarlo).

## Matriz de versiones probada

Rain se prueba sobre estas combinaciones:

- **Alpine (contenedor)**, linux/arm64 y linux/amd64.
- **Termux**, ARMv7/arm64.
- JDK **17** (recomendado en PRoot/Termux; el kernel PRoot rompe OpenJDK 21).
- Kotlin **2.4.21** + R8 **9.5.x** desde Google Maven.
- `android.jar` de la plataforma **36**.

Kotlin y R8 se provisionan vía `rain doctor --fix`, lo que desacopla el build
del `build-tools` del sistema y evita las combinaciones rotas.

## Release (R8) y firma

`rain build --release` minifica con R8 y firma el APK. Comportamiento:

- **Reglas**: se aplican, en este orden, las reglas base de Rain
  (`build/r8-rain.pro`, generadas y nunca sobrescriben las tuyas), tu
  `proguard-rules.pro`/`proguard.pro` del proyecto y los `proguard.txt`
  (consumer-rules) que traen los AAR. Las reglas base mantienen los puntos de
  entrada que el framework instancia por reflexión (Activity, Service,
  BroadcastReceiver, ContentProvider, Views infladas) y la metadata de Kotlin.
- **Clases ausentes**: R8 valida las referencias del programa (a diferencia de
  d8). Las dependencias de AndroidX referencian clases de artefactos opcionales
  o de versiones nuevas de la plataforma (emoji2, window, profileinstaller,
  tracing, startup) y anotaciones de JetBrains. Rain emite los `-dontwarn`
  equivalentes a los que genera AGP y añade `kotlin-stdlib`/`annotations` del
  toolchain como librerías de R8, para que un release con AndroidX/Compose no
  aborte con `Missing class`.
- **`mapping.txt`**: se escribe en `build/mapping.txt` para desofuscar crashes.
- **Firma**: si defines `[signing]` se usa tu keystore (se valida que exista y
  que `key_alias` no esté vacío). Si no, un release se firma con la keystore de
  **debug** y Rain te lo **avisa**; confírmalo con `--allow-debug-signing`.
  Rain **nunca** cae a debug en silencio.
- **Verificación**: tras firmar, `apksigner verify` comprueba el APK; si falla,
  el build termina con error en vez de entregar un APK inválido.

Tamaños medidos (debug → release):

| Proyecto | Debug | Release |
| --- | --- | --- |
| `hello` (Java) | 12,7 KiB | 8,6 KiB |
| `notes` (Java + recursos) | 40 KiB | 21 KiB |
| Compose (`rain init --template completo`) | 11,3 MiB | 946 KiB |

## Modos de resolución de dependencias

- **completo** (`mode = "completo"`): Rain lee los POMs de Google Maven y Maven
  Central y resuelve las dependencias transitivas (como Gradle). Es el modo
  necesario para AndroidX/Compose.
- **simple** (`mode = "simple"`): solo se descargan las coordenadas que declaran
  `dependencies`; no se siguen transitivas. Útil para control total o proyectos
  pequeños.

En ambos modos, si una coordenada resuelve a un **JAR de Kotlin Multiplatform**
(solo metadata, sin clases `.class` — p. ej. `androidx.annotation:annotation`),
Rain descarga automáticamente la variante `-jvm` hermana (equivalente a lo que
Gradle hace con Gradle Module Metadata). Así `androidx.annotation:annotation:1.6.0`
funciona igual en modo simple que en completo.
