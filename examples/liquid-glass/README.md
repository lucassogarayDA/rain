# Liquid Glass Music Player (`examples/liquid-glass`)

Ejemplo Android completo escrito en **Kotlin + Jetpack Compose** que reproduce
una interfaz de reproductor musical con efecto *liquid glass*. Sirve como
prueba de humo del pipeline de Rain con Compose: recursos, Kotlin, DEX,
empaquetado y firma.

## Qué es (y qué no es)

- **Es una demo de UI.** No reproduce audio real, no lee archivos del
  dispositivo y no pide permisos de almacenamiento.
- La lista de reproducción es de ejemplo y el progreso avanza con el reloj
  para que la interacción se sienta real. Al llegar al final salta a la
  siguiente pista.
- El estado del onboarding y la preferencia de reproducción automática se
  persisten en `SharedPreferences`.

Si buscas un ejemplo de integración real con `MediaPlayer`, este no es el
sitio: aquí se prioriza enseñar el pipeline de Rain y el efecto visual.

## Compilar

Requiere toolchain Android y Kotlin (`rain doctor --fix --profile complex`).

```bash
cd examples/liquid-glass
../../rain --devlog build
```

El APK queda en:

```text
app/build/LiquidGlassMusicPlayer-1.0-universal.apk
app/build/app-debug.apk          # ruta estable que usa CI
```

Verificación de firma (lo mismo que hace CI):

```bash
java -jar ~/.rain/toolchain/apksigner.jar verify --verbose app/build/app-debug.apk
```

## Estructura

```text
liquid-glass/
├── rain.toml                     # mode = "completo", dependencias y proyecto
├── app/
│   ├── AndroidManifest.xml       # sin permisos: la demo no toca archivos
│   ├── java/com/kyant/liquidglass/music/
│   │   ├── MainActivity.kt       # onboarding <-> reproductor, prefs
│   │   ├── OnboardingView.kt     # 3 pasos honestos + preferencia real
│   │   ├── MusicPlayerView.kt    # progreso, pista actual, lista
│   │   └── LiquidGlass.kt        # fondo, tarjeta, botón, switch y slider
│   └── build/                    # generado por Rain (ignorado por git)
├── cache/                        # POMs descargados (ignorado por git)
└── .rain-deps/                   # AAR/JAR preparados (ignorado por git)
```

## Decisiones de UI/UX

- **Onboarding honesto.** No simula permisos ni carpetas: explica que la
  lista es de ejemplo y ofrece un ajuste que sí se aplica (reproducción
  automática). Tiene botón **Saltar**.
- **Progreso real.** El tiempo transcurrido se calcula desde el progreso y la
  duración de cada pista, en lugar de un `1:18` fijo.
- **Slider arrastrable** con toque y gesto horizontal sobre un área táctil de
  48 dp; expone `ProgressBarRangeInfo` y `setProgress` para TalkBack.
- **Accesibilidad.** Los controles con icono usan `ImageVector` + `Icon` con
  `contentDescription`; el interruptor usa `Role.Switch` y las pistas
  `Role.RadioButton`.
- **Iconos, no glifos.** La UI usa `androidx.compose.material:material-icons-extended`
  en lugar de emojis. Cada icono informativo lleva `contentDescription`; los
  decorativos pasan `null`. Es una dependencia grande: en debug el APK crece
  porque no hay shrinking, y en release R8 elimina los iconos no usados.
- **Pantallas pequeñas.** El reproductor usa un `LazyColumn` raíz, así que el
  contenido se desplaza en vez de recortarse; la carátula se limita a 220 dp.
- **Estado sobrevive a la rotación** con `rememberSaveable`; las preferencias
  sobreviven al proceso con `SharedPreferences`.

## DX: cosas que suelen fallar

- `../../rain` es el binario del repo. Si está desactualizado, ejecuta
  `make build` en la raíz antes de probar el ejemplo.
- Sin `--devlog` no ves qué funciones del pipeline se ejecutan; úsalo siempre.
- No edites `app/build/`, `cache/` ni `.rain-deps/` a mano: se regeneran.
- `[build] debug` en `rain.toml` no se usa; la variante release se activa con
  `rain build --release`.
- El primer build descarga decenas de AAR (incluido
  `material-icons-extended`, que es grande) y tarda minutos. Los siguientes
  reutilizan `cache/`, `.rain-deps/` y el DEX de dependencias.
