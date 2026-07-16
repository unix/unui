package command

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateSkillInstallsBundledSkillForCodex(t *testing.T) {
	userHome := t.TempDir()
	t.Setenv("HOME", userHome)
	t.Setenv("USERPROFILE", userHome)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(
		[]string{"update-skill", "--client", "codex", "--json"},
		&stdout,
		&stderr,
	)
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d\n%s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty in JSON mode: %q", stderr.String())
	}

	var envelope struct {
		Data updateSkillData `json:"data"`
		OK   bool            `json:"ok"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not one JSON document: %v", err)
	}
	if !envelope.OK || envelope.Data.SkillVersion == "" {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	if len(envelope.Data.Targets) != 1 ||
		envelope.Data.Targets[0].Client != "codex" {
		t.Fatalf("unexpected targets: %#v", envelope.Data.Targets)
	}

	path := filepath.Join(
		userHome,
		".agents",
		"skills",
		"unui",
		"SKILL.md",
	)
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	if !strings.Contains(string(payload), "name: unui") {
		t.Fatalf("unexpected skill content:\n%s", payload)
	}
}

func TestUpdateSkillRejectsUnsupportedClient(t *testing.T) {
	userHome := t.TempDir()
	t.Setenv("HOME", userHome)
	t.Setenv("USERPROFILE", userHome)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(
		[]string{"update-skill", "--client", "other", "--json"},
		&stdout,
		&stderr,
	)
	if exitCode == 0 {
		t.Fatal("expected a non-zero exit code")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty in JSON mode: %q", stderr.String())
	}

	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not one JSON document: %v", err)
	}
	if envelope.Error.Code != "UNSUPPORTED_CLIENT" {
		t.Fatalf("unexpected error: %#v", envelope.Error)
	}
}
