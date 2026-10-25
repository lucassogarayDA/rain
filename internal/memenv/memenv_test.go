package memenv

import (
	"os"
	"strings"
	"testing"
)

func TestParseSize(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"1g", 1 << 30, true},
		{"1G", 1 << 30, true},
		{"512m", 512 << 20, true},
		{"1200MiB", 1200 << 20, true},
		{"2GiB", 2 << 30, true},
		{"64k", 64 << 10, true},
		{"1048576", 1048576, true},
		{"", 0, false},
		{"abc", 0, false},
		{"12x", 0, false},
		{"-1g", 0, false},
	}

	for _, tc := range cases {
		got, ok := ParseSize(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("ParseSize(%q) = (%d, %v), want (%d, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestFormatSizeRedondeaAMultiplosDe128MiB(t *testing.T) {
	// 1400 MiB exactos no son múltiplo de 128: debe bajar a 1280m.
	if got := FormatSize(1400 << 20); got != "1280m" {
		t.Errorf("FormatSize(1400MiB) = %q, want %q", got, "1280m")
	}
	if got := FormatSize(1 << 30); got != "1024m" {
		t.Errorf("FormatSize(1GiB) = %q, want %q", got, "1024m")
	}
	// Nunca por debajo del mínimo de 256 MiB.
	if got := FormatSize(64 << 20); got != "256m" {
		t.Errorf("FormatSize(64MiB) = %q, want %q", got, "256m")
	}
}

func TestFormatBytes(t *testing.T) {
	if got := FormatBytes(0); got != "0 B" {
		t.Errorf("FormatBytes(0) = %q", got)
	}
	if got := FormatBytes(512); got != "512 B" {
		t.Errorf("FormatBytes(512) = %q", got)
	}
	if got := FormatBytes(1 << 30); got != "1.0 GiB" {
		t.Errorf("FormatBytes(1GiB) = %q", got)
	}
}

func TestHeapLimitRespetaRainJavaMaxHeap(t *testing.T) {
	t.Setenv(EnvJavaMaxHeap, "777m")
	if got := HeapLimit("2g"); got != "777m" {
		t.Errorf("HeapLimit con %s=777m = %q, want %q", EnvJavaMaxHeap, got, "777m")
	}
}

func TestHeapLimitRespetaElFallback(t *testing.T) {
	t.Setenv(EnvJavaMaxHeap, "")
	got := HeapLimit("512m")
	if !strings.HasSuffix(got, "m") {
		t.Fatalf("HeapLimit = %q, se esperaba un valor en MiB", got)
	}
	// El valor automático nunca puede superar el tope indicado.
	bytes, ok := ParseSize(got)
	if !ok {
		t.Fatalf("HeapLimit devolvió un valor no parseable: %q", got)
	}
	if bytes > 512<<20 {
		t.Errorf("HeapLimit = %q (%d bytes) supera el tope de 512m", got, bytes)
	}
	// ...ni bajar de 256 MiB.
	if bytes < 256<<20 {
		t.Errorf("HeapLimit = %q (%d bytes) baja del mínimo de 256m", got, bytes)
	}
}

func TestUserHeapOverride(t *testing.T) {
	t.Setenv(EnvJavaMaxHeap, "")
	t.Setenv("JAVA_OPTS", "")
	t.Setenv("JAVA_TOOL_OPTIONS", "")
	t.Setenv("JDK_JAVA_OPTIONS", "")

	if UserHeapOverride() {
		t.Fatal("sin variables de entorno no debería detectarse override")
	}

	t.Setenv("JAVA_OPTS", "-Xmx3g")
	if !UserHeapOverride() {
		t.Error("JAVA_OPTS con -Xmx debe detectarse como override")
	}
}

func TestGoMemoryLimitRespetaRainGoMemLimit(t *testing.T) {
	t.Setenv(EnvGoMemLimit, "321MiB")
	got := GoMemoryLimit()
	if got != 321<<20 {
		t.Errorf("GoMemoryLimit = %d, want %d", got, 321<<20)
	}
}

func TestGoMemoryLimitRespetaGOMEMLIMIT(t *testing.T) {
	// GOMEMLIMIT ya la aplica el runtime al arrancar: Rain no debe pisarla.
	t.Setenv(EnvGoMemLimit, "")
	t.Setenv("GOMEMLIMIT", "1200MiB")
	if got := GoMemoryLimit(); got != 0 {
		t.Errorf("GoMemoryLimit con GOMEMLIMIT posto = %d, want 0 (lo aplica el runtime)", got)
	}
}

func TestGoGCPercent(t *testing.T) {
	t.Setenv(EnvGoGCPercent, "")
	t.Setenv("GOGC", "")

	t.Setenv(EnvGoGCPercent, "42")
	if got := GoGCPercent(); got != 42 {
		t.Errorf("GoGCPercent con %s=42 = %d, want 42", EnvGoGCPercent, got)
	}

	t.Setenv(EnvGoGCPercent, "")
	t.Setenv("GOGC", "15")
	if got := GoGCPercent(); got != 0 {
		t.Errorf("GoGCPercent con GOGC=15 = %d, want 0 (lo aplica el runtime)", got)
	}
}

func TestApplyGoLimitNoPisaLoPuestoPorElUsuario(t *testing.T) {
	t.Setenv(EnvGoMemLimit, "")
	t.Setenv("GOMEMLIMIT", "1200MiB")
	if got := ApplyGoLimit(); got != 0 {
		t.Errorf("ApplyGoLimit con GOMEMLIMIT puesto = %d, want 0", got)
	}
}

func TestDescribeIncluyeElEstadoDeLaMemoria(t *testing.T) {
	t.Setenv(EnvJavaMaxHeap, "")
	detail := Describe()
	for _, want := range []string{"total", "disponible", "heap JVM"} {
		if !strings.Contains(detail, want) {
			t.Errorf("Describe() = %q, no contiene %q", detail, want)
		}
	}
}

func TestDetectLeeMemInfo(t *testing.T) {
	info := Detect()
	// En Linux siempre hay /proc/meminfo; si no, el test no aplica.
	if _, err := os.Stat("/proc/meminfo"); err != nil {
		t.Skip("sin /proc/meminfo")
	}
	if info.TotalBytes <= 0 {
		t.Errorf("TotalBytes = %d, se esperaba la RAM total", info.TotalBytes)
	}
	if info.Budget() <= 0 {
		t.Errorf("Budget() = %d, se esperaba un presupuesto positivo", info.Budget())
	}
}
