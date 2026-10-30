package main

import (
"flag"
"fmt"
"os"
"path/filepath"

"rain/internal/build"
"rain/internal/config"
"rain/internal/pom"
"rain/internal/resolver"
"rain/internal/ui"
)

func main() {
if len(os.Args) < 2 {
fmt.Println("uso: rain <comando>")
os.Exit(1)
}

switch os.Args[1] {
case "init":
cmdInit(os.Args[2:])
case "build":
cmdBuildReal(os.Args[2:])
case "clean":
cmdClean(os.Args[2:])
default:
ui.Error("Comando desconocido: %s", os.Args[1])
os.Exit(1)
}
}

func cmdInit(args []string) {
fs := flag.NewFlagSet("init", flag.ExitOnError)
fs.Parse(args)

if _, err := os.Stat("rain.toml"); err == nil {
ui.Warn("rain.toml ya existe. No se sobrescribe.")
return
}

fmt.Println("Rain init — ¿modo de dependencias?")
fmt.Println("  1) simple   — AAR/JAR explícitos, sin transitivas")
fmt.Println("  2) completo — resuelve POMs, parents, dependencyManagement")
fmt.Print("Elige [1/2]: ")

var choice string
fmt.Scanln(&choice)

mode := "simple"
if choice == "2" { mode = "completo" }

cfg := config.Config{
Project:      config.Project{Name: "MiApp", Package: "com.rain.miapp", Version: "1.0"},
Build:        config.Build{Debug: true},
Mode:         mode,
Dependencies: []string{},
}

if err := config.Write("rain.toml", cfg); err != nil {
ui.Error("Error escribiendo rain.toml: %v", err)
os.Exit(1)
}
ui.Success("rain.toml creado (modo: %s)", mode)
}

func cmdBuildReal(args []string) {
fs := flag.NewFlagSet("build", flag.ExitOnError)
androidJar := fs.String("android-jar", "", "ruta a android.jar")
release := fs.Bool("release", false, "compilar en modo release (R8)")
verbose := fs.Bool("verbose", false, "mostrar detalles de ejecución")
quiet := fs.Bool("quiet", false, "solo mostrar errores")
fs.Parse(args)

ui.Verbose = *verbose
ui.Quiet = *quiet

if *androidJar == "" {
ui.Error("Falta -android-jar")
fmt.Println("\nSugerencia:")
fmt.Println("  Usa --android-jar <ruta> para indicar el archivo android.jar del SDK.")
fmt.Println("  Ejemplo: rain build --android-jar ./sdk/android.jar")
os.Exit(1)
}

cfg, err := config.Read("rain.toml")
fmt.Printf("DEBUG: Signing es nil? %v\n", cfg.Signing == nil)
if err != nil {
ui.Warn("No se pudo leer rain.toml (%v). Usando modo manual.", err)
}

// Resolver siempre el grafo completo para no omitir dependencias transitivas
// cuando los JAR de primer nivel ya existen en libs.
if len(cfg.Dependencies) > 0 {
	ui.Info("Resolviendo y descargando dependencias...")
	if err := resolveAndFetchWithProgress(cfg, "libs"); err != nil {
		ui.Error("Resolviendo dependencias: %v", err)
		os.Exit(1)
	}
}

// Determinar módulos
modules := cfg.Modules
if len(modules) == 0 {
modules = []config.Module{{Name: "app", Path: "app"}}
}

for _, mod := range modules {
ui.Info("=== Compilando módulo: %s ===", mod.Name)
bcfg := build.Config{
AppDir:     mod.Path,
BuildDir:   filepath.Join(mod.Path, "build"),
LibsDir:    "libs",
AndroidJar: *androidJar,
Package:    cfg.Project.Package,
Release:    *release,
Signing:    cfg.Signing,
}
if err := build.Run(bcfg); err != nil {
ui.Error("Módulo %s: %v", mod.Name, err)
os.Exit(1)
}
}
}

func resolveAndFetchWithProgress(cfg config.Config, libsDir string) error {
	var allCoords []string

	if cfg.Mode == "completo" {
		resolved, err := pom.ResolveTransitives(cfg.Dependencies, "cache")
		if err != nil {
			return fmt.Errorf("resolviendo árbol de dependencias: %w", err)
		}
		for _, dep := range resolved {
			allCoords = append(allCoords, dep.Coordinate())
		}
	} else {
		allCoords = append(allCoords, cfg.Dependencies...)
	}

	if len(allCoords) == 0 {
		return nil
	}

	pb := ui.NewProgressBar(len(allCoords), "Descargando ")
	for i, coord := range allCoords {
		pb.Current = i + 1
		pb.Render()
		if _, err := resolver.DownloadArtifact(coord, libsDir); err != nil {
			pb.Finish()
			return fmt.Errorf("no se pudo descargar %s: %w", coord, err)
		}
	}
	pb.Finish()
	return nil
}

func cmdClean(args []string) {
fs := flag.NewFlagSet("clean", flag.ExitOnError)
fs.Parse(args)
ui.Info("Limpiando build, libs y cache...")
os.RemoveAll("app/build")
os.RemoveAll("libs")
os.RemoveAll("cache")
ui.Success("Limpieza completada")
}
