package build

import (
	"os"
	"path/filepath"
	"testing"
)

// writeDexCache simula el estado del directorio de build tras una fase de
// compilación.
//
//	javaHash:  la marca que escribe cualquier compilación (incluido "rain check")
//	dexHash:   la marca que solo escribe un build que llegó a generar el DEX
//	withDex:   si existe el classes.dex
func writeDexCache(t *testing.T, buildDir, javaHash, dexHash string, withDex bool) {
	t.Helper()
	if javaHash != "" {
		writeHashFile(t, filepath.Join(buildDir, javaHashFile), javaHash)
	}
	if dexHash != "" {
		writeHashFile(t, filepath.Join(buildDir, dexHashFile), dexHash)
	}
	if withDex {
		writeHashFile(t, filepath.Join(buildDir, "dex", "classes.dex"), "dex")
	}
}

// TestCheckNoEnvenenaElBuildSiguiente es el test de regresión del bug más
// grave que se ha encontrado: "rain check" escribía la marca de compilación y
// el "rain build" siguiente reutilizaba el classes.dex anterior, así que el
// APK llevaba código que ya no estaba en el disco.
func TestCheckNoEnvenenaElBuildSiguiente(t *testing.T) {
	dir := t.TempDir()
	const hash = "hash-de-las-fuentes"

	// Estado tras un build correcto: hay DEX y su marca.
	writeDexCache(t, dir, hash, hash, true)
	if !dexIsUpToDate(dir, hash) {
		t.Fatal("tras un build correcto el DEX debería poder reutilizarse")
	}

	// Ahora se modifican las fuentes y se ejecuta "rain check": compila bien
	// (marca de java actualizada) pero NO genera DEX.
	writeDexCache(t, dir, "hash-nuevo", hash, true)
	if dexIsUpToDate(dir, "hash-nuevo") {
		t.Error("el DEX se reutilizó tras un check: el APK llevaría código viejo")
	}
}

// TestDexNoSeReutilizaSiCambianLasFuentes comprueba que un cambio en las
// fuentes invalida el DEX aunque la marca de compilación siga ahí.
func TestDexNoSeReutilizaSiCambianLasFuentes(t *testing.T) {
	dir := t.TempDir()
	writeDexCache(t, dir, "hash-1", "hash-1", true)

	if dexIsUpToDate(dir, "hash-2") {
		t.Error("el DEX se reutilizó con fuentes distintas")
	}
	if !dexIsUpToDate(dir, "hash-1") {
		t.Error("el DEX no se reutilizó sin cambios")
	}
}

// TestDexNoSeReutilizaSiFaltaElFichero cubre el caso en que la marca existe
// pero el classes.dex se ha borrado.
func TestDexNoSeReutilizaSiFaltaElFichero(t *testing.T) {
	dir := t.TempDir()
	writeDexCache(t, dir, "hash-1", "hash-1", false)

	if dexIsUpToDate(dir, "hash-1") {
		t.Error("el DEX se reutilizó aunque no exista el classes.dex")
	}
}

// TestSinMarcasNoHayReutilizacion: sin marcas (proyecto recién clonado) no
// se puede reutilizar nada.
func TestSinMarcasNoHayReutilizacion(t *testing.T) {
	dir := t.TempDir()
	writeHashFile(t, filepath.Join(dir, "dex", "classes.dex"), "dex")

	if dexIsUpToDate(dir, "hash-1") {
		t.Error("el DEX se reutilizó sin ninguna marca")
	}
}

// TestLasMarcasSonDistintas documenta que las dos marcas no son el mismo
// fichero: si se mezclaran, el bug volvería.
func TestLasMarcasSonDistintas(t *testing.T) {
	if javaHashFile == dexHashFile {
		t.Fatal("las marcas de compilación y de DEX deben ser ficheros distintos")
	}
	if javaHashFile == "" || dexHashFile == "" {
		t.Fatal("las marcas de compilación no pueden estar vacías")
	}
}

// TestCompileSourcesLimpiaElDirectorioDeObjetos comprueba que al compilar
// desde cero no quedan clases de ficheros borrados.
func TestCompileSourcesLimpiaElDirectorioDeObjetos(t *testing.T) {
	dir := t.TempDir()
	obj := filepath.Join(dir, "obj")

	// Una clase "vieja" de un fichero que ya no existe.
	writeHashFile(t, filepath.Join(obj, "com", "x", "Viejo.class"), "clase")

	// Una configuración mínima: sin fuentes, el compilador no tiene nada que
	// hacer, pero el directorio debe quedar limpio.
	cfg := Config{AppDir: filepath.Join(dir, "app"), BuildDir: dir}
	if _, err := compileSources(cfg, filepath.Join(dir, "gen"), obj); err != nil {
		t.Fatalf("compileSources: %v", err)
	}

	if _, err := os.Stat(filepath.Join(obj, "com", "x", "Viejo.class")); !os.IsNotExist(err) {
		t.Error("quedó una clase de un fichero borrado")
	}
	if _, err := os.Stat(obj); err != nil {
		t.Errorf("el directorio de objetos no existe tras compilar: %v", err)
	}
}
