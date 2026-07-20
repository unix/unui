package command

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cliconfig "github.com/unix/unui/internal/config"
	"github.com/unix/unui/internal/proof"
	"github.com/unix/unui/internal/store"
)

func TestAuthShowUsesHumanReadableOutput(t *testing.T) {
	server := authShowTestServer(t)
	defer server.Close()
	prepareAuthShowTest(t, server.URL)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(
		[]string{"auth", "show", "--no-color"},
		&stdout,
		&stderr,
	)
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d\n%s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty: %q", stderr.String())
	}

	output := stdout.String()
	for _, expected := range []string{
		"✓ Signed in  designer@example.com",
		"✓ CLI access  Ready",
		"➜ Membership  PRO · expires",
		"➜ Device  Design Mac · darwin/arm64",
		"➜ Account ID  user-123",
		"➜ Device ID  device-123",
		"➜ Credential expires",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("auth output is missing %q:\n%s", expected, output)
		}
	}
	if strings.Contains(output, "{") || strings.Contains(output, "usage") {
		t.Fatalf("human output must not contain raw JSON or usage: %q", output)
	}
}

func TestAuthShowJSONReturnsStructuredDataWithoutUsage(t *testing.T) {
	server := authShowTestServer(t)
	defer server.Close()
	prepareAuthShowTest(t, server.URL)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(
		[]string{"auth", "show", "--json"},
		&stdout,
		&stderr,
	)
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d\n%s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty: %q", stderr.String())
	}

	var envelope struct {
		Data map[string]any `json:"data"`
		OK   bool           `json:"ok"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not one JSON document: %v", err)
	}
	if !envelope.OK {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	if _, exists := envelope.Data["usage"]; exists {
		t.Fatalf("auth show JSON must not include usage: %#v", envelope.Data)
	}
	account, ok := envelope.Data["account"].(map[string]any)
	if !ok || account["email"] != "designer@example.com" {
		t.Fatalf("unexpected account data: %#v", envelope.Data["account"])
	}
}

func TestAuthStatusCommandIsRemoved(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(
		[]string{"auth", "status", "--json"},
		&stdout,
		&stderr,
	)
	if exitCode == 0 {
		t.Fatal("expected auth status to fail")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty in JSON mode: %q", stderr.String())
	}
}

func TestDoctorVerifiesCachedAccessTokenWithAPI(t *testing.T) {
	statusRequested := false
	refreshRequested := false
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/v1/cli/status":
				if request.Method != http.MethodGet {
					http.NotFound(writer, request)
					return
				}
				statusRequested = true
				http.Error(writer, "unauthorized", http.StatusUnauthorized)
			case "/v1/cli/auth/access-tokens":
				if request.Method != http.MethodPost {
					http.NotFound(writer, request)
					return
				}
				refreshRequested = true
				http.Error(writer, "unauthorized", http.StatusUnauthorized)
			default:
				http.NotFound(writer, request)
			}
		},
	))
	defer server.Close()

	device, err := proof.NewDevice()
	if err != nil {
		t.Fatalf("create test device: %v", err)
	}
	directory := t.TempDir()
	t.Setenv("UNUI_CONFIG_PATH", filepath.Join(directory, "config.json"))
	t.Setenv("UNUI_CREDENTIALS_PATH", filepath.Join(directory, "credentials.json"))
	if _, err := cliconfig.DefaultStore().SetRegistry(server.URL); err != nil {
		t.Fatalf("set registry: %v", err)
	}
	if err := store.Save(store.Credentials{
		DeviceID:   device.DeviceID,
		DeviceName: "Design Mac",
		Platform:   "darwin/arm64",
		PrivateKey: device.PrivateKey,
		PublicKey:  device.PublicKey,
		Registries: map[string]store.RegistryCredentials{
			server.URL: {
				AccessToken:          "revoked-access-token",
				AccessTokenExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339Nano),
				PersonalToken:        "revoked-personal-token",
			},
		},
	}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute([]string{"doctor", "--json"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d\n%s", exitCode, stderr.String())
	}
	if !statusRequested {
		t.Fatal("doctor must verify the cached access token with the API")
	}
	if !refreshRequested {
		t.Fatal("doctor must try to refresh a rejected access token")
	}
	if strings.Contains(stdout.String(), server.URL) || strings.Contains(stderr.String(), server.URL) {
		t.Fatalf("doctor output exposed registry: stdout=%s stderr=%s", stdout.String(), stderr.String())
	}

	var envelope struct {
		Data struct {
			AccessIssue map[string]string `json:"accessIssue"`
			AccessReady bool              `json:"accessReady"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not one JSON document: %v", err)
	}
	if envelope.Data.AccessReady {
		t.Fatalf("revoked access token must not be ready: %#v", envelope.Data)
	}
	if envelope.Data.AccessIssue["code"] != "AUTH_REQUIRED" {
		t.Fatalf("unexpected access issue: %#v", envelope.Data.AccessIssue)
	}
}

