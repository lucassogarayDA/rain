package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWriteServiceFilesFusionaLosProveedores comprueba que los ficheros de
// servicio de varias librerías se fusionan en uno solo (el ServiceLoader
// admite varias implementaciones en el mismo fichero).
func TestWriteServiceFilesFusionaLosProveedores(t *testing.T) {
	dir := t.TempDir()

	services := map[string][]string{
		"com.example.Service": {"com.example.A", "com.example.B"},
	}

	res, err := writeServiceFiles(filepath.Join(dir, "javares"), services)
	if err != nil {
		t.Fatalf("writeServiceFiles: %v", err)
	}

	want := "META-INF/services/com.example.Service"
	path, ok := res[want]
	if !ok {
		t.Fatalf("falta la entrada %s en %v", want, res)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "com.example.A\ncom.example.B\n" {
		t.Errorf("contenido = %q, quiero las dos implementaciones", data)
	}
}

// TestWriteServiceFilesIgnoraNombresInvalidos comprueba que un nombre con
// separadores de directorio no escapa de META-INF/services.
func TestWriteServiceFilesIgnoraNombresInvalidos(t *testing.T) {
	dir := t.TempDir()

	res, err := writeServiceFiles(filepath.Join(dir, "javares"), map[string][]string{
		"../escapada": {"com.example.A"},
	})
	if err != nil {
		t.Fatalf("writeServiceFiles: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("res = %v, quiero ningún recurso", res)
	}
}

// TestServiceKeepRulesMantieneLosProveedores comprueba que las reglas
// generadas para R8 mantienen la clase y sus constructores: el nombre de la
// clase no se puede ofuscar porque el fichero de servicio la nombra tal cual.
func TestServiceKeepRulesMantieneLosProveedores(t *testing.T) {
	rules := string(serviceKeepRules(map[string][]string{
		"kotlinx.coroutines.internal.MainDispatcherFactory": {
			"kotlinx.coroutines.android.AndroidDispatcherFactory",
		},
	}))

	if !strings.Contains(rules, "-keep class kotlinx.coroutines.android.AndroidDispatcherFactory { <init>(...); }") {
		t.Errorf("las reglas no mantienen al proveedor:\n%s", rules)
	}
}
