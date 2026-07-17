package output

import (
	"bytes"
	"os"
	"strings"
	"testing"
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
