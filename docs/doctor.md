# Diagnóstico y Entorno — Rain Doctor

`rain doctor` comprueba que el entorno contenga las herramientas necesarias para compilar aplicaciones Android.

## Uso

```bash
rain doctor [opciones]
```

### Opciones

- `--profile <simple|complex>`: Perfil de comprobación.
  - `simple`: Comprueba JDK 17+, AAPT2, zipalign, apksigner y android.jar (para proyectos Java).
  - `complex`: Comprueba `kotlinc`, el plugin de Kotlin Compose, R8 y dependencias avanzadas.
- `--json`: Devuelve el diagnóstico en formato JSON estructurado.
- `--print-install`: Imprime las instrucciones de instalación del sistema.
- `--fix`: Descarga y configura automáticamente las herramientas que faltan en `~/.rain/bin` (sin requerir acceso root).
- `--lint`: Con `--fix`, descarga además los analizadores estáticos opcionales
  (ktlint 1.8.0 y detekt 1.23.8) en `~/.rain/toolchain`. Es la misma operación
  que `rain lint --install`.

## Checks

Además de las herramientas, `rain doctor` informa del estado de la memoria
(check `memoria`): total disponible, heap que se pasará a la JVM y valores de
`GOMEMLIMIT`/`GOGC`. Es el primer sitio donde mirar cuando una compilación
muere con `fatal error: found pointer to free object`.

Ver [memoria.md](memoria.md) para el detalle de las variables de entorno.

### Analizadores estáticos (ktlint y detekt)

`rain doctor` incluye dos comprobaciones **opcionales**: `ktlint` y `detekt`.
No hacen falta para compilar un APK, así que no bloquean el estado `Ready`;
solo avisan de si están disponibles para `rain lint`.

| Check | Obligatorio | Qué significa |
| --- | --- | --- |
| `ktlint` | No | Analizador de estilo de Kotlin. Si no está, se instala con `rain doctor --fix --lint`. |
| `detekt` | No | Analizador de "code smells" de Kotlin. Igual que ktlint. |

Los dos se buscan primero en el toolchain de Rain (`~/.rain/toolchain`) y
después en el `PATH`. `pmd` no aparece en el informe: se usa si está
instalado en el sistema y se detecta por el `PATH`.

## Ejemplo

```bash
# Diagnóstico simple
rain doctor

# Instalar/corregir herramientas faltantes en el espacio de usuario (~/.rain)
rain doctor --fix
```
