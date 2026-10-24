# Rain

A lightweight Android build system written in Go, designed to run on Termux without the overhead of Gradle.

## Overview

Rain is a minimalist alternative to Gradle for building Android APKs directly on mobile devices or Linux environments. It executes the essential Android build pipeline (`aapt2` → `javac`/`kotlinc` → `d8`/`R8` → `zipalign` → `apksigner`) with minimal dependencies, no persistent JVM daemon, and efficient dependency resolution.

## Features

- **Lightweight** - Single Go binary with instant startup and low memory footprint
- **Dependency resolution** - Reads Maven POMs, handles parent inheritance and `dependencyManagement` (full mode)
- **Mixed language support** - Compile Java and Kotlin in the same project
- **Incremental builds** - SHA-256 hashing of sources and resources skips unnecessary recompilation
- **Fast type checking** - `rain check` validates types without generating an APK
- **Static analysis** - `rain lint` integrates ktlint and detekt
- **Release builds** - Minify with R8 and sign with your keystore
- **Multi-module support** - Define multiple modules in `rain.toml`
- **Android2apk compatible** - Drop-in replacement for projects using the `android2apk` flat layout

## Requirements

- **OS**: Termux or Linux with glibc/musl (tested on Alpine in containers)
- **JDK**: Java 17 or higher (JDK 21 works in most environments, but may be unstable under PRoot; JDK 17 is recommended)
- **aapt2**: Available from system packages (`pkg install aapt2` on Termux)
- **Toolchain**: Rain auto-provisions `d8`/R8, `apksigner`, `zipalign`, `android.jar`, Kotlinc, and Compose plugin to `~/.rain/`

## Installation

```bash
git clone https://github.com/itskreisler/rain.git
cd rain
go build -o rain .
# Optional: install globally
cp rain $PREFIX/bin/
```

## Quick Start

### 1. Install the toolchain

```bash
rain doctor --fix
```

If you already have `android.jar`, you can specify it with `--android-jar <path>`.

### 2. Initialize a project

```bash
rain init
```

Choose a resolution mode:
- `simple` - Explicit dependencies only, no transitive resolution. Best for small projects.
- `completo` (full) - Resolves transitive dependencies from Maven POMs. Required for AndroidX.

### 3. Configure `rain.toml`

```toml
mode = "completo"

dependencies = [
    "androidx.core:core:1.12.0",
    "androidx.appcompat:appcompat:1.6.1",
    "androidx.compose.material3:material3-android:1.5.0-beta01",
]

[project]
name = "MyApp"
package = "com.example.myapp"
version = "1.0"

[build]
debug = true

[signing]
store_file = "release.keystore"
store_password = "password"
key_alias = "alias"
key_password = "password"  # Optional, defaults to store_password

[[modules]]
name = "app"
path = "app"

[repositories]
extra = ["https://my-nexus.local/repository/maven"]  # Optional custom repos
```

### 4. Build your APK

```bash
# Debug build
rain build

# Release build with R8 minification
rain build --release

# Build from a specific directory
rain build /path/to/project
```

APKs are generated in `app/build/` as `app-debug.apk`, `app-release.apk`, and a descriptive copy like `<name>-<version>-<abi>.apk`.

### 5. Clean build artifacts

```bash
rain clean
```

## Project Structure

```text
my-app/
├── rain.toml          # Build configuration
├── app/
│   ├── src/
│   │   └── main/
│   │       ├── AndroidManifest.xml
│   │       ├── java/      # Java sources
│   │       ├── kotlin/    # Kotlin sources (optional)
│   │       └── res/       # Resources
│   └── build/         # Generated APKs and build artifacts
├── libs/              # Downloaded dependencies (generated)
└── cache/             # POM cache (generated)
```

## Commands

| Command | Description | Flags |
| --- | --- | --- |
| `rain init` | Initialize a new project | `--template simple\|completo` |
| `rain build [dir]` | Build and sign APK (supports flat android2apk layout) | `--android-jar <path>`, `--release`, `--allow-debug-signing`, `--verbose`, `--quiet` |
| `rain check [dir]` | Type-check without generating APK | `--fast`, `--android-jar <path>`, `--release` |
| `rain lint [dir]` | Run static analysis (ktlint, detekt, pmd) | `--install`, `--tools`, `--fail-on`, `--type-resolution` |
| `rain doctor` | Diagnose and install toolchain | `--profile simple\|complex`, `--fix`, `--lint`, `--json`, `--print-install` |
| `rain gradle` | Import a Gradle project and generate `rain.toml` | — |
| `rain deps` | Dependency management commands | — |
| `rain version` | Show Rain version | — |
| `rain clean` | Remove build artifacts, dependencies and caches | — |
| `rain completion` | Generate shell autocompletion scripts | — |

