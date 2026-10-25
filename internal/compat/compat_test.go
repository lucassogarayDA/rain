package compat

import "testing"

func TestMajor(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"17.0.20", 17},
		{"21", 21},
		{"1.8.0_392", 8},
		{"2.4.21", 2},
		{"9.5.28", 9},
		{"36.0.0", 36},
		{"", 0},
		{"no-es-version", 0},
	}
	for _, tc := range tests {
		if got := Major(tc.in); got != tc.want {
			t.Errorf("Major(%q) = %d, quiero %d", tc.in, got, tc.want)
		}
	}
}

func TestCheckMatrixJDKTooOld(t *testing.T) {
	issues := CheckMatrix(Versions{JDK: "11.0.2"})
	if !HasBlocking(issues) {
		t.Fatalf("JDK 11 debería ser bloqueante: %+v", issues)
	}
	if issues[0].Topic != "jdk" || issues[0].Severity != Error {
		t.Errorf("issue = %+v, quiero error de jdk", issues[0])
	}
}

func TestCheckMatrixJDKModernIsSilent(t *testing.T) {
	for _, jdk := range []string{"17.0.20", "21", "25"} {
		issues := CheckMatrix(Versions{JDK: jdk})
		if HasBlocking(issues) {
			t.Errorf("JDK %s no debería bloquear: %+v", jdk, issues)
		}
	}
}

func TestCheckMatrixR8KotlinCompatible(t *testing.T) {
	issues := CheckMatrix(Versions{R8: "9.5.28", Kotlin: "2.4.21"})
	if HasBlocking(issues) {
		t.Errorf("R8 9.5 + Kotlin 2.4 no debería bloquear: %+v", issues)
	}
	found := false
	for _, issue := range issues {
		if issue.Topic == "r8-kotlin" && issue.Severity == Advice {
			found = true
		}
	}
	if !found {
		t.Errorf("debería haber un aviso informativo de r8-kotlin: %+v", issues)
	}
}

func TestCheckMatrixR8OldKotlin2Blocks(t *testing.T) {
	issues := CheckMatrix(Versions{R8: "4.0.63", Kotlin: "2.4.21"})
	if !HasBlocking(issues) {
		t.Fatalf("R8 4 + Kotlin 2.4 debería bloquear: %+v", issues)
	}
}

func TestCheckMatrixR8OldKotlin1Silent(t *testing.T) {
	issues := CheckMatrix(Versions{R8: "4.0.63", Kotlin: "1.9.22"})
	if HasBlocking(issues) {
		t.Errorf("R8 viejo con Kotlin 1.9 no debería bloquear: %+v", issues)
	}
}

func TestCheckMatrixOldCompileSDK(t *testing.T) {
	issues := CheckMatrix(Versions{CompileSDK: 30})
	if len(issues) == 0 || issues[0].Topic != "sdk" {
		t.Fatalf("compileSdk 30 debería avisar: %+v", issues)
	}
	if issues[0].Severity != Warning {
		t.Errorf("severidad = %q, quiero warning", issues[0].Severity)
	}
}

func TestCheckMatrixBuildToolsOlderThanSDK(t *testing.T) {
	issues := CheckMatrix(Versions{BuildTools: "30.0.3", CompileSDK: 36})
	found := false
	for _, issue := range issues {
		if issue.Topic == "build-tools" {
			found = true
		}
	}
	if !found {
		t.Errorf("build-tools 30 con compileSdk 36 debería avisar: %+v", issues)
	}
}

func TestCheckMatrixEmptyIsClean(t *testing.T) {
	if issues := CheckMatrix(Versions{}); len(issues) != 0 {
		t.Errorf("versiones vacías no deberían producir issues: %+v", issues)
	}
}
