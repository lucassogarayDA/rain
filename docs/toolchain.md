# Toolchain de Rain

Rain mantiene un toolchain aislado en el directorio personal del usuario (`~/.rain`), permitiendo compilar APKs sin permisos de root ni modificar la configuración del sistema.

## Estructura de `~/.rain`

```
~/.rain/
├── bin/            # Enlaces simbólicos y ejecutable wrappers
├── toolchain/
│   ├── r8.jar       # R8 descargado desde Google Maven
│   ├── android.jar  # SDK de Android (API 36)
│   ├── kotlinc/     # Compilador de Kotlin
│   └── aapt2        # Herramienta de empaquetado de recursos
└── cache/          # Caché de AARs y archivos POM
```

## Gestión del Toolchain

Puedes descargar y mantener actualizado el toolchain de Rain ejecutando:

```bash
rain doctor --fix
```

Esto descargará automáticamente los binarios necesarios en `~/.rain` sin requerir `sudo` ni gestores de paquetes del sistema operativo.
