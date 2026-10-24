package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/itskreisler/rain/internal/config"
	"github.com/itskreisler/rain/internal/gradle"
	"github.com/itskreisler/rain/internal/pom"
	"github.com/itskreisler/rain/internal/ui"
)

func newGradleCmd() *cobra.Command {
	devlog("newGradleCmd", "", "Crea el subcomando gradle para importar proyectos Gradle")
	var (
		output string
		force  bool
	)

	cmd := &cobra.Command{
		Use:   "gradle [dir]",
		Short: "Importa un proyecto Gradle y genera su rain.toml",
		Long: "Lee la configuración de un proyecto Gradle (Kotlin DSL) y escribe un\n" +
			"rain.toml equivalente, para poder compilarlo con \"rain build\" sin Gradle.\n\n" +
			"Se importan el namespace, los niveles de API, las versiones, las\n" +
			"dependencias (resolviendo los BOM) y los annotation processors de kapt.\n\n" +
			"El rain.toml se escribe en la raíz del proyecto Gradle, que es donde\n" +
			"está el módulo de la app.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}

			proj, err := gradle.Detect(dir)
			if err != nil {
				return err
			}
			app := proj.App
			if app == nil {
				return fmt.Errorf("el proyecto no tiene ningún módulo de aplicación")
			}

			for _, w := range proj.Warnings {
				ui.Warn("%s", w)
			}

			cfg, err := gradleConfig(proj)
			if err != nil {
				return err
			}

			outPath := output
			if outPath == "" {
				outPath = filepath.Join(proj.RootDir, "rain.toml")
			}
			if _, err := os.Stat(outPath); err == nil && !force {
				return fmt.Errorf("%s ya existe (usa --force para sobrescribirlo)", outPath)
			}

			if err := config.Write(outPath, cfg); err != nil {
				return fmt.Errorf("escribiendo %s: %w", outPath, err)
			}

			ui.Success("rain.toml generado en %s", outPath)
			ui.Info("Proyecto: %s (%s)", cfg.Project.Name, cfg.Project.Package)
			ui.Info("Dependencias: %d | processors de kapt: %d", len(cfg.Dependencies), len(cfg.Kapt.Processors))
			for _, w := range importWarnings {
				ui.Warn("%s", w)
			}
			ui.Info("Siguiente paso: cd %s && rain build", filepath.Dir(outPath))
			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "fichero de salida (por defecto <proyecto>/rain.toml)")
	cmd.Flags().BoolVar(&force, "force", false, "sobrescribir el rain.toml existente")

	return cmd
}

// importWarnings recoge los avisos de la última importación.
var importWarnings []string

// gradleConfig traduce un proyecto Gradle a la configuración de rain.
func gradleConfig(proj *gradle.Project) (config.Config, error) {
	devlog("gradleConfig", fmt.Sprintf("proj=%+v", proj), "Traduce la configuración de un proyecto Gradle a una estructura de rain")
	importWarnings = nil

	app := proj.App
	if app == nil {
		return config.Config{}, fmt.Errorf("el proyecto no tiene ningún módulo de aplicación")
	}

	name := proj.Name
	if name == "" {
		name = app.ApplicationID
	}

	cfg := config.Config{
		Mode: "completo",
		Project: config.Project{
			Name:    name,
			Package: app.Namespace,
			Version: app.VersionName,
		},
		Build: config.Build{
			Debug:         true,
			MinSdk:        app.MinSdk,
			TargetSdk:     app.TargetSdk,
			VersionCode:   app.VersionCode,
			VersionName:   app.VersionName,
			ApplicationID: app.ApplicationID,
			JavaTarget:    app.JavaTarget,
			ProguardFiles: app.ProguardFiles,
		},
		Modules: []config.Module{{Name: "app", Path: app.Path}},
	}

	if cfg.Project.Package == "" {
		cfg.Project.Package = app.ApplicationID
	}

	// Las versiones gestionadas por los BOM se leen de todos los artefactos
	// (los BOM suelen estar en las dependencias comunes, pero las de una
	// variante pueden declarar la versión sin ella).
	all := make([]gradle.Dependency, 0, len(app.Dependencies)+len(app.DebugDependencies)+len(app.ReleaseDependencies))
	all = append(all, app.Dependencies...)
	all = append(all, app.DebugDependencies...)
	all = append(all, app.ReleaseDependencies...)
	managed, err := managedVersions(all)
	if err != nil {
		return config.Config{}, err
	}

	deps, err := expandDependencies(app.Dependencies, managed)
	if err != nil {
		return config.Config{}, err
	}
	cfg.Dependencies = deps

	// Las dependencias por variante: en Gradle, debugImplementation solo está
	// en el debug y releaseImplementation solo en el release. Rain lo respeta
	// para que un release no lleve dependencias de depuración, que además
	// referencian clases opcionales y rompen R8.
	debugDeps, err := expandDependencies(app.DebugDependencies, managed)
	if err != nil {
		return config.Config{}, err
	}
	cfg.Build.DebugDependencies = debugDeps

	releaseDeps, err := expandDependencies(app.ReleaseDependencies, managed)
	if err != nil {
		return config.Config{}, err
	}
	cfg.Build.ReleaseDependencies = releaseDeps

	for _, k := range app.Kapt {
		if k.Platform {
			continue
		}
		cfg.Kapt.Processors = append(cfg.Kapt.Processors, k.Coordinate)
	}

	// Los módulos que la app declara como dependencia de proyecto no se
	// pueden importar: rain no construye librerías Android.
	for _, local := range app.LocalProjects {
		importWarnings = append(importWarnings,
			fmt.Sprintf("la dependencia de proyecto %q se ha omitido: rain no compila módulos de librería", local))
	}

	return cfg, nil
}

// managedVersions lee de los BOM declarados las versiones que gestionan.
//
// El BOM no es un artefacto que se pueda descargar: solo declara versiones en
// su dependencyManagement, así que se lee su POM y se usa como tabla de
// versiones.
func managedVersions(deps []gradle.Dependency) (map[string]string, error) {
	devlog("managedVersions", fmt.Sprintf("deps=%v", deps), "Obtiene las versiones gestionadas por BOMs")
	managed := map[string]string{}
	for _, d := range deps {
		if !d.Platform {
			continue
		}
		versions, err := pom.ManagedVersions(d.Coordinate, "cache")
		if err != nil {
			return nil, fmt.Errorf("leyendo el BOM %s: %w", d.Coordinate, err)
		}
		for k, v := range versions {
			managed[k] = v
		}
	}
	return managed, nil
}

// expandDependencies resuelve las versiones que faltan en las dependencias.
//
// Las dependencias declaradas sin versión heredan la que fija un BOM
// (platform(...)).
func expandDependencies(deps []gradle.Dependency, managed map[string]string) ([]string, error) {
	devlog("expandDependencies", fmt.Sprintf("deps=%v, managed=%+v", deps, managed), "Expande y resuelve las versiones faltantes en dependencias")
	var out []string
	seen := map[string]bool{}
	for _, d := range deps {
		if d.Platform {
			continue
		}
		coord := d.Coordinate
		parts := strings.Split(coord, ":")
		switch len(parts) {
		case 2:
			version, ok := managed[coord]
			if !ok {
				return nil, fmt.Errorf(
					"no se pudo resolver la versión de %s: no está en ningún BOM y no trae versión", coord)
			}
			coord = coord + ":" + version
		case 3:
		default:
			return nil, fmt.Errorf("coordenada no válida: %s", d.Coordinate)
		}
		if seen[coord] {
			continue
		}
		seen[coord] = true
		out = append(out, coord)
	}
	return out, nil
}
