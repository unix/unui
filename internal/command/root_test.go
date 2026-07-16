package command

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/unix/unui/internal/buildinfo"
	cliconfig "github.com/unix/unui/internal/config"
	"github.com/unix/unui/internal/output"
	"github.com/unix/unui/internal/store"
)

var testBuildInfo = buildinfo.Info{
	Version:   "0.1.0",
	Commit:    "abcdef1234567890",
	Date:      "2026-07-16T08:20:00Z",
	Dirty:     false,
	GoVersion: "go1.25.8",
}

func TestVersionJSONWritesOneEnvelopeAndNoStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := execute(
		[]string{"version", "--json"},
		&stdout,
		&stderr,
		testBuildInfo,
	)
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty in JSON mode: %q", stderr.String())
	}

	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not one JSON document: %v", err)
	}
	if !envelope.OK || envelope.SchemaVersion != output.SchemaVersion {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	if envelope.ExitCode != 0 {
		t.Fatalf("unexpected JSON exit code: %d", envelope.ExitCode)
	}
	data, ok := envelope.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected version data: %#v", envelope.Data)
	}
	if data["version"] != "0.1.0" ||
		data["commit"] != "abcdef1234567890" ||
		data["date"] != "2026-07-16T08:20:00Z" ||
		data["dirty"] != false ||
		data["go"] != "go1.25.8" {
		t.Fatalf("unexpected version data: %#v", data)
	}
}

func TestCommandErrorJSONWritesOneEnvelopeAndNoStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute([]string{"ask", "--json"}, &stdout, &stderr)
	if exitCode == 0 {
		t.Fatal("expected a non-zero exit code")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty in JSON mode: %q", stderr.String())
	}

	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not one JSON document: %v", err)
	}
	if envelope.OK || envelope.Error == nil {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	if envelope.ExitCode != exitCode {
		t.Fatalf(
			"JSON exit code %d does not match process exit code %d",
			envelope.ExitCode,
			exitCode,
		)
	}
	if envelope.Error.Title != "Missing argument" {
		t.Fatalf("unexpected error title: %#v", envelope.Error)
	}
	if envelope.Error.Hint == "" {
		t.Fatalf("expected a recovery hint: %#v", envelope.Error)
	}
}

func TestHelpUsesStructuredLayout(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := execute(
		[]string{"--help", "--no-color"},
		&stdout,
		&stderr,
		testBuildInfo,
	)
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty: %q", stderr.String())
	}

	output := stdout.String()
	for _, expected := range []string{
		"unUI v0.1.0",
		"Usage",
		"$ unui [command] [options]",
		"Commands",
		"auth [command]",
		"config [command]",
		"skill [command]",
		"uninstall",
		"update",
		"Options",
		"Examples",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("help is missing %q:\n%s", expected, output)
		}
	}
	if strings.Contains(output, "update-skill") {
		t.Fatalf("help must not expose the replaced update-skill command:\n%s", output)
	}
	if strings.Contains(output, "\x1b[") {
		t.Fatalf("--no-color must disable ANSI output: %q", output)
	}
	if strings.Contains(output, "--profile") {
		t.Fatalf("help must not expose credential profiles:\n%s", output)
	}
	if strings.Contains(output, "--api-url") {
		t.Fatalf("help must not expose the removed API URL option:\n%s", output)
	}
	if strings.Contains(output, "--timeout") {
		t.Fatalf("help must not expose a configurable timeout:\n%s", output)
	}
	if strings.Contains(output, "completion <shell>") {
		t.Fatalf("help must not expose the completion command:\n%s", output)
	}
}

func TestCompletionIsHiddenButAvailable(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := execute(
		[]string{"completion", "zsh"},
		&stdout,
		&stderr,
		testBuildInfo,
	)
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty: %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "#compdef unui") {
		t.Fatalf("unexpected completion script:\n%s", stdout.String())
	}
}

func TestVersionUsesDetailedHumanOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	info := testBuildInfo
	info.Dirty = true
	exitCode := execute(
		[]string{"version", "--no-color"},
		&stdout,
		&stderr,
		info,
	)
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty: %q", stderr.String())
	}
	for _, expected := range []string{
		"unUI 0.1.0",
		"commit abcdef123456 (dirty)",
		"built 2026-07-16T08:20:00Z",
		"runtime go1.25.8",
	} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("version output is missing %q:\n%s", expected, stdout.String())
		}
	}
}

func TestProfileOptionIsNotAvailable(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(
		[]string{"auth", "login", "--profile", "work", "--no-color"},
		&stdout,
		&stderr,
	)
	if exitCode == 0 {
		t.Fatal("expected a non-zero exit code")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout must be empty: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Unknown option") {
		t.Fatalf("unexpected error:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "`--profile`") {
		t.Fatalf("expected removed option name:\n%s", stderr.String())
	}
}

func TestTimeoutOptionIsNotAvailable(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(
		[]string{"version", "--timeout", "1m", "--no-color"},
		&stdout,
		&stderr,
	)
	if exitCode == 0 {
		t.Fatal("expected a non-zero exit code")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout must be empty: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Unknown option") {
		t.Fatalf("unexpected error:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "`--timeout`") {
		t.Fatalf("expected removed option name:\n%s", stderr.String())
	}
}

func TestCommandTimeoutIsFixedAtThirtySeconds(t *testing.T) {
	application := &app{registry: cliconfig.DefaultRegistry}
	if timeout := application.client().HTTPClient.Timeout; timeout != 30*time.Second {
		t.Fatalf("unexpected HTTP timeout: %s", timeout)
	}

	ctx, cancel := application.context(context.Background())
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected a command deadline")
	}
	remaining := time.Until(deadline)
	if remaining < 29*time.Second || remaining > 30*time.Second {
		t.Fatalf("unexpected command timeout: %s", remaining)
	}
}

func TestAuthorizationTimeoutIsFixedAtFiveMinutes(t *testing.T) {
	application := &app{}
	ctx, cancel := application.authorizationContext(context.Background())
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected an authorization deadline")
	}
	remaining := time.Until(deadline)
	if remaining < 4*time.Minute+59*time.Second || remaining > 5*time.Minute {
		t.Fatalf("unexpected authorization timeout: %s", remaining)
	}
}

