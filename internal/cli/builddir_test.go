package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func escribirArchivo(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectarLayout(t *testing.T) {
	casos := []struct {
		nombre string
		crear  func(dir string)
		want   layoutProyecto
	}{
		{
			nombre: "vacio",
			crear:  func(string) {},
			want:   layoutDesconocido,
		},
		{
			nombre: "rain.toml",
			crear: func(dir string) {
				escribirArchivo(t, filepath.Join(dir, "rain.toml"), "")
			},
			want: layoutRain,
		},
		{
			nombre: "manifest en la raiz",
			crear: func(dir string) {
				escribirArchivo(t, filepath.Join(dir, "AndroidManifest.xml"), "<manifest/>")
			},
			want: layoutPlano,
		},
		{
			nombre: "manifest en src/main",
			crear: func(dir string) {
				escribirArchivo(t, filepath.Join(dir, "src", "main", "AndroidManifest.xml"), "<manifest/>")
			},
			want: layoutPlano,
		},
		{
			nombre: "rain.toml gana al manifest suelto",
			crear: func(dir string) {
				escribirArchivo(t, filepath.Join(dir, "rain.toml"), "")
				escribirArchivo(t, filepath.Join(dir, "AndroidManifest.xml"), "<manifest/>")
			},
			want: layoutRain,
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			dir := t.TempDir()
			caso.crear(dir)
			if got := detectarLayout(dir); got != caso.want {
				t.Errorf("detectarLayout = %v, quiero %v", got, caso.want)
			}
		})
	}
}
