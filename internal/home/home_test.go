package home

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryUsesUnuiHomeEnvironment(t *testing.T) {
	directory := t.TempDir()
	t.Setenv(environment, directory)

	actual, err := Directory()
	if err != nil {
		t.Fatalf("resolve unUI home: %v", err)
	}
	if actual != directory {
		t.Fatalf("unexpected unUI home: %q", actual)
	}
}

func TestDirectoryDefaultsToDotUnuiUnderUserHome(t *testing.T) {
	t.Setenv(environment, "")
	userHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("resolve user home: %v", err)
	}

	actual, err := Directory()
	if err != nil {
		t.Fatalf("resolve unUI home: %v", err)
	}
	expected := filepath.Join(userHome, ".unui")
	if actual != expected {
		t.Fatalf("unexpected unUI home: %q", actual)
	}
}

func TestDirectoryRejectsRelativeEnvironmentPath(t *testing.T) {
	t.Setenv(environment, "relative/unui")

	if _, err := Directory(); !errors.Is(err, ErrRelativePath) {
		t.Fatalf("expected relative path error, got: %v", err)
	}
}

func TestPathJoinsFileNameToUnuiHome(t *testing.T) {
	directory := t.TempDir()
	t.Setenv(environment, directory)

	actual, err := Path("config.json")
	if err != nil {
		t.Fatalf("resolve config path: %v", err)
	}
	expected := filepath.Join(directory, "config.json")
	if actual != expected {
		t.Fatalf("unexpected config path: %q", actual)
	}
}

func TestDisplayPathShortensPathsUnderUserHome(t *testing.T) {
	userHome := filepath.Join(string(filepath.Separator), "Users", "test")
	path := filepath.Join(userHome, ".unui", "config.json")

	actual := displayPath(path, userHome)
	expected := filepath.Join("~", ".unui", "config.json")
	if actual != expected {
		t.Fatalf("unexpected display path: %q", actual)
	}
}

func TestDisplayPathShortensUserHome(t *testing.T) {
	userHome := filepath.Join(string(filepath.Separator), "Users", "test")

	actual := displayPath(userHome, userHome)
	if actual != "~" {
		t.Fatalf("unexpected display path: %q", actual)
	}
}

func TestDisplayPathKeepsPathsOutsideUserHome(t *testing.T) {
	userHome := filepath.Join(string(filepath.Separator), "Users", "test")
	path := filepath.Join(string(filepath.Separator), "Users", "testing", "config.json")

	actual := displayPath(path, userHome)
	if actual != path {
		t.Fatalf("unexpected display path: %q", actual)
	}
}
