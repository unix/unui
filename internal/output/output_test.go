package output

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/unix/unui-cli/internal/buildinfo"
)

func TestStatusUsesColorWhenForced(t *testing.T) {
	restoreEnvironment(t, "NO_COLOR")
	if err := os.Unsetenv("NO_COLOR"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLICOLOR_FORCE", "1")
	t.Setenv("TERM", "xterm-256color")

	var stdout bytes.Buffer
	printer := Printer{Stdout: &stdout}
	if output := printer.Done("Ready", "checks passed"); !strings.Contains(output, "\x1b[") {
		t.Fatalf("expected ANSI color output: %q", output)
	}
}

func TestNoColorDisablesForcedColor(t *testing.T) {
	t.Setenv("CLICOLOR_FORCE", "1")
	t.Setenv("TERM", "xterm-256color")

	var stdout bytes.Buffer
	printer := Printer{
		NoColor: true,
		Stdout:  &stdout,
	}
	if output := printer.Done("Ready", "checks passed"); strings.Contains(output, "\x1b[") {
		t.Fatalf("expected plain output: %q", output)
	}
}

func TestVersionShortensCommitInHumanOutput(t *testing.T) {
	var stdout bytes.Buffer
	printer := Printer{
		NoColor: true,
		Stdout:  &stdout,
	}
	output := printer.Version("unUI", buildinfo.Info{
		Version:   "1.2.3",
		Commit:    "abcdef1234567890",
		Date:      "2026-07-16T08:20:00Z",
		Dirty:     true,
		GoVersion: "go1.25.8",
	})
	for _, expected := range []string{
		"unUI 1.2.3",
		"commit abcdef123456 (dirty)",
		"built 2026-07-16T08:20:00Z",
		"runtime go1.25.8",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("version output is missing %q: %q", expected, output)
		}
	}
}

func restoreEnvironment(t *testing.T, name string) {
	t.Helper()
	value, exists := os.LookupEnv(name)
	t.Cleanup(func() {
		if exists {
			_ = os.Setenv(name, value)
			return
		}
		_ = os.Unsetenv(name)
	})
}
