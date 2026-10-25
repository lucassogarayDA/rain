package build

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/itskreisler/rain/internal/aar"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSourceRootPrefersGradleLayout(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "main", "AndroidManifest.xml"), "<manifest/>")

	if got := sourceRoot(dir); got != filepath.Join(dir, "src", "main") {
		t.Errorf("sourceRoot = %q, quiero el layout Gradle", got)
	}
	if got := manifestPath(dir); got != filepath.Join(dir, "src", "main", "AndroidManifest.xml") {
		t.Errorf("manifestPath = %q", got)
	}
}

func TestSourceRootFallsBackToFlatLayout(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "AndroidManifest.xml"), "<manifest/>")

	if got := sourceRoot(dir); got != dir {
		t.Errorf("sourceRoot = %q, quiero %q (layout plano)", got, dir)
	}
	if got := resPath(dir); got != filepath.Join(dir, "res") {
		t.Errorf("resPath = %q", got)
	}
}

func TestPackageFromManifest(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		want     string
	}{
		{
			name:     "package simple",
			manifest: `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.klei.hello">`,
			want:     "com.klei.hello",
		},
		{
			name:     "package con espacios",
			manifest: `<manifest package = "com.demo.app" >`,
			want:     "",
		},
		{
			name:     "sin package",
			manifest: `<manifest xmlns:android="http://schemas.android.com/apk/res/android">`,
			want:     "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "AndroidManifest.xml"), tc.manifest)
			if got := PackageFromManifest(dir); got != tc.want {
				t.Errorf("PackageFromManifest = %q, quiero %q", got, tc.want)
			}
		})
	}
}

func TestPackageFromManifestMissingFile(t *testing.T) {
	if got := PackageFromManifest(t.TempDir()); got != "" {
		t.Errorf("PackageFromManifest sin manifest = %q, quiero vacío", got)
	}
}

func TestSanitizeTag(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Mi App", "Mi-App"},
		{"Notas", "Notas"},
		{"app/compose", "app-compose"},
		{"  v1.2.3  ", "v1.2.3"},
		{"", ""},
		{"---", ""},
		{"ñandú", "and"},
	}
	for _, tc := range tests {
		if got := sanitizeTag(tc.in); got != tc.want {
			t.Errorf("sanitizeTag(%q) = %q, quiero %q", tc.in, got, tc.want)
		}
	}
}

func TestDetectABI(t *testing.T) {
	t.Run("sin lib es universal", func(t *testing.T) {
		if got := detectABI(t.TempDir()); got != "universal" {
			t.Errorf("detectABI = %q, quiero universal", got)
		}
	})

	t.Run("una abi", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "lib", "arm64-v8a", "libfoo.so"), "x")
		if got := detectABI(dir); got != "arm64-v8a" {
			t.Errorf("detectABI = %q, quiero arm64-v8a", got)
		}
	})

	t.Run("varias abis ordenadas", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "lib", "x86_64", "libfoo.so"), "x")
		writeFile(t, filepath.Join(dir, "lib", "arm64-v8a", "libfoo.so"), "x")
		if got := detectABI(dir); got != "arm64-v8a_x86_64" {
			t.Errorf("detectABI = %q, quiero arm64-v8a_x86_64", got)
		}
	})

	t.Run("las cuatro abis es universal", func(t *testing.T) {
		dir := t.TempDir()
		for _, abi := range []string{"arm64-v8a", "armeabi-v7a", "x86", "x86_64"} {
			writeFile(t, filepath.Join(dir, "lib", abi, "libfoo.so"), "x")
		}
		if got := detectABI(dir); got != "universal" {
			t.Errorf("detectABI = %q, quiero universal", got)
		}
	})

	t.Run("directorio abi sin .so se ignora", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "lib", "arm64-v8a"), 0o755); err != nil {
			t.Fatal(err)
		}
		if got := detectABI(dir); got != "universal" {
			t.Errorf("detectABI = %q, quiero universal", got)
		}
	})
}

