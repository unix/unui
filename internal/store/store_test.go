package store

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestStoreSavesLoadsAndDeletesCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unui", "credentials.json")
	credentialStore := Store{FilePath: path}
	expected := Credentials{
		AccessToken:          "access-token",
		AccessTokenExpiresAt: "2099-01-01T00:00:00Z",
		DeviceID:             "device-id",
		DeviceName:           "device-name",
		Platform:             "test/test",
		PrivateKey:           "private-key",
		PublicKey:            "public-key",
		PersonalToken:        "personal-token",
		PersonalTokenExpires: "2099-01-01T00:00:00Z",
		Registry:             "https://api.unui.cc",
	}

	if err := credentialStore.Save(expected); err != nil {
		t.Fatalf("save credentials: %v", err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat credentials: %v", err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("unexpected credentials permissions: %o", info.Mode().Perm())
		}
	}

	actual, err := credentialStore.Load()
	if err != nil {
		t.Fatalf("load credentials: %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("unexpected credentials: %#v", actual)
	}

	if err := credentialStore.Delete(); err != nil {
		t.Fatalf("delete credentials: %v", err)
	}
	if _, err := credentialStore.Load(); !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("expected missing credentials, got: %v", err)
	}
}

func TestStoreUsesCredentialsPathEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	t.Setenv(credentialsPathEnvironment, path)

	actual, err := DefaultStore().Path()
	if err != nil {
		t.Fatalf("resolve credentials path: %v", err)
	}
	if actual != path {
		t.Fatalf("unexpected credentials path: %q", actual)
	}
}

func TestStoreUsesUnuiHome(t *testing.T) {
	directory := t.TempDir()
	t.Setenv(credentialsPathEnvironment, "")
	t.Setenv("UNUI_HOME", directory)

	actual, err := DefaultStore().Path()
	if err != nil {
		t.Fatalf("resolve credentials path: %v", err)
	}
	expected := filepath.Join(directory, "credentials.json")
	if actual != expected {
		t.Fatalf("unexpected credentials path: %q", actual)
	}
}

func TestStoreCredentialsPathEnvironmentOverridesUnuiHome(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "override.json")
	t.Setenv(credentialsPathEnvironment, path)
	t.Setenv("UNUI_HOME", filepath.Join(directory, ".unui"))

	actual, err := DefaultStore().Path()
	if err != nil {
		t.Fatalf("resolve credentials path: %v", err)
	}
	if actual != path {
		t.Fatalf("unexpected credentials path: %q", actual)
	}
}

func TestStoreRepairsCredentialsPermissionsOnLoad(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX file permissions")
	}
	path := filepath.Join(t.TempDir(), "credentials.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write credentials: %v", err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("set credentials permissions: %v", err)
	}

	if _, err := (Store{FilePath: path}).Load(); err != nil {
		t.Fatalf("load credentials: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat credentials: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("unexpected repaired permissions: %o", info.Mode().Perm())
	}
}
