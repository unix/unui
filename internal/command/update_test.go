package command

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/unix/unui-cli/internal/buildinfo"
	"github.com/unix/unui-cli/internal/installation"
	"github.com/unix/unui-cli/internal/output"
)

func TestUpdateJSONShowsInstallScriptCommand(t *testing.T) {
	result, stderr := runUpdateCommand(t, verifiedInstallScriptInfo())

	if stderr != "" {
		t.Fatalf("stderr must be empty in JSON mode: %q", stderr)
	}
	if result.Command != installation.InstallScriptUpdateCommand {
		t.Fatalf("unexpected install.sh update command: %q", result.Command)
	}
	if result.CurrentVersion != testBuildInfo.Version {
		t.Fatalf("unexpected current version: %q", result.CurrentVersion)
	}
	if result.LatestVersion != "0.2.0" ||
		!result.UpdateAvailable ||
		!result.CanUpdate {
		t.Fatalf("unexpected update result: %#v", result)
	}
}

func TestUpdateJSONShowsNPMCommand(t *testing.T) {
	info := installation.Info{
		Confidence:     installation.ConfidenceReported,
		ExecutablePath: "/tmp/unui",
		Global:         true,
		Manager:        installation.ManagerPNPM,
		Source:         installation.SourceNPM,
	}
	result, stderr := runUpdateCommand(t, info)

	if stderr != "" {
		t.Fatalf("stderr must be empty in JSON mode: %q", stderr)
	}
	expected := "pnpm update -g --latest @unix/unui"
	if result.Command != expected {
		t.Fatalf("unexpected npm update command: %q", result.Command)
	}
	if !result.UpdateAvailable || !result.CanUpdate {
		t.Fatalf("expected an available npm update: %#v", result)
	}
}

func TestUpdateTemporaryNPMRunDoesNotSuggestGlobalUpdate(t *testing.T) {
	info := installation.Info{
		Confidence:     installation.ConfidenceReported,
		ExecutablePath: "/tmp/unui",
		Manager:        installation.ManagerNPM,
		Source:         installation.SourceNPM,
		Temporary:      true,
	}
	result, stderr := runUpdateCommand(t, info)

	if stderr != "" {
		t.Fatalf("stderr must be empty in JSON mode: %q", stderr)
	}
	if result.Command != "" {
		t.Fatalf("temporary npm run must not have an update command: %q", result.Command)
	}
	if !result.UpdateAvailable || result.CanUpdate {
		t.Fatalf("unexpected temporary npm update result: %#v", result)
	}
}

func TestUpdateReportsAlreadyLatestWithoutDetectingInstallation(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	detectCalls := 0
	application := &app{
		buildInfo: testBuildInfo,
		detectInstall: func() (installation.Info, error) {
			detectCalls++
			return installation.Info{}, nil
		},
		fetchRelease: func(context.Context) (releaseInfo, error) {
			return releaseInfo{
				URL:     "https://github.com/unix/unui-cli/releases/tag/v0.1.0",
				Version: "v0.1.0",
			}, nil
		},
		noColor: true,
		stderr:  &stderr,
		stdout:  &stdout,
	}
	command := application.updateCommand()
	if err := command.Execute(); err != nil {
		t.Fatalf("execute update command: %v", err)
	}

	if detectCalls != 0 {
		t.Fatalf("installation detection must not run, received %d calls", detectCalls)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty: %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "Already up to date") ||
		!strings.Contains(stdout.String(), "v0.1.0") {
		t.Fatalf("latest version message was not printed:\n%s", stdout.String())
	}
}

func TestUpdateJSONTreatsNewerLocalVersionAsCurrent(t *testing.T) {
	buildInfo := testBuildInfo
	buildInfo.Version = "0.3.0"
	result, stderr := runUpdateCommandWithRelease(
		t,
		buildInfo,
		verifiedInstallScriptInfo(),
		releaseInfo{
			URL:     "https://github.com/unix/unui-cli/releases/tag/v0.2.0",
			Version: "v0.2.0",
		},
	)

	if stderr != "" {
		t.Fatalf("stderr must be empty in JSON mode: %q", stderr)
	}
	if result.UpdateAvailable || result.CanUpdate || result.Command != "" {
		t.Fatalf("newer local version must not update: %#v", result)
	}
	if result.LatestVersion != "0.2.0" {
		t.Fatalf("unexpected latest version: %q", result.LatestVersion)
	}
}

func TestFetchLatestGitHubRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodGet {
				t.Fatalf("unexpected request method: %s", request.Method)
			}
			if request.Header.Get("Accept") != "application/vnd.github+json" {
				t.Fatalf("unexpected Accept header: %q", request.Header.Get("Accept"))
			}
			if request.Header.Get("User-Agent") != githubUserAgent {
				t.Fatalf(
					"unexpected User-Agent header: %q",
					request.Header.Get("User-Agent"),
				)
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(
				`{"tag_name":"v1.2.3","html_url":"https://example.com/v1.2.3"}`,
			))
		},
	))
	defer server.Close()

	release, err := fetchLatestGitHubRelease(
		context.Background(),
		server.Client(),
		server.URL,
	)
	if err != nil {
		t.Fatalf("fetch latest GitHub release: %v", err)
	}
	if release.Version != "v1.2.3" ||
		release.URL != "https://example.com/v1.2.3" {
		t.Fatalf("unexpected release: %#v", release)
	}
}

func TestUpdateInstallPowerShellPrintsCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	application := &app{
		buildInfo: testBuildInfo,
		detectInstall: func() (installation.Info, error) {
			return verifiedInstallPowerShellInfo(), nil
		},
		fetchRelease: func(context.Context) (releaseInfo, error) {
			return releaseInfo{Version: "v0.2.0"}, nil
		},
		noColor: true,
		stderr:  &stderr,
		stdout:  &stdout,
	}
	command := application.updateCommand()
	if err := command.Execute(); err != nil {
		t.Fatalf("execute update command: %v", err)
	}

	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty: %q", stderr.String())
	}
	if !strings.Contains(
		stdout.String(),
		installation.InstallPowerShellUpdateCommand,
	) {
		t.Fatalf("PowerShell update command was not printed:\n%s", stdout.String())
	}
}

func runUpdateCommand(
	t *testing.T,
	info installation.Info,
) (updateResult, string) {
	t.Helper()
	return runUpdateCommandWithRelease(
		t,
		testBuildInfo,
		info,
		releaseInfo{
			URL:     "https://github.com/unix/unui-cli/releases/tag/v0.2.0",
			Version: "v0.2.0",
		},
	)
}

func runUpdateCommandWithRelease(
	t *testing.T,
	buildInfo buildinfo.Info,
	info installation.Info,
	release releaseInfo,
) (updateResult, string) {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	application := &app{
		buildInfo: buildInfo,
		detectInstall: func() (installation.Info, error) {
			return info, nil
		},
		fetchRelease: func(context.Context) (releaseInfo, error) {
			return release, nil
		},
		json:    true,
		noColor: true,
		stderr:  &stderr,
		stdout:  &stdout,
	}
	command := application.updateCommand()
	if err := command.Execute(); err != nil {
		t.Fatalf("execute update command: %v", err)
	}

	var envelope struct {
		SchemaVersion string       `json:"schemaVersion"`
		OK            bool         `json:"ok"`
		Data          updateResult `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode update output: %v\n%s", err, stdout.String())
	}
	if !envelope.OK || envelope.SchemaVersion != output.SchemaVersion {
		t.Fatalf("unexpected update envelope: %#v", envelope)
	}
	return envelope.Data, stderr.String()
}
