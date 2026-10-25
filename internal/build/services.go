package build

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/itskreisler/rain/internal/aar"
)

// serviceProviders devuelve los proveedores de java.util.ServiceLoader que
// declaran las dependencias del proyecto, indexados por nombre de servicio.
//
// El ServiceLoader funciona con ficheros META-INF/services: el nombre del
// fichero es la interfaz y cada línea es una implementación. Rain los mete en
// el APK (como hace AGP) porque el ClassLoader de Android los busca ahí; sin
// ellos la app no encuentra al proveedor y revienta al arrancar. El caso
// típico es el dispatcher principal de las coroutines: sin él, Compose falla
// con "Module with the Main dispatcher is missing".
func (c Config) serviceProviders() (map[string][]string, error) {
	if c.Libs != nil {
		return c.Libs.ServiceFiles()
	}
	jars := c.libraryJars()
	return aar.ServiceFilesFromJars(jars)
}

// writeServiceFiles escribe los ficheros de servicio fusionados en dir y
// devuelve el mapa ruta dentro del APK -> fichero en disco.
//
// Se fusionan los ficheros que traen varias librerías con el mismo nombre:
// el ServiceLoader acepta varias implementaciones en el mismo fichero.
func writeServiceFiles(dir string, services map[string][]string) (map[string]string, error) {
	out := map[string]string{}
	if len(services) == 0 {
		return out, nil
	}

	names := make([]string, 0, len(services))
	for name := range services {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		// Un nombre de servicio con separadores de directorio rompería el
		// APK: se descarta en vez de escribir fuera de META-INF/services.
		if name == "" || strings.ContainsAny(name, `/\`) {
			continue
		}
		impls := services[name]
		if len(impls) == 0 {
			continue
		}
		path := filepath.Join(dir, "META-INF", "services", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("creando directorio de servicios: %w", err)
		}
		var b strings.Builder
		for _, impl := range impls {
			b.WriteString(impl)
			b.WriteString("\n")
		}
		if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
			return nil, fmt.Errorf("escribiendo %s: %w", path, err)
		}
		out["META-INF/services/"+name] = path
	}
	return out, nil
}

// serviceKeepRules genera las reglas ProGuard/R8 que mantienen vivos los
// proveedores de servicio.
//
// R8 solo es capaz de reescribir los ficheros META-INF/services cuando el
// build los usa como recursos de programa (así lo hace AGP). Rain los mete en
// el APK por su cuenta, así que R8 no sabe que esos clases existen por el
// servicio y los elimina: el ServiceLoader se queda sin nada que cargar.
//
// Con -keep la clase conserva su nombre (imprescindible: el fichero de
// servicio la nombra tal cual) y sus constructores, que es lo que el
// ServiceLoader necesita para instanciarla.
func serviceKeepRules(services map[string][]string) []byte {
	var b strings.Builder
	b.WriteString("# Generado por rain.\n")
	b.WriteString("# Los proveedores de java.util.ServiceLoader se descubren por los\n")
	b.WriteString("# ficheros META-INF/services del APK. R8 no reescribe esos ficheros,\n")
	b.WriteString("# asi que sus implementaciones se eliminan salvo que se le diga lo\n")
	b.WriteString("# contrario. Sin esto la app puede fallar al arrancar con\n")
	b.WriteString("# \"Module with the Main dispatcher is missing\".\n")

	names := make([]string, 0, len(services))
	for name := range services {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		for _, impl := range services[name] {
			fmt.Fprintf(&b, "-keep class %s { <init>(...); }\n", impl)
		}
	}
	return []byte(b.String())
}
