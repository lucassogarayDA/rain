package pom

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"2.1.0", "2.1.0"},
		{"[2.1.0]", "2.1.0"},
		{"[1.0,2.0)", "1.0"},
		{"[1.0,2.0]", "1.0"},
		{"(1.0,2.0]", ""},
		{"  [3.4.5]  ", "3.4.5"},
		{"", ""},
	}
	for _, tc := range tests {
		if got := NormalizeVersion(tc.in); got != tc.want {
			t.Errorf("NormalizeVersion(%q) = %q, quiero %q", tc.in, got, tc.want)
		}
	}
}

func TestParsePOM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo-1.0.pom")
	writeFile(t, path, `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <groupId>com.demo</groupId>
  <artifactId>demo</artifactId>
  <version>1.0</version>
  <properties>
    <kotlin.version>2.4.21</kotlin.version>
  </properties>
  <dependencyManagement>
    <dependencies>
      <dependency>
        <groupId>androidx.core</groupId>
        <artifactId>core</artifactId>
        <version>1.12.0</version>
      </dependency>
    </dependencies>
  </dependencyManagement>
  <dependencies>
    <dependency>
      <groupId>org.jetbrains.kotlin</groupId>
      <artifactId>kotlin-stdlib</artifactId>
      <version>${kotlin.version}</version>
    </dependency>
  </dependencies>
</project>`)

	p, err := ParsePOM(path)
	if err != nil {
		t.Fatalf("ParsePOM: %v", err)
	}
	if p.GroupID != "com.demo" || p.ArtifactID != "demo" || p.Version != "1.0" {
		t.Errorf("campos base = %q:%q:%q", p.GroupID, p.ArtifactID, p.Version)
	}
	if len(p.Properties) != 1 || p.Properties[0].XMLName.Local != "kotlin.version" {
		t.Errorf("properties = %+v", p.Properties)
	}
	if p.DependencyManagement == nil || len(p.DependencyManagement.Dependencies) != 1 {
		t.Fatalf("dependencyManagement = %+v", p.DependencyManagement)
	}
	if len(p.Dependencies) != 1 || p.Dependencies[0].ArtifactID != "kotlin-stdlib" {
		t.Errorf("dependencies = %+v", p.Dependencies)
	}
}

func TestParsePOMMissing(t *testing.T) {
	if _, err := ParsePOM(filepath.Join(t.TempDir(), "noexiste.pom")); err == nil {
		t.Fatal("ParsePOM con archivo inexistente debería fallar")
	}
}

func TestParsePOMInvalidXML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "roto.pom")
	writeFile(t, path, "<project><groupId>sin cerrar")
	if _, err := ParsePOM(path); err == nil {
		t.Fatal("ParsePOM con XML inválido debería fallar")
	}
}

func TestResolveProperties(t *testing.T) {
	props := map[string]string{
		"kotlin.version": "2.4.21",
		"indirecta":      "${kotlin.version}",
	}
	tests := []struct {
		in   string
		want string
	}{
		{"${kotlin.version}", "2.4.21"},
		{"org.jetbrains.kotlin:kotlin-stdlib:${kotlin.version}", "org.jetbrains.kotlin:kotlin-stdlib:2.4.21"},
		{"${indirecta}", "2.4.21"},
		{"sin placeholders", "sin placeholders"},
		{"", ""},
		{"${no.existe}", "${no.existe}"},
	}
	for _, tc := range tests {
		if got := ResolveProperties(tc.in, props); got != tc.want {
			t.Errorf("ResolveProperties(%q) = %q, quiero %q", tc.in, got, tc.want)
		}
	}
}

