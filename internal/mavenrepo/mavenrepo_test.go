package mavenrepo

import "testing"

func TestForAddsJitPackOnlyForGitHubGroups(t *testing.T) {
	ExtraRepositories = nil
	t.Cleanup(func() { ExtraRepositories = nil })

	cases := []struct {
		group string
		want  bool
	}{
		{"com.github.QWEA0", true},
		{"io.github.itskreisler", true},
		{"androidx.core", false},
		{"com.google.android.material", false},
		{"org.jetbrains.kotlin", false},
	}
	for _, tc := range cases {
		repos := For(Base, tc.group)
		got := contains(repos, JitPack)
		if got != tc.want {
			t.Errorf("For(%q) incluye JitPack = %v, quiero %v (%v)", tc.group, got, tc.want, repos)
		}
	}
}

func TestForAlwaysIncludesBaseFirst(t *testing.T) {
	ExtraRepositories = nil
	repos := For(Base, "androidx.core")
	if len(repos) < len(Base) {
		t.Fatalf("For devolvió %d repos, quiero al menos %d", len(repos), len(Base))
	}
	for i, want := range Base {
		if repos[i] != want {
			t.Errorf("repos[%d] = %q, quiero %q", i, repos[i], want)
		}
	}
}

func TestForAppendsExtras(t *testing.T) {
	ExtraRepositories = []string{"https://nexus.local/maven", ""}
	t.Cleanup(func() { ExtraRepositories = nil })

	repos := For(Base, "androidx.core")
	if last := repos[len(repos)-1]; last != "https://nexus.local/maven" {
		t.Errorf("último repo = %q, quiero el extra (y las entradas vacías se ignoran)", last)
	}
	for _, r := range repos {
		if r == "" {
			t.Errorf("For no debe incluir entradas vacías: %v", repos)
		}
	}
}

func TestForDeduplicatesExtrasAndJitPack(t *testing.T) {
	ExtraRepositories = []string{JitPack}
	t.Cleanup(func() { ExtraRepositories = nil })

	repos := For(Base, "com.github.QWEA0")
	count := 0
	for _, r := range repos {
		if r == JitPack {
			count++
		}
	}
	if count != 1 {
		t.Errorf("JitPack aparece %d veces, quiero 1 (%v)", count, repos)
	}
}
