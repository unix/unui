package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreUsesDefaultRegistryWhenConfigIsMissing(t *testing.T) {
	configStore := Store{FilePath: filepath.Join(t.TempDir(), "config.json")}

	values, err := configStore.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if values.EffectiveRegistry() != DefaultRegistry {
		t.Fatalf("unexpected registry: %q", values.EffectiveRegistry())
	}
}

func TestStoreSetsAndResetsRegistry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unui", "config.json")
	configStore := Store{FilePath: path}

	registry, err := configStore.SetRegistry("HTTP://127.0.0.1:3001/")
	if err != nil {
		t.Fatalf("set registry: %v", err)
	}
	if registry != "http://127.0.0.1:3001" {
		t.Fatalf("unexpected normalized registry: %q", registry)
	}
	values, err := configStore.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if values.EffectiveRegistry() != registry {
		t.Fatalf("unexpected saved registry: %q", values.EffectiveRegistry())
	}

	if err := configStore.ResetRegistry(); err != nil {
		t.Fatalf("reset registry: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("config file must be removed, got: %v", err)
	}
	values, err = configStore.Load()
	if err != nil {
		t.Fatalf("load reset config: %v", err)
	}
	if values.EffectiveRegistry() != DefaultRegistry {
		t.Fatalf("unexpected reset registry: %q", values.EffectiveRegistry())
	}
}

func TestStoreUsesUnuiHome(t *testing.T) {
	directory := t.TempDir()
	t.Setenv(configPathEnvironment, "")
	t.Setenv("UNUI_HOME", directory)

	actual, err := DefaultStore().Path()
	if err != nil {
		t.Fatalf("resolve config path: %v", err)
	}
	expected := filepath.Join(directory, "config.json")
	if actual != expected {
		t.Fatalf("unexpected config path: %q", actual)
	}
}

func TestStoreConfigPathEnvironmentOverridesUnuiHome(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "override.json")
	t.Setenv(configPathEnvironment, path)
	t.Setenv("UNUI_HOME", filepath.Join(directory, ".unui"))

	actual, err := DefaultStore().Path()
	if err != nil {
		t.Fatalf("resolve config path: %v", err)
	}
	if actual != path {
		t.Fatalf("unexpected config path: %q", actual)
	}
}

func TestNormalizeRegistryRejectsUnsupportedURLs(t *testing.T) {
	for _, value := range []string{
		"",
		"api.unui.cc",
		"ftp://api.unui.cc",
		"https://user:pass@api.unui.cc",
		"https://api.unui.cc?environment=local",
		"https://api.unui.cc#local",
	} {
		if _, err := NormalizeRegistry(value); !errors.Is(
			err,
			ErrInvalidRegistry,
		) {
			t.Fatalf("expected %q to be rejected, got: %v", value, err)
		}
	}
}
