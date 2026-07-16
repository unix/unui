package version

import "testing"

func TestSemanticAcceptsCommonReleaseVersionForms(t *testing.T) {
	for input, expected := range map[string]string{
		"0.1.0":          "v0.1.0",
		"V1.2.3":         "v1.2.3",
		"v1.2.3-rc.1":    "v1.2.3-rc.1",
		"v1.2.3+release": "v1.2.3",
	} {
		version, err := Semantic(input)
		if err != nil {
			t.Fatalf("normalize %q: %v", input, err)
		}
		if version != expected {
			t.Fatalf("normalize %q: got %q, want %q", input, version, expected)
		}
	}
}

func TestCompareUsesSemanticVersionPrecedence(t *testing.T) {
	comparison, err := Compare("1.0.0-rc.1", "1.0.0")
	if err != nil {
		t.Fatalf("compare versions: %v", err)
	}
	if comparison >= 0 {
		t.Fatalf("expected prerelease to be older, received %d", comparison)
	}
}

func TestSemanticRejectsIncompleteVersions(t *testing.T) {
	for _, value := range []string{"1", "1.2", "v1.2"} {
		if _, err := Semantic(value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

func TestIsDevelopmentRecognizesBuildFallbacks(t *testing.T) {
	for _, value := range []string{"", "dev", "DEVEL", "(devel)"} {
		if !IsDevelopment(value) {
			t.Fatalf("expected %q to be a development version", value)
		}
	}
}
