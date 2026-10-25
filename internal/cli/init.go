package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/itskreisler/rain/internal/config"
	"github.com/itskreisler/rain/internal/ui"
)

func newInitCmd() *cobra.Command {
	devlog("newInitCmd", "", "Crea el subcomando init para inicializar nuevos proyectos rain")
	var (
		template       string
		modeAlias      string
		name           string
		pkg            string
		version        string
		dir            string
		nonInteractive bool
		force          bool
	)

	cmd := &cobra.Command{
		Use:   "init [dir]",
		Short: "Crea un proyecto rain listo para compilar",
		Long: "Crea un proyecto Android completo (rain.toml, AndroidManifest.xml, fuentes y\n" +
			"recursos) en [dir] o en el directorio actual.\n\n" +
			"Plantillas:\n" +
			"  simple    Java con vistas programáticas, sin dependencias externas\n" +
			"  completo  Kotlin + Compose (AndroidX), resuelto en modo completo\n\n" +
			"El proyecto generado compila tal cual con `rain build`.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			if dir != "" {
				root = dir
			}

			if template == "" {
				template = modeAlias
			}
			if template == "" {
				if nonInteractive {
					template = "simple"
				} else {
					template = promptTemplate(cmd)
				}
			}
			if template != "simple" && template != "completo" {
				return usageErr("plantilla inválida %q: usa simple o completo", template)
			}

			if name == "" {
				name = "MiApp"
			}
			if pkg == "" {
				pkg = defaultPackage(name)
			}
			if version == "" {
				version = "1.0"
			}

			if err := validatePackage(pkg); err != nil {
				return usageErr("%v", err)
			}

			cfgPath := filepath.Join(root, "rain.toml")
			if _, err := os.Stat(cfgPath); err == nil && !force {
				ui.Warn("rain.toml ya existe en %s. No se sobrescribe (usa --force).", root)
				return nil
			}

			files, cfg, err := scaffold(template, name, pkg, version)
			if err != nil {
				return err
			}

			if err := os.MkdirAll(root, 0o755); err != nil {
				return fmt.Errorf("creando %s: %w", root, err)
			}
			for rel, content := range files {
				path := filepath.Join(root, rel)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					return fmt.Errorf("creando %s: %w", filepath.Dir(path), err)
				}
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					return fmt.Errorf("escribiendo %s: %w", path, err)
				}
			}
			if err := config.Write(cfgPath, cfg); err != nil {
				return fmt.Errorf("escribiendo rain.toml: %w", err)
			}

			ui.Success("Proyecto %q creado en %s (plantilla: %s)", name, root, template)
			ui.Info("Siguiente paso: cd %s && rain build", root)
			if template == "completo" {
				ui.Info("La plantilla completo descarga AndroidX/Compose en el primer build.")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&template, "template", "", "plantilla: simple|completo")
	cmd.Flags().StringVar(&modeAlias, "mode", "", "alias de --template")
	cmd.Flags().StringVar(&name, "name", "", "nombre de la app (por defecto MiApp)")
	cmd.Flags().StringVar(&pkg, "package", "", "package id (por defecto com.rain.<nombre>)")
	cmd.Flags().StringVar(&version, "version", "", "versión (por defecto 1.0)")
	cmd.Flags().StringVar(&dir, "dir", "", "directorio destino (por defecto el actual)")
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "no preguntar; usar valores por defecto")
	cmd.Flags().BoolVar(&force, "force", false, "sobrescribir archivos existentes")

	return cmd
}

func promptTemplate(cmd *cobra.Command) string {
	devlog("promptTemplate", fmt.Sprintf("cmd=%s", cmd.Name()), "Solicita interactiva al usuario la plantilla a utilizar")
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "Rain init — ¿qué plantilla quieres?")
	fmt.Fprintln(out, "  1) simple    — Java, vistas programáticas, sin dependencias externas")
	fmt.Fprintln(out, "  2) completo  — Kotlin + Compose (AndroidX), modo completo")
	fmt.Fprint(out, "Elige [1/2]: ")

	reader := bufio.NewReader(cmd.InOrStdin())
	line, _ := reader.ReadString('\n')
	if strings.TrimSpace(line) == "2" {
		return "completo"
	}
	return "simple"
}

// scaffold devuelve los archivos a crear (ruta relativa -> contenido) y la
// configuración rain.toml para la plantilla dada.
func scaffold(template, name, pkg, version string) (map[string]string, config.Config, error) {
	devlog("scaffold", fmt.Sprintf("template=%s, name=%s, pkg=%s, version=%s", template, name, pkg, version), "Genera la estructura de archivos inicial del proyecto")
	cfg := config.Config{
		Project: config.Project{Name: name, Package: pkg, Version: version},
		Build:   config.Build{Debug: true},
		Mode:    template,
	}

	files := map[string]string{
		"app/AndroidManifest.xml":    manifestXML(name, pkg, template),
		"app/res/values/strings.xml": stringsXML(name),
		"app/res/values/themes.xml":  themesXML(template),
	}

	pkgPath := strings.ReplaceAll(pkg, ".", "/")

	switch template {
	case "simple":
		files[filepath.ToSlash(filepath.Join("app/java", pkgPath, "MainActivity.java"))] = javaMainActivity(pkg)
		cfg.Dependencies = []string{}
	case "completo":
		files[filepath.ToSlash(filepath.Join("app/java", pkgPath, "MainActivity.kt"))] = kotlinComposeMainActivity(pkg)
		// Cierre mínimo de Compose; rain resuelve el resto en modo completo.
		cfg.Dependencies = []string{
			"androidx.activity:activity-compose:1.8.2",
			"androidx.compose.material3:material3-android:1.5.0-beta01",
		}
	default:
		return nil, cfg, usageErr("plantilla inválida %q", template)
	}

	return files, cfg, nil
}

// defaultPackage deriva un package válido a partir del nombre.
func defaultPackage(name string) string {
	devlog("defaultPackage", fmt.Sprintf("name=%s", name), "Genera un package ID predeterminado basado en el nombre")
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	slug := b.String()
	if slug == "" {
		slug = "miapp"
	}
	if slug[0] >= '0' && slug[0] <= '9' {
		slug = "app" + slug
	}
	return "com.rain." + slug
}

// validatePackage comprueba que el package sea un identificador Java válido.
func validatePackage(pkg string) error {
	devlog("validatePackage", fmt.Sprintf("pkg=%s", pkg), "Valida que el package id sea un identificador Java válido")
	if pkg == "" {
		return fmt.Errorf("el package no puede estar vacío")
	}
	for _, part := range strings.Split(pkg, ".") {
		if part == "" {
			return fmt.Errorf("package inválido %q: componente vacío", pkg)
		}
		for i, r := range part {
			ok := r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
				(i > 0 && r >= '0' && r <= '9')
			if !ok {
				return fmt.Errorf("package inválido %q: %q no es un identificador válido", pkg, part)
			}
		}
	}
	return nil
}
