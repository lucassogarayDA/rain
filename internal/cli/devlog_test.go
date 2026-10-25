package cli

import (
	"strings"
	"testing"
)

func TestDevlogFlagOutputsDevlogMessages(t *testing.T) {
	out, err := runCmd(t, "--devlog", "version")
	if err != nil {
		t.Fatalf("version con --devlog: %v", err)
	}
	if !strings.Contains(out, "[DEVLOG]") {
		t.Errorf("salida con --devlog debería contener [DEVLOG]:\n%s", out)
	}
}

func TestDevlogDisabledByDefault(t *testing.T) {
	out, err := runCmd(t, "version")
	if err != nil {
		t.Fatalf("version sin --devlog: %v", err)
	}
	if strings.Contains(out, "[DEVLOG]") {
		t.Errorf("salida sin --devlog no debería contener [DEVLOG]:\n%s", out)
	}
}