func TestApkNameFor(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "nombre y versión",
			cfg:  Config{AppName: "Notas", VersionName: "1.0", AppDir: t.TempDir()},
			want: "Notas-1.0-universal.apk",
		},
		{
			name: "sin nombre usa el directorio",
			cfg:  Config{VersionName: "2.0", AppDir: filepath.Join(t.TempDir(), "MiProyecto")},
			want: "MiProyecto-2.0-universal.apk",
		},
		{
			name: "sin versión usa 1.0",
			cfg:  Config{AppName: "App", AppDir: t.TempDir()},
			want: "App-1.0-universal.apk",
		},
		{
			name: "saneado de espacios",
			cfg:  Config{AppName: "Mi App Grande", VersionName: "1.0 beta", AppDir: t.TempDir()},
			want: "Mi-App-Grande-1.0-beta-universal.apk",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := apkNameFor(tc.cfg); got != tc.want {
				t.Errorf("apkNameFor = %q, quiero %q", got, tc.want)
			}
		})
	}
}

func TestDexNumber(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{"/x/classes.dex", 1},
		{"/x/classes2.dex", 2},
		{"/x/classes10.dex", 10},
		{"/x/raro.dex", 1 << 30},
	}
	for _, tc := range tests {
		if got := dexNumber(tc.path); got != tc.want {
			t.Errorf("dexNumber(%q) = %d, quiero %d", tc.path, got, tc.want)
		}
	}
}

func TestAppendDexPreservesEntries(t *testing.T) {
	dir := t.TempDir()
	baseApk := filepath.Join(dir, "base.apk")

	// APK base con una entrada que NO debe perderse (resources.arsc).
	writeZip(t, baseApk, map[string]string{
		"AndroidManifest.xml": "manifest",
		"resources.arsc":      "recursos",
	})

	dex1 := filepath.Join(dir, "classes.dex")
	dex2 := filepath.Join(dir, "classes2.dex")
	writeFile(t, dex1, "dex-uno")
	writeFile(t, dex2, "dex-dos")

	if err := appendDex(baseApk, []string{dex1, dex2}, nil, nil); err != nil {
		t.Fatalf("appendDex: %v", err)
	}

	got := readZip(t, baseApk)
	if got["resources.arsc"] != "recursos" {
		t.Errorf("resources.arsc = %q, se perdió al empaquetar", got["resources.arsc"])
	}
	if got["AndroidManifest.xml"] != "manifest" {
		t.Errorf("AndroidManifest.xml = %q", got["AndroidManifest.xml"])
	}
	if got["classes.dex"] != "dex-uno" {
		t.Errorf("classes.dex = %q, quiero dex-uno", got["classes.dex"])
	}
	if got["classes2.dex"] != "dex-dos" {
		t.Errorf("classes2.dex = %q, quiero dex-dos", got["classes2.dex"])
	}
}

func writeZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	w := zip.NewWriter(f)
	for name, content := range entries {
		entry, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDedupeJarsByClassNoOverlap(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.jar")
	b := filepath.Join(dir, "b.jar")
	writeZip(t, a, map[string]string{"com/x/A.class": "a"})
	writeZip(t, b, map[string]string{"com/x/B.class": "b"})

	got := dedupeJarsByClass([]string{a, b}, filepath.Join(dir, "scratch"))
	if len(got) != 2 {
		t.Fatalf("jars = %v, quiero 2 sin solapes", got)
	}
	// Sin solapes no se reempaqueta: se conservan las rutas originales.
	if got[0] != a || got[1] != b {
		t.Errorf("got = %v, quiero originales sin tocar", got)
	}
}

func TestDedupeJarsByClassFullDuplicateDropped(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.jar")
	b := filepath.Join(dir, "b.jar")
	writeZip(t, a, map[string]string{"com/x/A.class": "new"})
	writeZip(t, b, map[string]string{"com/x/A.class": "old"}) // idéntico

	got := dedupeJarsByClass([]string{a, b}, filepath.Join(dir, "scratch"))
	if len(got) != 1 || got[0] != a {
		t.Fatalf("got = %v, quiero solo el primero", got)
	}
}

func TestDedupeJarsByClassPartialOverlapStripped(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.jar")
	b := filepath.Join(dir, "b.jar")
	writeZip(t, a, map[string]string{"com/x/A.class": "a"})
	// b comparte A con a pero aporta B y un recurso embebido.
	writeZip(t, b, map[string]string{
		"com/x/A.class":        "dup",
		"com/x/B.class":        "b",
		"META-INF/MANIFEST.MF": "manifest",
	})

	scratch := filepath.Join(dir, "scratch")
	got := dedupeJarsByClass([]string{a, b}, scratch)
	if len(got) != 2 {
		t.Fatalf("got = %v, quiero 2 (b filtrado)", got)
	}
	if got[0] != a {
		t.Errorf("got[0] = %q, quiero el original a", got[0])
	}

	// El jar filtrado debe conservar B y el recurso, pero no la clase duplicada.
	zr, err := zip.OpenReader(got[1])
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	names := make(map[string]bool)
	for _, f := range zr.File {
		names[f.Name] = true
	}
	if names["com/x/A.class"] {
		t.Error("la clase duplicada A debería haberse eliminado")
	}
	if !names["com/x/B.class"] {
		t.Error("falta la clase única B")
	}
	if !names["META-INF/MANIFEST.MF"] {
		t.Error("falta el recurso embebido")
	}
}

func TestDedupeJarsByClassUnreadableKept(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.jar")
	if err := os.WriteFile(bad, []byte("no soy zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := dedupeJarsByClass([]string{bad}, filepath.Join(dir, "scratch"))
	if len(got) != 1 || got[0] != bad {
		t.Errorf("got = %v, un jar ilegible se conserva", got)
	}
}

func TestAppendDexDescartaDexSobrantesDelAPKBase(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.apk")

	// APK base de un build anterior con más DEX de los que genera este build.
	f, err := os.Create(base)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for _, name := range []string{"AndroidManifest.xml", "resources.arsc", "classes.dex", "classes9.dex"} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("viejo")); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	dex := filepath.Join(dir, "classes.dex")
	writeFile(t, dex, "nuevo")
	if err := appendDex(base, []string{dex}, nil, nil); err != nil {
		t.Fatal(err)
	}

	zr, err := zip.OpenReader(base)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()

	names := make(map[string]string)
	for _, entry := range zr.File {
		names[entry.Name] = ""
	}
	if _, ok := names["classes9.dex"]; ok {
		t.Error("el APK conserva classes9.dex de un build anterior")
	}
	if _, ok := names["classes.dex"]; !ok {
		t.Error("el APK no contiene el classes.dex actual")
	}
	if _, ok := names["AndroidManifest.xml"]; !ok {
		t.Error("el APK perdió el manifest")
	}
}

func readZip(t *testing.T, path string) map[string]string {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()

	out := make(map[string]string)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(rc); err != nil {
			t.Fatal(err)
		}
		_ = rc.Close()
		out[f.Name] = buf.String()
	}
	return out
}

func TestUsesComposeDetectsImports(t *testing.T) {
	dir := t.TempDir()

	// Kotlin puro, sin Compose.
	writeFile(t, filepath.Join(dir, "app", "java", "com", "x", "Main.kt"),
		"package com.x\n\nimport android.os.Bundle\n\nclass Main\n")

	cfg := Config{AppDir: filepath.Join(dir, "app")}
	if cfg.usesCompose() {
		t.Error("usesCompose() = true sin imports de androidx.compose")
	}

	// Fuente con import de Compose.
	writeFile(t, filepath.Join(dir, "app", "java", "com", "x", "Ui.kt"),
		"package com.x\n\nimport androidx.compose.runtime.Composable\n\n@Composable fun Ui() {}\n")

	if !cfg.usesCompose() {
		t.Error("usesCompose() = false con import de androidx.compose")
	}
}

func TestAppliesComposePluginRequiresPluginAndUsage(t *testing.T) {
	dir := t.TempDir()
	plugin := filepath.Join(dir, "compose-compiler-plugin.jar")
	writeFile(t, plugin, "plugin")

	app := filepath.Join(dir, "app")

	// Sin fuentes Compose: no se aplica aunque el plugin exista.
	writeFile(t, filepath.Join(app, "java", "com", "x", "Main.kt"), "package com.x\nclass Main\n")
	cfg := Config{AppDir: app, ComposePlugin: plugin}
	if cfg.appliesComposePlugin() {
		t.Error("appliesComposePlugin() = true con Kotlin sin Compose")
	}

	// Con Compose y plugin presente: se aplica.
	writeFile(t, filepath.Join(app, "java", "com", "x", "Ui.kt"),
		"package com.x\nimport androidx.compose.runtime.Composable\n@Composable fun Ui() {}\n")
	if !cfg.appliesComposePlugin() {
		t.Error("appliesComposePlugin() = false con Compose y plugin presente")
	}
}

func TestJVMTargetPorDefectoEs17(t *testing.T) {
	// AGP moderno compila a Java 17: ese es el nivel por defecto de Rain,
	// tanto para proyectos Java como para los de Compose.
	cfg := Config{}
	if got := cfg.jvmTarget(); got != 17 {
		t.Errorf("jvmTarget() = %d, quiero 17 por defecto", got)
	}

	// java_target explícito manda.
	cfg = Config{JVMTarget: 8}
	if got := cfg.jvmTarget(); got != 8 {
		t.Errorf("jvmTarget() = %d, quiero 8 con java_target=8", got)
	}
}

func TestValidateJVMTarget(t *testing.T) {
	for _, valid := range []int{0, 8, 11, 17, 21} {
		if err := ValidateJVMTarget(valid); err != nil {
			t.Errorf("ValidateJVMTarget(%d) = %v, quiero nil", valid, err)
		}
	}
	for _, invalid := range []int{1, 9, 16, 25, 100} {
		if err := ValidateJVMTarget(invalid); err == nil {
			t.Errorf("ValidateJVMTarget(%d) = nil, quiero error", invalid)
		}
	}
}

func TestProguardFilesIncludesUserRulesAndLibs(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "app")

	// Reglas del proyecto (layout plano).
	writeFile(t, filepath.Join(app, "proguard-rules.pro"), "-keep class com.x.** { *; }\n")

	cfg := Config{
		AppDir:        app,
		ProguardFiles: []string{"/extra/user.pro"},
		Libs: &aar.Set{Libraries: []aar.Library{
			{Coordinate: "a:a:1", ProguardRules: "/deps/a.pro"},
		}},
	}

	got := cfg.proguardFiles()
	want := []string{
		filepath.Join(app, "proguard-rules.pro"),
		"/extra/user.pro",
		"/deps/a.pro",
	}
	if len(got) != len(want) {
		t.Fatalf("proguardFiles() = %v, quiero %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("proguardFiles()[%d] = %q, quiero %q", i, got[i], want[i])
		}
	}
}

