// Package gradle lee la configuración de un proyecto Gradle (Kotlin DSL) para
// convertirla en un rain.toml.
//
// El objetivo no es reimplementar Gradle, sino leer lo justo que hace falta
// para compilar el proyecto con rain: el namespace, los niveles de API, las
// versiones y las dependencias (incluidos los BOM y los annotation processors
// de kapt).
package gradle

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Project es el resultado de analizar un proyecto Gradle.
type Project struct {
	// RootDir es el directorio donde está settings.gradle.kts.
	RootDir string
	// Name es el nombre del proyecto (rootProject.name).
	Name string
	// Modules son los módulos declarados con include(...) en settings.
	Modules []Module
	// App es el módulo de aplicación (el que aplica el plugin
	// com.android.application), si lo hay.
	App *Module
	// Warnings recoge lo que no se ha podido traducir al rain.toml.
	Warnings []string
}

// Module es un módulo de Gradle (un subproyecto).
type Module struct {
	// Name es el nombre del módulo con su prefijo de Gradle (":app").
	Name string
	// Path es la ruta del módulo relativa a la raíz del proyecto ("app").
	Path string
	// Plugins son los ids de los plugins que aplica ("com.android.application").
	Plugins []string
	// Namespace es el namespace de Android (el paquete del código).
	Namespace string
	// ApplicationID es el id de la app en el dispositivo.
	ApplicationID string
	// CompileSdk, MinSdk y TargetSdk son los niveles de API.
	CompileSdk int
	MinSdk     int
	TargetSdk  int
	// VersionCode y VersionName son la versión de la app.
	VersionCode int
	VersionName string
	// JavaTarget es el nivel de bytecode del código Java/Kotlin.
	JavaTarget int
	// Dependencies son las dependencias de compilación y runtime.
	Dependencies []Dependency
	// DebugDependencies son las que solo van en la variante debug
	// (debugImplementation). En un release no están en el classpath.
	DebugDependencies []Dependency
	// ReleaseDependencies son las que solo van en la variante release
	// (releaseImplementation).
	ReleaseDependencies []Dependency
	// Kapt son los annotation processors (kapt).
	Kapt []Dependency
	// LocalProjects son las dependencias que apuntan a otro módulo del
	// proyecto ("project(":baselineprofile")").
	LocalProjects []string
	// ProguardFiles son las reglas de R8/ProGuard del módulo, con la ruta
	// relativa a la raíz del proyecto ("app/proguard-rules.pro").
	ProguardFiles []string
}

// Dependency es una dependencia declarada en el bloque dependencies.
type Dependency struct {
	// Scope es la configuración de Gradle (implementation, api, kapt...).
	Scope string
	// Coordinate es "grupo:artifacto" o "grupo:artifacto:version". La
	// versión puede faltar si la fija un BOM.
	Coordinate string
	// Platform indica que es un BOM (platform(...)).
	Platform bool
	// Local indica que apunta a otro módulo del proyecto.
	Local bool
}

// reInclude busca include(":app", ":lib") en settings.gradle.kts.
var reInclude = regexp.MustCompile(`include\s*\(([^)]*)\)`)

// rePluginID busca id("com.android.application") en el bloque plugins.
var rePluginID = regexp.MustCompile(`id\s*\(\s*"([^"]+)"\s*\)`)

// Detect analiza el proyecto Gradle que hay en dir.
func Detect(dir string) (*Project, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	settingsPath := findSettingsFile(abs)
	if settingsPath == "" {
		return nil, fmt.Errorf("no se encontró settings.gradle.kts ni settings.gradle en %s", abs)
	}

	settings, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil, err
	}
	root := filepath.Dir(settingsPath)

	proj := &Project{
		RootDir: root,
		Name:    stringValue(string(settings), "rootProject.name"),
	}

	// Los módulos declarados con include(...).
	for _, m := range reInclude.FindAllStringSubmatch(string(settings), -1) {
		for _, name := range splitStrings(m[1]) {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			proj.Modules = append(proj.Modules, Module{
				Name: name,
				Path: strings.TrimPrefix(name, ":"),
			})
		}
	}

	// Si settings no declara ningún módulo, se prueba el módulo "app" por
	// convención.
	if len(proj.Modules) == 0 {
		proj.Modules = append(proj.Modules, Module{Name: ":app", Path: "app"})
	}

	for i := range proj.Modules {
		mod, err := ParseModule(filepath.Join(root, proj.Modules[i].Path))
		if err != nil {
			proj.Warnings = append(proj.Warnings,
				fmt.Sprintf("no se pudo leer el módulo %s: %v", proj.Modules[i].Name, err))
			continue
		}
		proj.Modules[i] = *mod
		if mod.hasPlugin("com.android.application") {
			app := proj.Modules[i]
			proj.App = &app
		}
	}

	if proj.App == nil {
		return proj, fmt.Errorf("no se encontró ningún módulo de aplicación (plugin com.android.application)")
	}
	return proj, nil
}

