package buildinfo

import (
	"runtime/debug"
	"testing"
)

func TestResolveUsesStampedReleaseInformation(t *testing.T) {
	info := resolve(
		stamp{
			version: "1.2.3",
			commit:  "release-commit",
			date:    "2026-07-16T08:20:00Z",
		},
		"go1.25.8",
		&debug.BuildInfo{
			GoVersion: "go1.26.0",
			Main: debug.Module{
				Version: "v9.9.9",
			},
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "local-commit"},
				{Key: "vcs.modified", Value: "false"},
			},
		},
	)

	if info.Version != "1.2.3" {
		t.Fatalf("unexpected version: %q", info.Version)
	}
	if info.Commit != "release-commit" {
		t.Fatalf("unexpected commit: %q", info.Commit)
	}
	if info.Date != "2026-07-16T08:20:00Z" {
		t.Fatalf("unexpected date: %q", info.Date)
	}
	if info.GoVersion != "go1.26.0" {
		t.Fatalf("unexpected Go version: %q", info.GoVersion)
	}
	if info.Dirty {
		t.Fatal("release build must be clean")
	}
}

func TestResolveUsesGoBuildInformation(t *testing.T) {
	info := resolve(
		stamp{version: "dev"},
		"go1.25.8",
		&debug.BuildInfo{
			GoVersion: "go1.25.8",
			Main: debug.Module{
				Version: "v0.2.0-0.20260716082000-abcdef123456+dirty",
			},
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "abcdef1234567890"},
				{Key: "vcs.time", Value: "2026-07-16T08:20:00Z"},
				{Key: "vcs.modified", Value: "true"},
			},
		},
	)

	if info.Version != "0.2.0-0.20260716082000-abcdef123456+dirty" {
		t.Fatalf("unexpected version: %q", info.Version)
	}
	if info.Commit != "abcdef1234567890" {
		t.Fatalf("unexpected commit: %q", info.Commit)
	}
	if info.Date != "2026-07-16T08:20:00Z" {
		t.Fatalf("unexpected date: %q", info.Date)
	}
	if !info.Dirty {
		t.Fatal("expected dirty build")
	}
}

func TestResolveFallsBackToDevelopmentVersion(t *testing.T) {
	for _, build := range []*debug.BuildInfo{
		nil,
		{
			Main: debug.Module{
				Version: "(devel)",
			},
		},
	} {
		info := resolve(stamp{}, "go1.25.8", build)
		if info.Version != "dev" {
			t.Fatalf("unexpected version: %q", info.Version)
		}
		if info.GoVersion != "go1.25.8" {
			t.Fatalf("unexpected Go version: %q", info.GoVersion)
		}
	}
}