func TestHelpJSONWritesStructuredData(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute([]string{"ask", "--help", "--json"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty in JSON mode: %q", stderr.String())
	}

	var envelope struct {
		SchemaVersion string              `json:"schemaVersion"`
		OK            bool                `json:"ok"`
		Data          output.HelpDocument `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not one JSON document: %v", err)
	}
	if !envelope.OK {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	if envelope.Data.Command != "unui ask" {
		t.Fatalf("unexpected help command: %#v", envelope.Data)
	}
	if envelope.Data.Usage != "unui ask <task> [options]" {
		t.Fatalf("unexpected usage: %#v", envelope.Data)
	}
}

func TestUnknownCommandUsesFriendlyError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute([]string{"missing", "--no-color"}, &stdout, &stderr)
	if exitCode == 0 {
		t.Fatal("expected a non-zero exit code")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout must be empty: %q", stdout.String())
	}

	output := stderr.String()
	for _, expected := range []string{
		"Unknown command",
		"`missing` is not a command under `unui`.",
		"Run `unui --help`",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("error is missing %q:\n%s", expected, output)
		}
	}
}

func TestInvalidFlagUsesFriendlyError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(
		[]string{"ask", "Create a dashboard", "--limit", "many", "--no-color"},
		&stdout,
		&stderr,
	)
	if exitCode == 0 {
		t.Fatal("expected a non-zero exit code")
	}
	if !strings.Contains(stderr.String(), "Invalid option value") {
		t.Fatalf("unexpected error:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "`--limit`") {
		t.Fatalf("expected option name:\n%s", stderr.String())
	}
}

func TestJSONFalseKeepsHumanOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(
		[]string{"version", "--json=false", "--no-color"},
		&stdout,
		&stderr,
	)
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty: %q", stderr.String())
	}
	if strings.HasPrefix(strings.TrimSpace(stdout.String()), "{") {
		t.Fatalf("--json=false must keep human output: %q", stdout.String())
	}
}

func TestNoColorEnvironmentDisablesForcedColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "1")
	t.Setenv("TERM", "xterm-256color")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute([]string{"version"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("NO_COLOR must disable ANSI output: %q", stdout.String())
	}
}

func TestVerboseErrorIncludesTechnicalDetails(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(
		[]string{"ask", "--verbose", "--no-color"},
		&stdout,
		&stderr,
	)
	if exitCode == 0 {
		t.Fatal("expected a non-zero exit code")
	}
	for _, expected := range []string{"Details", `"expected"`, `"task"`} {
		if !strings.Contains(stderr.String(), expected) {
			t.Fatalf("verbose error is missing %q:\n%s", expected, stderr.String())
		}
	}
}

func TestJSONFlagIsHonoredAfterInvalidOption(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(
		[]string{"--unknown", "--json"},
		&stdout,
		&stderr,
	)
	if exitCode == 0 {
		t.Fatal("expected a non-zero exit code")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty in JSON mode: %q", stderr.String())
	}

	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not one JSON document: %v", err)
	}
	if envelope.Error == nil || envelope.Error.Code != "UNKNOWN_OPTION" {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
}

func TestLoginHelpDocumentsRegistryOption(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(
		[]string{"auth", "login", "--help", "--no-color"},
		&stdout,
		&stderr,
	)
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty: %q", stderr.String())
	}
	for _, expected := range []string{
		"--registry <string>",
		"https://api.unui.cc",
		"http://127.0.0.1:3001",
	} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("login help is missing %q:\n%s", expected, stdout.String())
		}
	}
}

func TestConfigSetAndResetRegistry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("UNUI_CONFIG_PATH", path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(
		[]string{
			"config",
			"set",
			"--registry",
			"http://127.0.0.1:3001/",
			"--json",
		},
		&stdout,
		&stderr,
	)
	if exitCode != 0 {
		t.Fatalf("unexpected set exit code: %d\n%s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty in JSON mode: %q", stderr.String())
	}

	configStore := cliconfig.Store{FilePath: path}
	values, err := configStore.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if values.EffectiveRegistry() != "http://127.0.0.1:3001" {
		t.Fatalf("unexpected registry: %q", values.EffectiveRegistry())
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = Execute(
		[]string{"config", "reset", "--registry", "--json"},
		&stdout,
		&stderr,
	)
	if exitCode != 0 {
		t.Fatalf("unexpected reset exit code: %d\n%s", exitCode, stderr.String())
	}
	values, err = configStore.Load()
	if err != nil {
		t.Fatalf("load reset config: %v", err)
	}
	if values.EffectiveRegistry() != cliconfig.DefaultRegistry {
		t.Fatalf("unexpected reset registry: %q", values.EffectiveRegistry())
	}
}

func TestConfigShowIncludesEffectiveValueSourceAndPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("UNUI_CONFIG_PATH", path)
	configStore := cliconfig.Store{FilePath: path}
	if _, err := configStore.SetRegistry("http://127.0.0.1:3001"); err != nil {
		t.Fatalf("set registry: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(
		[]string{"config", "show", "--json"},
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
		Data struct {
			ConfigFile string `json:"configFile"`
			Registry   string `json:"registry"`
			Source     string `json:"source"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not one JSON document: %v", err)
	}
	if envelope.Data.ConfigFile != path {
		t.Fatalf("unexpected config path: %q", envelope.Data.ConfigFile)
	}
	if envelope.Data.Registry != "http://127.0.0.1:3001" {
		t.Fatalf("unexpected registry: %q", envelope.Data.Registry)
	}
	if envelope.Data.Source != "configFile" {
		t.Fatalf("unexpected registry source: %q", envelope.Data.Source)
	}
}

func TestConfigShowShortensConfigPathUnderUserHomeInHumanMode(t *testing.T) {
	userHome := t.TempDir()
	path := filepath.Join(userHome, ".unui", "config.json")
	t.Setenv("HOME", userHome)
	t.Setenv("UNUI_CONFIG_PATH", path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(
		[]string{"config", "show", "--no-color"},
		&stdout,
		&stderr,
	)
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d\n%s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty: %q", stderr.String())
	}

	expectedPath := path
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		expectedPath = filepath.Join("~", ".unui", "config.json")
	}
	expectedLine := "➜ Config file  " + expectedPath
	if !strings.Contains(stdout.String(), expectedLine) {
		t.Fatalf("config output is missing %q:\n%s", expectedLine, stdout.String())
	}
}