func TestResolveDependencies(t *testing.T) {
	props := map[string]string{"x.version": "9.9"}
	mgmt := map[string]string{"androidx.core:core": "1.12.0"}

	t.Run("versión gestionada", func(t *testing.T) {
		p := &Project{Dependencies: []Dependency{
			{GroupID: "androidx.core", ArtifactID: "core"},
		}}
		deps, err := ResolveDependencies(p, mgmt, props)
		if err != nil {
			t.Fatalf("ResolveDependencies: %v", err)
		}
		if len(deps) != 1 || deps[0].Version != "1.12.0" {
			t.Errorf("deps = %+v, quiero versión 1.12.0 desde management", deps)
		}
	})

	t.Run("propiedad en la versión", func(t *testing.T) {
		p := &Project{Dependencies: []Dependency{
			{GroupID: "com.x", ArtifactID: "y", Version: "${x.version}"},
		}}
		deps, err := ResolveDependencies(p, nil, props)
		if err != nil {
			t.Fatalf("ResolveDependencies: %v", err)
		}
		if deps[0].Version != "9.9" {
			t.Errorf("versión = %q, quiero 9.9", deps[0].Version)
		}
	})

	t.Run("rango de versión se colapsa", func(t *testing.T) {
		p := &Project{Dependencies: []Dependency{
			{GroupID: "com.x", ArtifactID: "y", Version: "[1.0,2.0)"},
		}}
		deps, err := ResolveDependencies(p, nil, props)
		if err != nil {
			t.Fatalf("ResolveDependencies: %v", err)
		}
		if deps[0].Version != "1.0" {
			t.Errorf("versión = %q, quiero 1.0", deps[0].Version)
		}
	})

	t.Run("sin versión ni management falla", func(t *testing.T) {
		p := &Project{Dependencies: []Dependency{
			{GroupID: "com.x", ArtifactID: "sin-version"},
		}}
		if _, err := ResolveDependencies(p, mgmt, props); err == nil {
			t.Fatal("debería fallar sin versión ni management")
		}
	})

	t.Run("rango sin límite inferior falla", func(t *testing.T) {
		p := &Project{Dependencies: []Dependency{
			{GroupID: "com.x", ArtifactID: "y", Version: "(1.0,2.0]"},
		}}
		if _, err := ResolveDependencies(p, nil, props); err == nil {
			t.Fatal("debería fallar con rango sin límite inferior")
		}
	})
}

func TestMergeProperties(t *testing.T) {
	// cadena: [hijo, padre, raíz]
	chain := []*Project{
		{Properties: []Property{{XMLName: xmlName("a"), Value: "hijo"}, {XMLName: xmlName("solo-hijo"), Value: "x"}}},
		{Properties: []Property{{XMLName: xmlName("a"), Value: "padre"}}},
		{Properties: []Property{{XMLName: xmlName("a"), Value: "raiz"}, {XMLName: xmlName("solo-raiz"), Value: "y"}}},
	}
	props := MergeProperties(chain)
	if props["a"] != "hijo" {
		t.Errorf("a = %q, el hijo debe ganar", props["a"])
	}
	if props["solo-hijo"] != "x" || props["solo-raiz"] != "y" {
		t.Errorf("props = %v, faltan claves heredadas", props)
	}
}

func TestMergeDependencyManagement(t *testing.T) {
	chain := []*Project{
		{DependencyManagement: &DepMgmt{Dependencies: []Dependency{
			{GroupID: "g", ArtifactID: "a", Version: "2.0"},
			{GroupID: "g", ArtifactID: "sin-version"},
		}}},
		{DependencyManagement: &DepMgmt{Dependencies: []Dependency{
			{GroupID: "g", ArtifactID: "a", Version: "1.0"},
			{GroupID: "g", ArtifactID: "b", Version: "3.0"},
		}}},
	}
	mgmt := MergeDependencyManagement(chain)
	if mgmt["g:a"] != "2.0" {
		t.Errorf("g:a = %q, el hijo debe ganar", mgmt["g:a"])
	}
	if mgmt["g:b"] != "3.0" {
		t.Errorf("g:b = %q, se hereda del padre", mgmt["g:b"])
	}
	if _, ok := mgmt["g:sin-version"]; ok {
		t.Error("no debería registrarse una entrada sin versión")
	}
}