## Global Flags

| Flag | Description |
| --- | --- |
| `--devlog` | Show development logs for CLI functions |
| `-v, --verbose` | Show detailed execution output |
| `-q, --quiet` | Show only errors |
| `--no-color` | Disable ANSI colors |
| `-h, --help` | Show help for any command |

## `check` vs `lint`

| Feature | `rain check` | `rain lint` |
| --- | --- | --- |
| Purpose | Type checking via compiler | Style and code quality analysis |
| Detects | Unresolved references, broken imports, type errors | Formatting issues, unused imports, complexity, code smells |
| Tool | kotlinc/javac | ktlint, detekt, pmd |
| Speed | Fast with incremental cache (~3s) | ~20s |
| Output | Compilation errors | Warnings and style violations |

A linter cannot replace type checking—`check` compiles the code, `lint` analyzes source text.

## Lint Configuration

Install linters:
```bash
rain lint --install
# or
rain doctor --fix --lint
```

Configure in `rain.toml`:
```toml
[lint]
tools = ["ktlint", "detekt"]  # or ["auto"]
fail_on = "error"              # never | warning | error
type_resolution = false        # Enable type resolution for detekt (slower)
```

## Migration from android2apk

Projects using the flat `android2apk` layout (`app.properties`, setup scripts) work with Rain without modification:

```bash
rain build /path/to/android2apk-project
```

See the [migration guide](docs/migracion-android2apk.md) for details on converting `app.properties` to `rain.toml`, Compose dependencies, toolchain setup, and signing.

## How It Works

Rain executes the standard Android build pipeline:

1. **Dependency resolution** (full mode) - Parse Maven POMs, resolve transitive dependencies, download AAR/JAR to `libs/`
2. **Resource compilation** - `aapt2 compile` compiles `res/` to `.flat` files
3. **Resource linking** - `aapt2 link` generates `R.java` and the base APK from manifest and resources
4. **BuildConfig generation** - Create `BuildConfig.java` with `DEBUG`, `APPLICATION_ID`, `VERSION_NAME`, etc.
5. **Kotlin compilation** - `kotlinc` compiles `.kt` files to `.class` (if present)
6. **Java compilation** - `javac` compiles `.java` files (including generated `R.java` and `BuildConfig.java`)
7. **DEX conversion** - `d8` converts `.class` to `classes.dex`. In release mode, `R8` performs minification and obfuscation
8. **APK packaging** - Add `classes.dex` to the base APK
9. **Alignment** - `zipalign` optimizes the APK for memory
10. **Signing** - `apksigner` signs with debug or release keystore

Incremental builds use SHA-256 hashes of sources and resources to reuse previous `classes.dex` and base APK when unchanged.

## Documentation

| Document | Description |
| --- | --- |
| [docs/doctor.md](docs/doctor.md) | `rain doctor` checks and installation flags |
| [docs/toolchain.md](docs/toolchain.md) | Toolchain contents in `~/.rain/` and rationale |
| [docs/compatibilidad.md](docs/compatibilidad.md) | JDK / Kotlin / R8 / compileSdk compatibility matrix |
| [docs/memoria.md](docs/memoria.md) | Memory management for containers (GOMEMLIMIT, GOGC, JVM heap) |
| [docs/migracion-android2apk.md](docs/migracion-android2apk.md) | Porting guide from android2apk |

## Troubleshooting

| Issue | Solution |
| --- | --- |
| `aapt2: command not found` | Install aapt2: `pkg install aapt2` |
| `package X does not exist` during compilation | Dependencies may be missing. Delete `libs/` and rebuild to re-download. |
| Jansi error when compiling Kotlin | Install libjansi: `pkg install libjansi`. Rain passes `-J-Dlibrary.jansi.path=...` automatically. |
| Progress bar not showing | Ensure `libs/` doesn't exist or is empty. Rain only shows download progress when downloading. |
| Crashes with "fatal error: found pointer to free object" or OOM | Common in low-RAM containers or under PRoot. Rain auto-detects PRoot and applies conservative GC. For containers, try `export GOGC=15 GOMEMLIMIT=1200MiB`. See [docs/memoria.md](docs/memoria.md). |

## License

See [LICENSE](LICENSE) for details.