func TestConfigGetRegistryWritesOnlyValueInHumanMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("UNUI_CONFIG_PATH", path)
	if _, err := (cliconfig.Store{FilePath: path}).SetRegistry(
		"http://127.0.0.1:3001",
	); err != nil {
		t.Fatalf("set registry: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(
		[]string{"config", "get", "--registry", "--no-color"},
		&stdout,
		&stderr,
	)
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d\n%s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty: %q", stderr.String())
	}
	if stdout.String() != "http://127.0.0.1:3001\n" {
		t.Fatalf("unexpected registry output: %q", stdout.String())
	}
}

func TestConfigPathDoesNotRequireConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "config.json")
	t.Setenv("UNUI_CONFIG_PATH", path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(
		[]string{"config", "path", "--no-color"},
		&stdout,
		&stderr,
	)
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d\n%s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty: %q", stderr.String())
	}
	if stdout.String() != path+"\n" {
		t.Fatalf("unexpected config path: %q", stdout.String())
	}
}

func TestLoginRegistryOptionIsSavedAndUsedForRequests(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config.json")
	credentialsPath := filepath.Join(directory, "credentials.json")
	t.Setenv("UNUI_CONFIG_PATH", configPath)
	t.Setenv("UNUI_CREDENTIALS_PATH", credentialsPath)

	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			requestedPath = request.URL.Path
			http.Error(writer, `{"message":"stop before browser"}`, http.StatusInternalServerError)
		},
	))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(
		[]string{
			"auth",
			"login",
			"--registry",
			server.URL,
			"--json",
		},
		&stdout,
		&stderr,
	)
	if exitCode == 0 {
		t.Fatal("expected the server error to fail login")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty in JSON mode: %q", stderr.String())
	}
	if requestedPath != "/v1/cli/auth/requests" {
		t.Fatalf("unexpected request path: %q", requestedPath)
	}

	values, err := (cliconfig.Store{FilePath: configPath}).Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if values.EffectiveRegistry() != server.URL {
		t.Fatalf("unexpected saved registry: %q", values.EffectiveRegistry())
	}
	credentials, err := store.Load()
	if err != nil {
		t.Fatalf("load credentials: %v", err)
	}
	if credentials.Registry != server.URL {
		t.Fatalf("unexpected credential registry: %q", credentials.Registry)
	}
}

func TestBindingCredentialsToAnotherRegistryClearsTokens(t *testing.T) {
	credentials := store.Credentials{
		AccessToken:          "access-token",
		AccessTokenExpiresAt: "2099-01-01T00:00:00Z",
		PersonalToken:        "personal-token",
		PersonalTokenExpires: "2099-01-01T00:00:00Z",
		Registry:             cliconfig.DefaultRegistry,
	}

	bindCredentialsToRegistry(&credentials, "http://127.0.0.1:3001")

	if credentials.AccessToken != "" || credentials.PersonalToken != "" {
		t.Fatalf("tokens must be cleared: %#v", credentials)
	}
	if credentials.Registry != "http://127.0.0.1:3001" {
		t.Fatalf("unexpected registry: %q", credentials.Registry)
	}
}