func TestDoctorFailsWhenCLIUpdateIsRequired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			if request.Header.Get("x-unui-cli-version") != testBuildInfo.Version {
				t.Fatalf(
					"unexpected CLI version header: %q",
					request.Header.Get("x-unui-cli-version"),
				)
			}
			writer.Header().Set("x-unui-cli-min-version", "0.2.0")
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{}`))
		},
	))
	defer server.Close()
	prepareAuthShowTest(t, server.URL)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := execute(
		[]string{"doctor", "--json"},
		&stdout,
		&stderr,
		testBuildInfo,
	)
	if exitCode != 1 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty in JSON mode: %q", stderr.String())
	}

	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode doctor output: %v\n%s", err, stdout.String())
	}
	if envelope.OK || envelope.Error.Code != "CLI_UPDATE_REQUIRED" {
		t.Fatalf("unexpected doctor result: %#v", envelope)
	}
}

func TestLogoutStillDeletesLocalCredentialsWhenCLIUpdateIsRequired(
	t *testing.T,
) {
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("x-unui-cli-min-version", "0.2.0")
			writer.WriteHeader(http.StatusUpgradeRequired)
		},
	))
	defer server.Close()

	device, err := proof.NewDevice()
	if err != nil {
		t.Fatalf("create test device: %v", err)
	}
	directory := t.TempDir()
	t.Setenv("UNUI_CONFIG_PATH", filepath.Join(directory, "config.json"))
	t.Setenv("UNUI_CREDENTIALS_PATH", filepath.Join(directory, "credentials.json"))
	if _, err := cliconfig.DefaultStore().SetRegistry(server.URL); err != nil {
		t.Fatalf("set registry: %v", err)
	}
	if err := store.Save(store.Credentials{
		DeviceID:   device.DeviceID,
		DeviceName: "Design Mac",
		Platform:   "darwin/arm64",
		PrivateKey: device.PrivateKey,
		PublicKey:  device.PublicKey,
		Registries: map[string]store.RegistryCredentials{
			server.URL: {PersonalToken: "personal-token"},
		},
	}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := execute(
		[]string{"auth", "logout", "--yes", "--json"},
		&stdout,
		&stderr,
		testBuildInfo,
	)
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d\n%s", exitCode, stderr.String())
	}
	if _, err := store.Load(); !errors.Is(err, store.ErrNotLoggedIn) {
		t.Fatalf("local credentials must be deleted, received %v", err)
	}

	var envelope struct {
		Data struct {
			LocalCredentialDeleted bool              `json:"localCredentialDeleted"`
			RemoteRevocationIssue  map[string]string `json:"remoteRevocationIssue"`
			RemoteRevoked          bool              `json:"remoteRevoked"`
		} `json:"data"`
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode logout output: %v\n%s", err, stdout.String())
	}
	if !envelope.OK ||
		!envelope.Data.LocalCredentialDeleted ||
		envelope.Data.RemoteRevoked ||
		envelope.Data.RemoteRevocationIssue["code"] != "CLI_UPDATE_REQUIRED" {
		t.Fatalf("unexpected logout result: %#v", envelope)
	}
}

func TestLogoutRemovesOnlyCurrentRegistryCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodDelete ||
				request.URL.Path != "/v1/cli/auth/personal-token" {
				http.NotFound(writer, request)
				return
			}
			writer.WriteHeader(http.StatusNoContent)
		},
	))
	defer server.Close()
	device, err := proof.NewDevice()
	if err != nil {
		t.Fatalf("create test device: %v", err)
	}
	directory := t.TempDir()
	t.Setenv("UNUI_CONFIG_PATH", filepath.Join(directory, "config.json"))
	t.Setenv("UNUI_CREDENTIALS_PATH", filepath.Join(directory, "credentials.json"))
	if _, err := cliconfig.DefaultStore().SetRegistry(server.URL); err != nil {
		t.Fatalf("set registry: %v", err)
	}
	if err := store.Save(store.Credentials{
		DeviceID:   device.DeviceID,
		DeviceName: "Design Mac",
		Platform:   "darwin/arm64",
		PrivateKey: device.PrivateKey,
		PublicKey:  device.PublicKey,
		Registries: map[string]store.RegistryCredentials{
			server.URL:                {PersonalToken: "current-token"},
			cliconfig.DefaultRegistry: {PersonalToken: "production-token"},
		},
	}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := Execute(
		[]string{"auth", "logout", "--yes", "--json"},
		&stdout,
		&stderr,
	); exitCode != 0 {
		t.Fatalf("unexpected exit code: %d\n%s", exitCode, stdout.String())
	}
	if strings.Contains(stdout.String(), server.URL) || strings.Contains(stderr.String(), server.URL) {
		t.Fatalf("logout output exposed registry: stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	saved, err := store.Load()
	if err != nil {
		t.Fatalf("load credentials: %v", err)
	}
	current, err := saved.ForRegistry(server.URL)
	if err != nil {
		t.Fatalf("select current credentials: %v", err)
	}
	production, err := saved.ForRegistry(cliconfig.DefaultRegistry)
	if err != nil {
		t.Fatalf("select production credentials: %v", err)
	}
	if !current.Empty() || production.PersonalToken != "production-token" {
		t.Fatalf("unexpected credentials after logout: %#v", saved)
	}
}

func authShowTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodGet ||
				request.URL.Path != "/v1/cli/status" {
				http.NotFound(writer, request)
				return
			}
			if request.Header.Get("Authorization") != "Bearer access-token" {
				http.Error(writer, "unauthorized", http.StatusUnauthorized)
				return
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{
				"account": {
					"email": "designer@example.com",
					"id": "user-123"
				},
				"device": {
					"deviceId": "device-123",
					"deviceName": "Design Mac",
					"platform": "darwin/arm64"
				},
				"membership": {
					"accessLevel": "PRO",
					"canUseCli": true,
					"expiresAt": "2026-07-20T02:00:00.000Z",
					"isAdmin": false,
					"isMember": true,
					"level": "PRO",
					"membershipLevel": "PRO",
					"remainingSeconds": 3600,
					"requiredCliLevel": "PRO"
				},
				"personalTokenExpiresAt": "2026-08-15T02:00:00.000Z"
			}`))
		},
	))
}

func prepareAuthShowTest(t *testing.T, registry string) {
	t.Helper()
	directory := t.TempDir()
	t.Setenv("UNUI_CONFIG_PATH", filepath.Join(directory, "config.json"))
	t.Setenv("UNUI_CREDENTIALS_PATH", filepath.Join(directory, "credentials.json"))
	if _, err := cliconfig.DefaultStore().SetRegistry(registry); err != nil {
		t.Fatalf("set registry: %v", err)
	}
	if err := store.Save(store.Credentials{
		DeviceID:   "device-123",
		DeviceName: "Design Mac",
		Platform:   "darwin/arm64",
		Registries: map[string]store.RegistryCredentials{
			registry: {
				AccessToken:          "access-token",
				AccessTokenExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339Nano),
			},
		},
	}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}
}