func TestDownloadPOMUsesCache(t *testing.T) {
	dir := t.TempDir()
	// Pre-poblar la caché con el nombre que usa DownloadPOM.
	path := filepath.Join(dir, "demo-1.0.pom")
	writeFile(t, path, "<project/>")

	got, err := DownloadPOM("com.demo:demo:1.0", dir)
	if err != nil {
		t.Fatalf("DownloadPOM desde caché: %v", err)
	}
	if got != path {
		t.Errorf("DownloadPOM = %q, quiero %q", got, path)
	}
}

func TestDownloadPOMInvalidCoordinate(t *testing.T) {
	if _, err := DownloadPOM("solo:dos", t.TempDir()); err == nil {
		t.Fatal("DownloadPOM con coordenada inválida debería fallar")
	}
}

func TestDownloadPOMFromServer(t *testing.T) {
	dir := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/com/demo/demo/1.0/demo-1.0.pom" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("<project><artifactId>demo</artifactId></project>"))
	}))
	defer server.Close()

	// Sustituye temporalmente la lista de repositorios.
	original := repositories
	repositories = []string{server.URL}
	t.Cleanup(func() { repositories = original })

	path, err := DownloadPOM("com.demo:demo:1.0", dir)
	if err != nil {
		t.Fatalf("DownloadPOM: %v", err)
	}
	p, err := ParsePOM(path)
	if err != nil {
		t.Fatalf("ParsePOM: %v", err)
	}
	if p.ArtifactID != "demo" {
		t.Errorf("ArtifactID = %q, quiero demo", p.ArtifactID)
	}
}

func TestDownloadPOMNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	original := repositories
	repositories = []string{server.URL}
	t.Cleanup(func() { repositories = original })

	if _, err := DownloadPOM("com.demo:ausente:1.0", t.TempDir()); err == nil {
		t.Fatal("DownloadPOM debería fallar si el servidor devuelve 404")
	}
}

func TestNeedFetch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo-1.0.pom")

	// No existe: hay que descargarlo.
	if !needFetch(path) {
		t.Error("needFetch = false para un fichero inexistente")
	}

	writeFile(t, path, "<project/>")
	if needFetch(path) {
		t.Error("needFetch = true para un fichero ya cacheado")
	}
}

func TestDownloadPOMEsAtomico(t *testing.T) {
	dir := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<project><artifactId>demo</artifactId></project>"))
	}))
	defer server.Close()

	original := repositories
	repositories = []string{server.URL}
	t.Cleanup(func() { repositories = original })

	path, err := DownloadPOM("com.demo:demo:1.0", dir)
	if err != nil {
		t.Fatalf("DownloadPOM: %v", err)
	}

	// El .part temporal no debe quedarse en la caché.
	if _, err := os.Stat(path + ".part"); err == nil {
		t.Error("quedó un fichero .part tras la descarga")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("el POM final no existe: %v", err)
	}
}

