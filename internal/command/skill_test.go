package command

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	bundledskill "github.com/unix/unui/skill"
)

func TestSkillUpdateInstallsBundledSkillForCodex(t *testing.T) {
	userHome := t.TempDir()
	t.Setenv("HOME", userHome)
	t.Setenv("USERPROFILE", userHome)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(
		[]string{"skill", "update", "--client", "codex", "--json"},
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

func TestSkillUpdateRejectsUnsupportedClient(t *testing.T) {
	userHome := t.TempDir()
	t.Setenv("HOME", userHome)
	t.Setenv("USERPROFILE", userHome)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(
		[]string{"skill", "update", "--client", "other", "--json"},
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

func TestSkillUpdateSupportsAllClients(t *testing.T) {
	userHome := t.TempDir()
	t.Setenv("HOME", userHome)
	t.Setenv("USERPROFILE", userHome)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(
		[]string{"skill", "update", "--client", "all", "--json"},
		&stdout,
		&stderr,
	)
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d\n%s", exitCode, stderr.String())
	}

	var envelope struct {
		Data updateSkillData `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not one JSON document: %v", err)
	}
	if len(envelope.Data.Targets) != 3 {
		t.Fatalf("unexpected targets: %#v", envelope.Data.Targets)
	}
	for _, target := range envelope.Data.Targets {
		if _, err := os.Stat(filepath.Join(target.Path, "SKILL.md")); err != nil {
			t.Fatalf("%s skill was not created: %v", target.Client, err)
		}
	}
}

func TestSkillShowPrintsBundledSkillText(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(
		[]string{"skill", "show", "--no-color"},
		&stdout,
		&stderr,
	)
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d\n%s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty: %q", stderr.String())
	}
	text, err := bundledskill.Text()
	if err != nil {
		t.Fatalf("read bundled skill text: %v", err)
	}
	if stdout.String() != text {
		t.Fatalf("unexpected skill text:\n%s", stdout.String())
	}
}

func TestSkillShowJSONIncludesVersionAndText(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(
		[]string{"skill", "show", "--json"},
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
		Data showSkillData `json:"data"`
		OK   bool          `json:"ok"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not one JSON document: %v", err)
	}
	if !envelope.OK ||
		envelope.Data.SkillVersion == "" ||
		!strings.Contains(envelope.Data.Text, "name: unui") {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
}
