package pom

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// metadataKMP es un recorte real del .module de androidx.compose.ui:ui-graphics
// 1.12.1: la variante Android redirige con "available-at" al módulo
// ui-graphics-android. Incluye un atributo booleano
// ("androidx.kmp.hasCInterop": false) porque el fichero real los trae y un
// map[string]string reventaba el parseo entero.
const metadataKMP = `{
  "formatVersion": "1.1",
  "component": { "group": "androidx.compose.ui", "module": "ui-graphics", "version": "1.12.1" },
  "variants": [
    {
      "name": "metadataApiElements",
      "attributes": {
        "org.gradle.category": "library",
        "org.gradle.usage": "java-api"
      },
      "dependencies": [
        { "group": "androidx.annotation", "module": "annotation", "version": { "requires": "1.7.0" } }
      ]
    },
    {
      "name": "linuxx64StubsApiElements-published",
      "attributes": {
        "androidx.kmp.hasCInterop": false,
        "org.gradle.jvm.environment": "linux-x64"
      }
    },
    {
      "name": "androidApiElements-published",
      "attributes": {
        "org.gradle.category": "library",
        "org.gradle.jvm.environment": "android",
        "org.gradle.libraryelements": "aar",
        "org.gradle.usage": "java-api",
        "org.jetbrains.kotlin.platform.type": "androidJvm"
      },
      "available-at": {
        "url": "../../ui-graphics-android/1.12.1/ui-graphics-android-1.12.1.module",
        "group": "androidx.compose.ui",
        "module": "ui-graphics-android",
        "version": "1.12.1"
      }
    }
  ]
}`

// metadataSinAtributoEntorno reproduce el .module real de
// androidx.compose.material:material-icons-core:1.7.8. Este módulo NO publica
// el atributo "org.gradle.jvm.environment": su variante Android se identifica
// solo por "org.jetbrains.kotlin.platform.type=androidJvm" y por el módulo al
// que apunta ("material-icons-core-android").
//
// Con la búsqueda por el atributo de entorno, la raíz KMP se quedaba como
// "cáscara" vacía (el AAR solo trae AndroidManifest.xml) y sus clases no
// llegaban al classpath: kotlinc fallaba con "unresolved reference 'Icons'".
const metadataSinAtributoEntorno = `{
  "formatVersion": "1.1",
  "component": { "group": "androidx.compose.material", "module": "material-icons-core", "version": "1.7.8" },
  "variants": [
    {
      "name": "metadataApiElements",
      "attributes": {
        "org.gradle.category": "library",
        "org.gradle.usage": "kotlin-metadata",
        "org.jetbrains.kotlin.platform.type": "common"
      }
    },
    {
      "name": "releaseApiElements-published",
      "attributes": {
        "org.gradle.category": "library",
        "org.gradle.usage": "java-api",
        "org.jetbrains.kotlin.platform.type": "androidJvm"
      },
      "available-at": {
        "url": "../../material-icons-core-android/1.7.8/material-icons-core-android-1.7.8.module",
        "group": "androidx.compose.material",
        "module": "material-icons-core-android",
        "version": "1.7.8"
      }
    },
    {
      "name": "desktopApiElements-published",
      "attributes": {
        "org.gradle.category": "library",
        "org.gradle.usage": "java-api",
        "org.jetbrains.kotlin.platform.type": "jvm"
      },
      "available-at": {
        "url": "../../material-icons-core-desktop/1.7.8/material-icons-core-desktop-1.7.8.module",
        "group": "androidx.compose.material",
        "module": "material-icons-core-desktop",
        "version": "1.7.8"
      }
    }
  ]
}`

func TestAndroidVariantSinAtributoDeEntorno(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "material-icons-core.module")
	if err := os.WriteFile(path, []byte(metadataSinAtributoEntorno), 0o644); err != nil {
		t.Fatalf("escribiendo metadata: %v", err)
	}
	md, err := ParseModuleMetadata(path)
	if err != nil {
		t.Fatalf("ParseModuleMetadata: %v", err)
	}

	v := md.AndroidVariant()
	if v == nil {
		t.Fatal("AndroidVariant devolvió nil")
	}
	if v.Name != "releaseApiElements-published" {
		t.Errorf("variante = %q, quiero %q", v.Name, "releaseApiElements-published")
	}

	want := "androidx.compose.material:material-icons-core-android:1.7.8"
	if got := md.AndroidTargetCoord(); got != want {
		t.Errorf("AndroidTargetCoord = %q, quiero %q", got, want)
	}
}

