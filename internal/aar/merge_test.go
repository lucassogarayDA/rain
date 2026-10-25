package aar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeRes escribe un recurso dentro de un res/ dado.
func writeRes(t *testing.T, resDir, rel, content string) {
	t.Helper()
	path := filepath.Join(resDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMergeResourcesDedupValues(t *testing.T) {
	base := t.TempDir()
	resA := filepath.Join(base, "a")
	resB := filepath.Join(base, "b")
	dest := filepath.Join(base, "merged")

	writeRes(t, resA, "values/strings.xml", `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="app_name">A</string>
    <string name="only_a">solo A</string>
</resources>`)
	writeRes(t, resB, "values/strings.xml", `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="app_name">B</string>
    <string name="only_b">solo B</string>
</resources>`)

	if err := MergeResources([]string{resA, resB}, dest); err != nil {
		t.Fatalf("MergeResources: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(dest, "values", "merged.xml"))
	if err != nil {
		t.Fatalf("leyendo merged.xml: %v", err)
	}
	s := string(out)

	// Gana el primero que define (app_name -> A), pero se conservan ambos únicos.
	if !strings.Contains(s, `name="app_name"`) || !strings.Contains(s, ">A<") {
		t.Errorf("app_name debería ganar A (el primero):\n%s", s)
	}
	if strings.Contains(s, ">B<") {
		t.Errorf("app_name de B debería descartarse:\n%s", s)
	}
	if !strings.Contains(s, "solo A") || !strings.Contains(s, "solo B") {
		t.Errorf("faltan strings únicas:\n%s", s)
	}
	if n := strings.Count(s, `name="app_name"`); n != 1 {
		t.Errorf("app_name aparece %d veces, quiero 1:\n%s", n, s)
	}
}

func TestMergeResourcesCopiesNonValues(t *testing.T) {
	base := t.TempDir()
	resA := filepath.Join(base, "a")
	resB := filepath.Join(base, "b")
	dest := filepath.Join(base, "merged")

	writeRes(t, resA, "layout/main.xml", "<A/>")
	writeRes(t, resB, "layout/main.xml", "<B/>") // duplicado: gana A
	writeRes(t, resB, "layout/other.xml", "<O/>")
	writeRes(t, resA, "drawable/ic.xml", "<D/>")

	if err := MergeResources([]string{resA, resB}, dest); err != nil {
		t.Fatalf("MergeResources: %v", err)
	}

	main, err := os.ReadFile(filepath.Join(dest, "layout", "main.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(main) != "<A/>" {
		t.Errorf("layout/main.xml = %q, quiero <A/> (el primero)", main)
	}
	for _, rel := range []string{"layout/other.xml", "drawable/ic.xml"} {
		if _, err := os.Stat(filepath.Join(dest, filepath.FromSlash(rel))); err != nil {
			t.Errorf("falta %s: %v", rel, err)
		}
	}
}

func TestMergeResourcesSeparateConfigs(t *testing.T) {
	base := t.TempDir()
	resA := filepath.Join(base, "a")
	dest := filepath.Join(base, "merged")

	writeRes(t, resA, "values/strings.xml", `<?xml version="1.0"?><resources><string name="hi">Hola</string></resources>`)
	writeRes(t, resA, "values-es/strings.xml", `<?xml version="1.0"?><resources><string name="hi">Hola ES</string></resources>`)

	if err := MergeResources([]string{resA}, dest); err != nil {
		t.Fatalf("MergeResources: %v", err)
	}

	// Cada configuración va a su propio merged.xml (el mismo nombre en ambas es válido).
	for _, cfg := range []string{"values", "values-es"} {
		if _, err := os.Stat(filepath.Join(dest, cfg, "merged.xml")); err != nil {
			t.Errorf("falta %s/merged.xml: %v", cfg, err)
		}
	}
}

func TestMergeResourcesSkipsMissingDirs(t *testing.T) {
	base := t.TempDir()
	resA := filepath.Join(base, "a")
	dest := filepath.Join(base, "merged")
	writeRes(t, resA, "values/strings.xml", `<?xml version="1.0"?><resources><string name="x">y</string></resources>`)

	// Un directorio inexistente no debe abortar la fusión.
	if err := MergeResources([]string{filepath.Join(base, "no-existe"), "", resA}, dest); err != nil {
		t.Fatalf("MergeResources: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "values", "merged.xml")); err != nil {
		t.Errorf("falta merged.xml: %v", err)
	}
}

func TestMergeResourcesOverwritesDest(t *testing.T) {
	base := t.TempDir()
	resA := filepath.Join(base, "a")
	dest := filepath.Join(base, "merged")

	writeRes(t, resA, "values/strings.xml", `<?xml version="1.0"?><resources><string name="x">y</string></resources>`)
	// Basura previa en dest que debe desaparecer.
	writeRes(t, dest, "values/old.xml", "viejo")

	if err := MergeResources([]string{resA}, dest); err != nil {
		t.Fatalf("MergeResources: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "values", "old.xml")); !os.IsNotExist(err) {
		t.Errorf("dest debería limpiarse antes de fusionar (err=%v)", err)
	}
}

func TestParseResourcesRejectsNonResourcesRoot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.xml")
	if err := os.WriteFile(path, []byte(`<otro/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := parseResources(path); err == nil {
		t.Fatal("una raíz distinta de <resources> debería fallar")
	}
}
