package memenv

import (
	"strings"
	"testing"
)

// withProot simula un entorno con proot (o sin él) durante el test.
func withProot(t *testing.T, enabled bool) {
	t.Helper()
	original := prootDetector
	prootDetector = func() bool { return enabled }
	t.Cleanup(func() { prootDetector = original })
}

func TestUnderProotDetectaLasVariablesDeProot(t *testing.T) {
	original := prootDetector
	prootDetector = detectProot
	t.Cleanup(func() { prootDetector = original })

	t.Setenv("PROOT_TMP_DIR", "/data/data/com.termux/files/usr/var/tmp/proot")
	if !UnderProot() {
		t.Error("UnderProot() = false con PROOT_TMP_DIR puesto, want true")
	}
}

func TestUnderProotDetectaElProcesoTrazado(t *testing.T) {
	original := prootDetector
	prootDetector = detectProot
	t.Cleanup(func() { prootDetector = original })

	for _, name := range []string{"PROOT_TMP_DIR", "PROOT_L2S_DIR", "PROOT_NO_SECCOMP", "PROOT_LOADER"} {
		t.Setenv(name, "")
	}
	// Bajo proot-distro el proceso está trazado; fuera de él, no.
	want := tracerPID() > 0
	if got := UnderProot(); got != want {
		t.Errorf("UnderProot() = %v, want %v (TracerPid=%d)", got, want, tracerPID())
	}
}

func TestGoGCPercentAplicaElValorConservadorBajoProot(t *testing.T) {
	withProot(t, true)
	t.Setenv(EnvGoGCPercent, "")
	t.Setenv("GOGC", "")

	if got := GoGCPercent(); got != prootGCPercent {
		t.Errorf("GoGCPercent bajo proot = %d, want %d", got, prootGCPercent)
	}
}

func TestGoMemoryLimitAplicaElLimiteConservadorBajoProot(t *testing.T) {
	withProot(t, true)
	t.Setenv(EnvGoMemLimit, "")
	t.Setenv("GOMEMLIMIT", "")

	got := GoMemoryLimit()
	if got <= 0 {
		t.Fatalf("GoMemoryLimit bajo proot = %d, want > 0", got)
	}
	if got > prootGoLimit {
		t.Errorf("GoMemoryLimit bajo proot = %d, no debe superar %d", got, prootGoLimit)
	}
}

func TestLoPuestoPorElUsuarioGanaBajoProot(t *testing.T) {
	withProot(t, true)

	// GOMEMLIMIT ya la aplica el runtime al arrancar: Rain no la pisa.
	t.Setenv(EnvGoMemLimit, "")
	t.Setenv("GOMEMLIMIT", "1200MiB")
	if got := GoMemoryLimit(); got != 0 {
		t.Errorf("GoMemoryLimit con GOMEMLIMIT puesta = %d, want 0", got)
	}

	// GOGC igual: lo aplica el runtime.
	t.Setenv(EnvGoGCPercent, "")
	t.Setenv("GOGC", "42")
	if got := GoGCPercent(); got != 0 {
		t.Errorf("GoGCPercent con GOGC=42 = %d, want 0", got)
	}

	// Y las variables propias de Rain tienen prioridad sobre la detección.
	t.Setenv(EnvGoGCPercent, "33")
	if got := GoGCPercent(); got != 33 {
		t.Errorf("GoGCPercent con %s=33 = %d, want 33", EnvGoGCPercent, got)
	}
	t.Setenv(EnvGoMemLimit, "512MiB")
	if got := GoMemoryLimit(); got != 512<<20 {
		t.Errorf("GoMemoryLimit con %s=512MiB = %d, want %d", EnvGoMemLimit, got, 512<<20)
	}
}

func TestSinProotNoSeAplicaElLimiteConservador(t *testing.T) {
	withProot(t, false)
	t.Setenv(EnvGoMemLimit, "")
	t.Setenv("GOMEMLIMIT", "")
	t.Setenv(EnvGoGCPercent, "")
	t.Setenv("GOGC", "")

	// Sin proot y con memoria de sobra no se toca nada.
	if got := GoGCPercent(); got != 0 {
		t.Errorf("GoGCPercent sin proot = %d, want 0", got)
	}
}

func TestDescribeYHintMencionanProot(t *testing.T) {
	withProot(t, true)

	detail := Describe()
	if !strings.Contains(detail, "proot") {
		t.Errorf("Describe() = %q, no contiene %q", detail, "proot")
	}
	if hint := Hint(); hint == "" {
		t.Error("Hint() = \"\", se esperaba un aviso sobre proot")
	}
}