// TestAndroidVariantIgnoraLaVarianteDeEscritorio comprueba que, sin atributo
// de entorno, no se elige por error la variante de escritorio (jvm).
func TestAndroidVariantIgnoraLaVarianteDeEscritorio(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "material-icons-core.module")
	if err := os.WriteFile(path, []byte(metadataSinAtributoEntorno), 0o644); err != nil {
		t.Fatalf("escribiendo metadata: %v", err)
	}
	md, err := ParseModuleMetadata(path)
	if err != nil {
		t.Fatalf("ParseModuleMetadata: %v", err)
	}

	v := md.AndroidVariant()
	if v == nil {
		t.Fatal("AndroidVariant devolvió nil")
	}
	if strings.Contains(v.Name, "desktop") {
		t.Errorf("se eligió la variante de escritorio: %q", v.Name)
	}
	if v.AvailableAt == nil || !strings.HasSuffix(v.AvailableAt.Module, "-android") {
		t.Errorf("la variante no apunta a un módulo -android: %+v", v.AvailableAt)
	}
}

func TestParseModuleMetadataConAtributosBooleanos(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ui-graphics.module")
	if err := os.WriteFile(path, []byte(metadataKMP), 0o644); err != nil {
		t.Fatalf("escribiendo metadata: %v", err)
	}

	md, err := ParseModuleMetadata(path)
	if err != nil {
		t.Fatalf("ParseModuleMetadata: %v", err)
	}
	if len(md.Variants) != 3 {
		t.Fatalf("Variants = %d, quiero 3", len(md.Variants))
	}
}

func TestAndroidVariantIgnoraVariantesNoAndroid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "m.module")
	if err := os.WriteFile(path, []byte(metadataKMP), 0o644); err != nil {
		t.Fatalf("escribiendo metadata: %v", err)
	}
	md, err := ParseModuleMetadata(path)
	if err != nil {
		t.Fatalf("ParseModuleMetadata: %v", err)
	}

	v := md.AndroidVariant()
	if v == nil {
		t.Fatal("AndroidVariant devolvió nil")
	}
	if v.Name != "androidApiElements-published" {
		t.Errorf("variante = %q, quiero %q", v.Name, "androidApiElements-published")
	}
}

func TestAndroidTargetCoordSigueAvailableAt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "m.module")
	if err := os.WriteFile(path, []byte(metadataKMP), 0o644); err != nil {
		t.Fatalf("escribiendo metadata: %v", err)
	}
	md, err := ParseModuleMetadata(path)
	if err != nil {
		t.Fatalf("ParseModuleMetadata: %v", err)
	}

	got := md.AndroidTargetCoord()
	want := "androidx.compose.ui:ui-graphics-android:1.12.1"
	if got != want {
		t.Errorf("AndroidTargetCoord = %q, quiero %q", got, want)
	}
}

func TestAndroidTargetCoordVacioSiNoHayRedireccion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "m.module")
	content := `{
	  "variants": [
	    {
	      "name": "androidApiElements-published",
	      "attributes": {
	        "org.gradle.jvm.environment": "android",
	        "org.gradle.usage": "java-api"
	      },
	      "dependencies": [
	        { "group": "androidx.core", "module": "core", "version": { "requires": "1.19.1" } }
	      ]
	    }
	  ]
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("escribiendo metadata: %v", err)
	}
	md, err := ParseModuleMetadata(path)
	if err != nil {
		t.Fatalf("ParseModuleMetadata: %v", err)
	}

	if got := md.AndroidTargetCoord(); got != "" {
		t.Errorf("AndroidTargetCoord = %q, quiero \"\"", got)
	}
	deps := md.AndroidDependencies()
	if len(deps) != 1 || deps[0] != "androidx.core:core:1.19.1" {
		t.Errorf("AndroidDependencies = %v, quiero [androidx.core:core:1.19.1]", deps)
	}
}