// findSettingsFile busca el fichero de settings del proyecto.
func findSettingsFile(dir string) string {
	for _, name := range []string{"settings.gradle.kts", "settings.gradle"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// ParseModule lee el build.gradle.kts de un módulo.
func ParseModule(dir string) (*Module, error) {
	var path string
	for _, name := range []string{"build.gradle.kts", "build.gradle"} {
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			path = candidate
			break
		}
	}
	if path == "" {
		return nil, fmt.Errorf("no se encontró build.gradle.kts en %s", dir)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	src := string(data)

	mod := &Module{
		Name: ":" + filepath.Base(dir),
		Path: filepath.Base(dir),
	}

	if plugins, ok := findBlock(src, "plugins"); ok {
		for _, m := range rePluginID.FindAllStringSubmatch(plugins, -1) {
			mod.Plugins = append(mod.Plugins, m[1])
		}
	}

	android, _ := findBlock(src, "android")
	if android != "" {
		mod.Namespace = stringValue(android, "namespace")
		mod.CompileSdk = intValue(android, "compileSdk")
		if defaultConfig, ok := findBlock(android, "defaultConfig"); ok {
			mod.ApplicationID = stringValue(defaultConfig, "applicationId")
			mod.MinSdk = intValue(defaultConfig, "minSdk")
			mod.TargetSdk = intValue(defaultConfig, "targetSdk")
			mod.VersionCode = intValue(defaultConfig, "versionCode")
			mod.VersionName = stringValue(defaultConfig, "versionName")
		}
		if compileOptions, ok := findBlock(android, "compileOptions"); ok {
			mod.JavaTarget = javaVersion(compileOptions)
		}
		if kotlin, ok := findBlock(android, "kotlin"); ok {
			if target := javaVersion(kotlin); target > 0 {
				mod.JavaTarget = target
			}
		}
	}

	// El bloque dependencies.
	if deps, ok := findBlock(src, "dependencies"); ok {
		sets := parseDependencies(deps)
		mod.Dependencies = sets.Common
		mod.DebugDependencies = sets.Debug
		mod.ReleaseDependencies = sets.Release
		mod.Kapt = sets.Kapt
		mod.LocalProjects = sets.Local
	}

	// El target de Java también puede estar fuera del bloque android
	// (kotlin { compilerOptions { ... } }).
	if mod.JavaTarget == 0 {
		if kotlin, ok := findBlock(src, "kotlin"); ok {
			mod.JavaTarget = javaVersion(kotlin)
		}
	}

	// Las reglas de R8 del módulo. Solo se importan las que están dentro del
	// proyecto: las que salen de getDefaultProguardFile() viven en el SDK de
	// Android y rain las sustituye por sus propias reglas base.
	for _, p := range parseProguardFiles(src) {
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
			continue
		}
		mod.ProguardFiles = append(mod.ProguardFiles, filepath.Join(mod.Path, p))
	}

	return mod, nil
}

// hasPlugin indica si el módulo aplica el plugin dado.
func (m *Module) hasPlugin(id string) bool {
	for _, p := range m.Plugins {
		if p == id {
			return true
		}
	}
	return false
}

// splitStrings divide una lista de cadenas entre comas.
func splitStrings(src string) []string {
	var out []string
	for _, part := range strings.Split(src, ",") {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, `"`)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
