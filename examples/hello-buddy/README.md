# Hello Buddy (`examples/hello-buddy`)

Ejemplo mínimo con el **layout plano estilo android2apk**: sin `rain.toml`, sin
AndroidX y con la interfaz construida en Java. Sirve para comprobar que Rain
compila proyectos planos sin tocar nada.

## Estructura

```text
hello-buddy/
├── AndroidManifest.xml
├── app.properties          # versionCode, versionName, minSdk...
├── java/com/buddyapps/hello/MainActivity.java
└── res/values/strings.xml
```

- Package `com.buddyapps.hello`, activity lanzadora con `android:exported="true"`.
- Tema `@android:style/Theme.DeviceDefault` (sin AppCompat).
- Icono reciclado del sistema: `@android:drawable/sym_def_app_icon`.
- UI en código: un contenedor con fondo claro, una tarjeta y dos filas de dos
  columnas con `layout_weight` y gutters (estilo grid de Bootstrap).

## Compilar

```bash
rain build examples/hello-buddy
```

O desde dentro del proyecto, porque Rain detecta el layout plano al no haber
`rain.toml`:

```bash
cd examples/hello-buddy
rain build
```

El APK queda en `build/Hello-Buddy-1.0-universal.apk` (y `build/app-debug.apk`).

## `app.properties` vs `rain.toml`

`app.properties` es el formato heredado de android2apk y **solo cubre unos
pocos campos**. `rain.toml` es la configuración completa:

| `app.properties` | `rain.toml` |
| --- | --- |
| `versionCode`, `versionName` | `[build] versionCode`, `versionName` |
| `minSdk`, `targetSdk`, `javaTarget` | `[build] minSdk`, `targetSdk`, `javaTarget` |
| `package` (respaldo) | `[project] package` |
| — | `dependencies`, `modules`, `[signing]`, `[kapt]`, `[lint]`, `[repositories]`, `[build] proguard_files` |

La precedencia es `rain.toml` > `app.properties` > defaults de Rain. En este
ejemplo no hay dependencias ni módulos, así que `app.properties` basta.
