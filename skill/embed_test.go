package skill

import (
	"io/fs"
	"strings"
	"testing"
)

func TestBundleContainsValidSkill(t *testing.T) {
	bundle, err := Bundle()
	if err != nil {
		t.Fatalf("open embedded skill: %v", err)
	}
	payload, err := fs.ReadFile(bundle, "SKILL.md")
	if err != nil {
		t.Fatalf("read embedded SKILL.md: %v", err)
	}
	content := string(payload)
	for _, expected := range []string{
		"name: unui",
		"description:",
		"unui ask",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("SKILL.md is missing %q:\n%s", expected, content)
		}
	}
}
