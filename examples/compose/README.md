# Icon Catalog Example (`examples/compose`)

Este ejemplo es un catálogo de iconos animados hecho con Jetpack Compose, pensado para ejecutarse con el sistema de build `rain` (sin Gradle).

## Características

- **Galería**: Grid con búsqueda por nombre de icono
- **Detalle en Bottom Sheet**: Al tocar un icono se abre un `ModalBottomSheet` desde abajo con sus controles
- **Animaciones interactivas**: Soporta animación por tap/click y hover (según el icono)
- **Controles**: Tamaño (16 - 128 dp), duración de animación (0.1 - 3.0 s), selector de color
- **Estados controlables**: Cada icono animado expone un `State` con `startAnimation()` / `stopAnimation()`

## Iconos incluidos

| ID | Nombre | Tipo | Notas |
|---|---|---|---|
| `fire` | `FireIcon` | Animado | Animación tipo llama/fuego |
| `flame` | `FlameIcon` | Animado | Variante con animación de llama |
| `cloudsync` | `CloudSyncIcon` | Animado | Giro + escala del grupo de sync |
| `bot` | `BotIcon` | Animado | Cabeza que rebota, antena y parpadeo de ojos |
| `mochi` | `Mochi` (`FireIconFlat`) | Estático (SVG) | Solo ajustes de tamaño, sin animación/color dinámico |

## Estructura

```text
examples/compose/
├── app/java/com/kyant/backdrop/catalog/
│   ├── MainActivity.kt        # Entry point
│   ├── IconsShowcaseApp.kt   # Galería + ModalBottomSheet con detalles
│   ├── FireIcon.kt           # Icono animado
│   ├── FlameIcon.kt          # Icono animado
│   ├── CloudSyncIcon.kt      # Icono animado
│   ├── BotIcon.kt            # Icono animado
│   ├── FireIconFlat.kt       # Icono estático (Mochi)
│   └── FireIconDemo.kt       # Demo auxiliar
└── rain.toml                 # Configuración de rain
```

## Configuración (`rain.toml`)

```toml
mode = "completo"

dependencies = [
    "androidx.activity:activity-compose:1.8.2",
    "androidx.compose.material3:material3-android:1.5.0-beta01",
]

[project]
name = "FireIconCatalog"
package = "com.kyant.backdrop.catalog"
version = "1.0"

[build]
debug = true
```

> Nota: `rain` no soporta sintaxis `platform(...)` (BOM) en este momento. Las dependencias se declaran explícitamente.

## Cómo compilar

Desde el directorio del ejemplo:

```bash
/root/projects/rain/rain build .
```

El build genera:
- `app/build/FireIconCatalog-1.0-universal.apk` (APK universal)
- `app/build/app-debug.apk` (ruta estable)

## Información del último build

| Campo | Valor |
|---|---|
| Fecha/Hora | 2026-10-24 ~04:57 UTC (aprox.) |
| Duración | ~113 segundos |
| Tamaño APK universal | ~11.1 MiB |
| Tamaño APK debug copiado | ~12 MiB |
| Ubicación copiada | `/data/data/com.termux/files/home/storage/shared/FireIconCatalog-debug.apk` |

## Notas de rendimiento

El tiempo de build (~1m53s) es considerable para este ejemplo. Posibles optimizaciones a considerar para el futuro:
- Mejorar caché de dependencias/resolución
- Paralelización en compilación Kotlin/DEX
- Optimización de deduplicación/merging
- Revisión del pipeline de empaquetado/firma

## Notas técnicas

- **Navegación**: Se usa estado manual en lugar de `NavHost` para evitar problemas de compatibilidad con ViewModel/lifecycle.
- **Bottom Sheet**: `ModalBottomSheet` de Material 3 con `skipPartiallyExpanded = true`.
- **Iconos animados**: Implementados con `Canvas` + `Animatable`/`coroutines` siguiendo el estilo de los ejemplos React.
- **Búsqueda**: Filtrado en tiempo real por nombre de icono.