func TestVersionLess(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"1.0", "2.0", true},
		{"2.0", "1.0", false},
		{"1.0", "1.0", false},
		{"1.2", "1.10", true},
		{"1.10", "1.2", false},
		{"1.0.0", "1.0", false},
		{"1.0", "1.0.1", true},
		{"1.0.0-alpha", "1.0.0-beta", true},
	}
	for _, tc := range tests {
		if got := versionLess(tc.a, tc.b); got != tc.want {
			t.Errorf("versionLess(%q, %q) = %v, quiero %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestModuleKey(t *testing.T) {
	tests := []struct {
		group, artifact, want string
	}{
		{"androidx.lifecycle", "lifecycle-runtime", "androidx.lifecycle:lifecycle-runtime"},
		{"androidx.lifecycle", "lifecycle-runtime-android", "androidx.lifecycle:lifecycle-runtime"},
		{"androidx.lifecycle", "lifecycle-runtime-jvm", "androidx.lifecycle:lifecycle-runtime"},
		// -ktx es un módulo distinto (aporta clases propias).
		{"androidx.lifecycle", "lifecycle-runtime-ktx", "androidx.lifecycle:lifecycle-runtime-ktx"},
		{"androidx.lifecycle", "lifecycle-runtime-ktx-android", "androidx.lifecycle:lifecycle-runtime-ktx"},
		// Un nombre que no es variante no se toca.
		{"com.example", "myandroid", "com.example:myandroid"},
	}
	for _, tc := range tests {
		if got := moduleKey(tc.group, tc.artifact); got != tc.want {
			t.Errorf("moduleKey(%q, %q) = %q, quiero %q", tc.group, tc.artifact, got, tc.want)
		}
	}
}

func TestResolvedDepCoordinate(t *testing.T) {
	dep := ResolvedDep{GroupID: "androidx.core", ArtifactID: "core", Version: "1.12.0"}
	if got := dep.Coordinate(); got != "androidx.core:core:1.12.0" {
		t.Errorf("Coordinate = %q", got)
	}
}

func TestResolveParentChainFromCache(t *testing.T) {
	cacheDir := t.TempDir()

	// Parent en caché: com.padre:base:1.0
	parentGroupDir := filepath.Join(cacheDir, "com.padre")
	if err := os.MkdirAll(parentGroupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(parentGroupDir, "base-1.0.pom"), `<project>
  <groupId>com.padre</groupId><artifactId>base</artifactId><version>1.0</version>
  <properties><a>padre</a></properties>
</project>`)

	child := &Project{
		GroupID: "com.hijo", ArtifactID: "app", Version: "1",
		Parent:     &Parent{GroupID: "com.padre", ArtifactID: "base", Version: "1.0"},
		Properties: []Property{{XMLName: xmlName("b"), Value: "hijo"}},
	}

	chain, err := ResolveParentChain(child, cacheDir)
	if err != nil {
		t.Fatalf("ResolveParentChain: %v", err)
	}
	if len(chain) != 2 {
		t.Fatalf("len(chain) = %d, quiero 2", len(chain))
	}
	if chain[0].ArtifactID != "app" || chain[1].ArtifactID != "base" {
		t.Errorf("orden = %q, %q (hijo primero)", chain[0].ArtifactID, chain[1].ArtifactID)
	}

	props := MergeProperties(chain)
	if props["a"] != "padre" || props["b"] != "hijo" {
		t.Errorf("props fusionadas = %v", props)
	}
}

func xmlName(local string) xml.Name {
	return xml.Name{Local: local}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDedupeCoordinates(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "highest wins conserva orden",
			in: []string{
				"androidx.core:core:1.7.0",
				"androidx.lifecycle:lifecycle-runtime:2.3.1",
				"androidx.core:core:1.13.1",
				"androidx.lifecycle:lifecycle-runtime:2.6.2",
				"androidx.viewpager2:viewpager2:1.1.0",
			},
			want: []string{
				"androidx.core:core:1.13.1",
				"androidx.lifecycle:lifecycle-runtime:2.6.2",
				"androidx.viewpager2:viewpager2:1.1.0",
			},
		},
		{
			name: "variantes -android/-jvm colapsan",
			in: []string{
				"androidx.lifecycle:lifecycle-runtime-android:2.6.1",
				"androidx.lifecycle:lifecycle-runtime-jvm:2.6.2",
			},
			want: []string{
				"androidx.lifecycle:lifecycle-runtime-jvm:2.6.2",
			},
		},
		{
			name: "formato inesperado se conserva",
			in:   []string{"paquete-raro", "androidx.core:core:1.9.0"},
			want: []string{"paquete-raro", "androidx.core:core:1.9.0"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DedupeCoordinates(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("DedupeCoordinates() = %v, quiero %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("[%d] = %q, quiero %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}
