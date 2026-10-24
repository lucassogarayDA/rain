# Memoria en contenedores y dispositivos con poca RAM

Rain está pensado para correr en Termux, en contenedores Alpine y en Docker
sobre Android, donde la RAM disponible es mucho menor que la de un portátil.
En esos entornos el fallo más habitual **no** es un error de compilación, sino
que el proceso muere:

```
fatal error: found pointer to free object
runtime: g 0 gp=0x... m=0x...: unknown pc
```

Es el runtime de Go reventando por presión de memoria. La causa raíz casi
siempre es la misma: **la JVM reserva 1/4 de la RAM física** aunque el
contenedor solo disponga de una fracción.

## Diagnóstico

```bash
rain doctor --profile complex
```

El check `memoria` muestra el total, el presupuesto disponible, el heap que se
pasará a la JVM y el estado de `GOMEMLIMIT`/`GOGC`:

```
⚠️  memoria          total 10.9 GiB, disponible 2.7 GiB, heap JVM 1280m, GOMEMLIMIT sin definir, GOGC sin definir → memoria justa: export GOGC=15 GOMEMLIMIT=1200MiB
```

## Variables de entorno

| Variable | Ámbito | Efecto |
| --- | --- | --- |
| `GOMEMLIMIT` | runtime de Go | Límite blando del heap de Go (`1200MiB`, `2GiB`, ...). Es la variable estándar de Go: si está puesta, Rain **no** aplica ningún límite automático. |
| `GOGC` | runtime de Go | Porcentaje de crecimiento del heap que dispara el GC. `15` es agresivo y mantiene el heap muy bajo. Estándar de Go. |
| `RAIN_GOMEMLIMIT` | Rain | Igual que `GOMEMLIMIT` pero lo aplica Rain al arrancar (`auto` = según la memoria detectada). |
| `RAIN_GOGC` | Rain | Porcentaje de GC que aplica Rain al arrancar. |
| `RAIN_JAVA_MAX_HEAP` | JVM | Heap máximo de `kotlinc`, `d8` y `apksigner` (`1g`, `1536m`). |
| `RAIN_JAVA_OPTS` | JVM | Opciones extra de la JVM, separadas por espacios (p. ej. `-XX:+UseSerialGC`). |
| `JAVA_TOOL_OPTIONS` | JVM | Variable estándar de Java; la JVM la lee sola. Rain detecta un `-Xmx` aquí y no pisa tu elección. |
| `JAVA_OPTS` | kotlinc | Variable que el propio lanzador de Kotlin ya respeta. |

### Recomendación para contenedores

```bash
export GOGC=15
export GOMEMLIMIT=1200MiB
export RAIN_JAVA_MAX_HEAP=1g
```

Con esos tres valores, un proyecto Kotlin + Compose compila con holgura en un
contenedor con ~2,5 GB libres.

## Qué hace Rain por su cuenta

1. **Acota el heap de la JVM.** La JVM reserva 1/4 de la RAM física por
   defecto. Rain mide la memoria disponible (el límite de cgroup si el proceso
   vive en un contenedor, o `MemAvailable` de `/proc/meminfo`) y pasa a
   `kotlinc` un `-Xmx` con la mitad de ese presupuesto, con un mínimo de
   256 MiB y un tope de 2 GiB. Los lanzadores de `d8` y `apksigner` usan 1g y
   512m respectivamente.

2. **Pisa lo que el usuario haya puesto.** Si detecta un `-Xmx` en
   `RAIN_JAVA_MAX_HEAP`, `JAVA_OPTS`, `JAVA_TOOL_OPTIONS` o `JDK_JAVA_OPTIONS`,
   no inyecta nada: tu elección manda.

3. **Protege el heap de Go.** Si no hay `GOMEMLIMIT`, Rain aplica un límite
   blando cuando la memoria está justa (la mitad del presupuesto, con tope de
   1 GiB) y baja el `GOGC` a 15. Si has exportado `GOMEMLIMIT` o `GOGC`, Rain
   los respeta y no toca nada.

4. **Se adapta a `proot`.** Si el proceso corre bajo `proot` (proot-distro,
   PRoot-Distro, contenedores con `proot`), Rain aplica GC conservador aunque
   la memoria abunde. Ver la sección siguiente.

## `proot`: cuando el runtime de Go se corrompe sin falta de memoria

En entornos con `proot` (PRoot-Distro sobre Android, contenedores Alpine
emulados con `proot`) el build de Rain puede morir con:

```
fatal error: found pointer to free object
SIGSEGV: segmentation violation
runtime.findRunnable / runtime.becomeSpinning
```

**No es falta de memoria.** El muestreo durante el fallo muestra el heap de Go
en 13–39 MB y 3,6–4,8 GB disponibles: la presión no explica nada.

La causa es `proot` en sí: intercepta las llamadas al sistema con `ptrace`, y
el runtime de Go usa señales y temporizadores de una forma que `proot` no
gestiona bien. El recolector acaba liberando memoria que el runtime todavía
referencia, y aparece el "pointer to free object" o el `SIGSEGV` en el
planificador de goroutines.

### Qué hace Rain

Rain detecta `proot` (variables `PROOT_TMP_DIR`, `PROOT_L2S_DIR`,
`PROOT_NO_SECCOMP` o `PROOT_LOADER`, y `TracerPid != 0` en
`/proc/self/status`) y aplica **GC conservador**: `GOGC=15` y
`GOMEMLIMIT=1200MiB`. El heap se queda en unos pocos MB, el recolector pasa
frecuentemente y el *scavenger* casi no llama a `madvise`, así que hay menos
llamadas al sistema que `proot` pueda estropear.

Es la misma recomendación que habría que dar a mano (`export GOGC=15` y
`export GOMEMLIMIT=1200MiB`), pero aplicada **automáticamente** y solo cuando
hace falta: fuera de `proot` Rain no cambia nada.

Si exportas `GOGC`, `GOMEMLIMIT`, `RAIN_GOGC` o `RAIN_GOMEMLIMIT`, tu valor
manda sobre el que Rain deduciria.

### Cómo comprobarlo

```bash
rain doctor --profile complex
```

El check `memoria` lo dice explícitamente:

```
✅ memoria   total 10.9 GiB, disponible 4.3 GiB, heap JVM 2048m, límite Go 1.2 GiB,
             GOGC 15 (automático), proot (GC conservador aplicado)
```

## Medir el consumo

```bash
GODEBUG=gctrace=1 rain build   # una línea por ciclo de GC
```

En el caso típico de un contenedor se ve cómo el heap de Go se mantiene
estable y la JVM se queda dentro de su `-Xmx`.