func TestProguardFilesDedupesAndSkipsMissing(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		AppDir:        dir, // sin proguard-rules.pro
		ProguardFiles: []string{"/a.pro", "/a.pro", ""},
	}
	got := cfg.proguardFiles()
	if len(got) != 1 || got[0] != "/a.pro" {
		t.Errorf("proguardFiles() = %v, quiero [/a.pro]", got)
	}
}

func TestDefaultProguardRulesKeepsFrameworkEntryPoints(t *testing.T) {
	for _, want := range []string{"android.app.Activity", "android.app.Service", "kotlin.Metadata"} {
		if !strings.Contains(defaultProguardRules, want) {
			t.Errorf("defaultProguardRules no menciona %q", want)
		}
	}
}

func TestDefaultProguardRulesDontWarnOptionalAndroidX(t *testing.T) {
	// R8 valida clases ausentes (a diferencia de d8): las deps de AndroidX
	// referencian clases de artefactos opcionales o de plataformas nuevas. Sin
	// estos -dontwarn, un release con Compose aborta con "Missing class".
	for _, want := range []string{
		"-dontwarn androidx.emoji2.**",
		"-dontwarn androidx.window.**",
	} {
		if !strings.Contains(defaultProguardRules, want) {
			t.Errorf("defaultProguardRules no incluye %q", want)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{12345678, "11.8 MiB"},
	}
	for _, c := range cases {
		if got := formatBytes(c.n); got != c.want {
			t.Errorf("formatBytes(%d) = %q, quiero %q", c.n, got, c.want)
		}
	}
}

func TestR8LibraryJarsAlwaysIncludesAndroidJar(t *testing.T) {
	cfg := Config{AndroidJar: "/sdk/android.jar"}
	jars := cfg.r8LibraryJars(nil)
	if len(jars) == 0 || jars[0] != "/sdk/android.jar" {
		t.Fatalf("r8LibraryJars() = %v, quiero que empiece por android.jar", jars)
	}
}

func TestDefaultProguardRulesDontWarnJetbrainsAnnotations(t *testing.T) {
	if !strings.Contains(defaultProguardRules, "-dontwarn org.jetbrains.annotations.**") {
		t.Error("defaultProguardRules no incluye -dontwarn org.jetbrains.annotations.**")
	}
}
