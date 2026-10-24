# Migrar de android2apk a Rain

`android2apk` es una colección de scripts de Bash para compilar APKs en Alpine
sin Gradle. `rain` es el mismo pipeline reescrito en Go: un binario, sin
`setup-*.sh`, sin daemon y con resolución de dependencias estilo Gradle.

Esta guía explica qué cambia, qué se conserva y cómo portar un proyecto que
hoy compila con `android2apk`.

## Resumen

| | `android2apk` | `rain` |
|---|---|---|
| Forma | Scripts Bash + `docs/` + skill de agente | Un binario Go + `rain.toml` |
| Configuración por proyecto | `app.properties` (clave=valor) | `rain.toml` (TOML) |
| Dependencias | `compose-deps.txt` + `setup-compose.sh` | `dependencies = [...]` en `rain.toml`, con transitivas |
| Toolchain | `~/.a2a/` (~400 MB, instalada a mano) | `~/.rain/`, se autoprovisiona con `rain doctor --fix` |
| Cache | `~/.a2a/cache` | `<proyecto>/cache` (POMs) + `<proyecto>/libs` (AAR/JAR) |
| Salida | `build/app.apk` + `<nombre>-<versión>-<abi>.apk` | `build/app-debug.apk` + `<nombre>-<versión>-<abi>.apk` |
| Compose | Script aparte (`build-compose.sh`) | Integrado: `mode = "completo"` |
| Firmado | Siempre, con keystore de debug | Igual, y avisa si un release usa la de debug |

## Los dos layouts que Rain entiende

Rain detecta solo el layout, sin tocar los ficheros del proyecto.

**Layout plano (el de `android2apk`)** — tal cual, sin modificar:

```
mi-app/
├── AndroidManifest.xml
├── app.properties
├── java/            # .java y .kt
└── res/
```

```bash
rain build /ruta/a/mi-app
```

**Layout Gradle**:

```
mi-app/
├── rain.toml
└── app/src/main/{AndroidManifest.xml,java,res,assets}
```

```bash
cd mi-app && rain build
```

## Migrar `app.properties` a `rain.toml`

`app.properties` sigue funcionando como *fallback* (Rain lo lee si no hay
`rain.toml`), pero lo interesante es declararlo en TOML:

```toml
mode = "completo"

dependencies = [
    "androidx.core:core-ktx:1.19.1",
    "androidx.activity:activity-compose:1.13.0",
    "androidx.compose.ui:ui:1.12.1",
    "androidx.compose.material:material:1.12.1",
]

[project]
name = "MiApp"
package = "com.ejemplo.miapp"
version = "1.0"

[build]
min_sdk = 26
target_sdk = 36
java_target = 17
```

Equivalencias:

| `app.properties` | `rain.toml` |
|---|---|
| `name=MiApp` | `[project] name` |
| `versionName=1.0` | `[project] version` o `[build] version_name` |
| `versionCode=1` | `[build] version_code` |
| `minSdk=26` | `[build] min_sdk` |
| `package=com.x` | `[project] package` |
| `compose=true` | `mode = "completo"` + `dependencies` |
| (nuevo) | `[build] target_sdk`, `java_target`, `application_id` |

> Ojo con el TOML: las claves raíz (`mode`, `dependencies`) van **antes** de
> cualquier cabecera `[tabla]`. Si no, TOML las mete dentro de la tabla
> anterior y Rain lo avisa con un error explícito.

## Migrar las dependencias de Compose

`android2apk` usa `compose-deps.txt` (una lista fija de AAR de AndroidX) y
`setup-compose.sh` para bajarlas y fusionarlas a mano. Rain resuelve el
grafo completo desde los POMs de Google Maven:

```bash
rain deps tree            # ver el grafo resuelto
rain deps tree --json     # salida para agentes
```

El resultado es que basta con declarar las dependencias **directas**; Rain
descarga las transitivas (`activity-compose` → `activity`, `core`, `lifecycle`…).

## Toolchain

En vez de `setup-toolchain.sh` + `setup-kotlin.sh` + `setup-compose.sh`:

```bash
rain doctor --fix          # descarga R8, apksigner, android.jar y Kotlin en ~/.rain
rain doctor --profile complex
```

Rain no necesita root y no toca `/etc` ni el sistema: todo vive en `~/.rain`.

## Firmado

Igual que en `android2apk`, sin keystore se usa la de debug. La diferencia es
que Rain **avisa** si compilas un release con la keystore de debug:

```bash
rain build --release --allow-debug-signing
```

Para release de verdad, declara tu keystore:

```toml
[signing]
store_file = "mi-release.keystore"
store_password = "..."
key_alias = "mi_alias"
key_password = "..."      # opcional: si se omite, se usa store_password
```

## Chequeo rápido de un proyecto portado

```bash
rain build /ruta/a/mi-app
apksigner verify --verbose /ruta/a/mi-app/build/app-debug.apk
aapt2 dump badging /ruta/a/mi-app/build/app-debug.apk | head -3
```

Si `aapt2 dump badging` muestra el package y la versión correctos y
`apksigner verify` dice `Verified`, el port está hecho.

## Lo que Rain todavía no hace

- No hay `rain.lock` todavía: el grafo se recalcula en cada build (usa la
  caché de POMs, así que no vuelve a la red salvo que cambie una versión).
- No hay `--offline`.
- La fusión de manifests de las AAR es un subconjunto: los permisos y los
  componentes de las librerías no se mezclan con los de la app.
