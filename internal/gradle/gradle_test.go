package gradle

import (
	"os"
	"path/filepath"
	"testing"
)

// writeProject crea un proyecto Gradle mínimo en un directorio temporal.
func writeProject(t *testing.T, settings string, buildFile string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings.gradle.kts"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}
	appDir := filepath.Join(dir, "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "build.gradle.kts"), []byte(buildFile), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestDetectReadsAppModule(t *testing.T) {
	dir := writeProject(t,
		`rootProject.name = "Smart Island"
include(":app")
include(":baselineprofile")`,
		`plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.compose")
    id("org.jetbrains.kotlin.kapt")
}

android {
    namespace = "com.agupta07505.smartisland"
    compileSdk = 36

    defaultConfig {
        applicationId = "com.agupta07505.smartisland"
        minSdk = 26
        targetSdk = 36
        versionCode = 9
        versionName = "7.6.0"
    }

    buildFeatures {
        compose = true
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
    }

    kotlin {
        compilerOptions {
            jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17)
        }
    }
}

dependencies {
    val composeBom = platform("androidx.compose:compose-bom:2026.02.01")

    implementation(composeBom)
    implementation("androidx.compose.ui:ui")
    implementation("androidx.core:core-ktx:1.18.0")
    "baselineProfile"(project(":baselineprofile"))
    implementation("com.google.dagger:hilt-android:2.60.1")
    kapt("com.google.dagger:hilt-compiler:2.60.1")

    debugImplementation("androidx.compose.ui:ui-tooling")

    testImplementation("junit:junit:4.13.2")
    androidTestImplementation("androidx.test.ext:junit:1.2.1")
}`)

	proj, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if proj.Name != "Smart Island" {
		t.Errorf("Name = %q, se esperaba %q", proj.Name, "Smart Island")
	}
	if len(proj.Modules) != 2 {
		t.Fatalf("Modules = %d, se esperaban 2", len(proj.Modules))
	}
	app := proj.App
	if app == nil {
		t.Fatal("App = nil")
	}
	if app.Namespace != "com.agupta07505.smartisland" {
		t.Errorf("Namespace = %q", app.Namespace)
	}
	if app.ApplicationID != "com.agupta07505.smartisland" {
		t.Errorf("ApplicationID = %q", app.ApplicationID)
	}
	if app.CompileSdk != 36 {
		t.Errorf("CompileSdk = %d, se esperaba 36", app.CompileSdk)
	}
	if app.MinSdk != 26 || app.TargetSdk != 36 {
		t.Errorf("MinSdk/TargetSdk = %d/%d, se esperaban 26/36", app.MinSdk, app.TargetSdk)
	}
	if app.VersionCode != 9 || app.VersionName != "7.6.0" {
		t.Errorf("Version = %d/%q, se esperaban 9/7.6.0", app.VersionCode, app.VersionName)
	}
	if app.JavaTarget != 17 {
		t.Errorf("JavaTarget = %d, se esperaba 17", app.JavaTarget)
	}

	// Dependencias: se ignoran las de test.
	if len(app.Dependencies) != 4 {
		t.Errorf("Dependencies = %d, se esperaban 4: %+v", len(app.Dependencies), app.Dependencies)
	}
	for _, d := range app.Dependencies {
		if d.Scope == "testImplementation" || d.Scope == "androidTestImplementation" {
			t.Errorf("no debería estar la dependencia de test %q", d.Coordinate)
		}
	}

	// Las de debug van aparte: en un release no están en el classpath.
	if len(app.DebugDependencies) != 1 ||
		app.DebugDependencies[0].Coordinate != "androidx.compose.ui:ui-tooling" {
		t.Errorf("DebugDependencies = %+v", app.DebugDependencies)
	}

	// El BOM es una platform, no una dependencia normal.
	var sawPlatform bool
	for _, d := range app.Dependencies {
		if d.Platform {
			sawPlatform = true
			if d.Coordinate != "androidx.compose:compose-bom:2026.02.01" {
				t.Errorf("BOM = %q", d.Coordinate)
			}
		}
	}
	if !sawPlatform {
		t.Error("no se detectó el BOM de Compose")
	}

	// Los annotation processors van aparte.
	if len(app.Kapt) != 1 {
		t.Fatalf("Kapt = %d, se esperaba 1", len(app.Kapt))
	}
	if app.Kapt[0].Coordinate != "com.google.dagger:hilt-compiler:2.60.1" {
		t.Errorf("processor = %q", app.Kapt[0].Coordinate)
	}

	// La dependencia de proyecto se registra como tal.
	if len(app.LocalProjects) != 1 || app.LocalProjects[0] != ":baselineprofile" {
		t.Errorf("LocalProjects = %v", app.LocalProjects)
	}
}

func TestParseDependenciesIgnoresTestScopes(t *testing.T) {
	sets := parseDependencies(`
    implementation("a:b:1.0")
    testImplementation("junit:junit:4.13.2")
    androidTestImplementation("androidx.test.ext:junit:1.2.1")
    kapt("com.google.dagger:hilt-compiler:2.60.1")
    "baselineProfile"(project(":baselineprofile"))
`)

	if len(sets.Common) != 1 || sets.Common[0].Coordinate != "a:b:1.0" {
		t.Errorf("Common = %+v", sets.Common)
	}
	if len(sets.Kapt) != 1 {
		t.Errorf("Kapt = %+v", sets.Kapt)
	}
	if len(sets.Local) != 1 || sets.Local[0] != ":baselineprofile" {
		t.Errorf("Local = %v", sets.Local)
	}
}

// TestParseDependenciesSeparatesVariants comprueba que las dependencias de
// debug y de release no se mezclan con las comunes: en Gradle,
// debugImplementation solo está en la variante debug.
func TestParseDependenciesSeparatesVariants(t *testing.T) {
	sets := parseDependencies(`
    implementation("a:b:1.0")
    debugImplementation("androidx.compose.ui:ui-tooling")
    releaseImplementation("com.example:only-in-release:1.0")
`)

	if len(sets.Common) != 1 || sets.Common[0].Coordinate != "a:b:1.0" {
		t.Errorf("Common = %+v", sets.Common)
	}
	if len(sets.Debug) != 1 || sets.Debug[0].Coordinate != "androidx.compose.ui:ui-tooling" {
		t.Errorf("Debug = %+v", sets.Debug)
	}
	if len(sets.Release) != 1 || sets.Release[0].Coordinate != "com.example:only-in-release:1.0" {
		t.Errorf("Release = %+v", sets.Release)
	}
}

// TestParseProguardFilesIgnoresSDKDefaults comprueba que solo se importan las
// reglas que están en el proyecto: las que trae getDefaultProguardFile viven
// en el SDK y rain usa las suyas.
func TestParseProguardFilesIgnoresSDKDefaults(t *testing.T) {
	files := parseProguardFiles(`
        proguardFiles(
            getDefaultProguardFile("proguard-android-optimize.txt"),
            "proguard-rules.pro"
        )
`)
	if len(files) != 1 || files[0] != "proguard-rules.pro" {
		t.Errorf("files = %v", files)
	}
}

func TestStripCommentsKeepsStrings(t *testing.T) {
	src := `// comentario
val x = "no // soy un comentario" /* otro */ val y = 1`
	out := stripComments(src)
	if got := stringValue(out, "x"); got != "no // soy un comentario" {
		t.Errorf("stringValue(x) = %q", got)
	}
	if got := intValue(out, "y"); got != 1 {
		t.Errorf("intValue(y) = %d", got)
	}
}

func TestFindBlockIgnoresBracesInStrings(t *testing.T) {
	src := `dependencies {
    implementation("a:b:1.0")
}`
	body, ok := findBlock(src, "dependencies")
	if !ok {
		t.Fatal("no se encontró el bloque dependencies")
	}
	if got := stringValue(body, "implementation"); got != "a:b:1.0" {
		t.Errorf("implementation = %q", got)
	}
}
