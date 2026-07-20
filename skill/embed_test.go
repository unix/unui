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
		"Do not inspect or collect project contents",
		"desired page content and approximate structure",
		"elements it should include and their rough functions",
		"Longer, relevant prompts",
		"Never send project contents to unUI",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("SKILL.md is missing %q:\n%s", expected, content)
		}
	}
}

func TestTextReturnsSkillMarkdown(t *testing.T) {
	content, err := Text()
	if err != nil {
		t.Fatalf("read embedded SKILL.md: %v", err)
	}
	if !strings.Contains(content, "name: unui") {
		t.Fatalf("unexpected skill text:\n%s", content)
	}
}
