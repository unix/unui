package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	cliconfig "github.com/unix/unui/internal/config"
	"github.com/unix/unui/internal/output"
	"github.com/unix/unui/internal/proof"
	"github.com/unix/unui/internal/store"
)

func TestAskRefreshesRejectedAccessTokenAndReturnsOneJSONEnvelope(t *testing.T) {
	var askRequests int
	var refreshRequests int
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/v1/cli/ask":
				askRequests++
				if request.Header.Get("Authorization") != "Bearer renewed-access-token" {
					http.Error(writer, "unauthorized", http.StatusUnauthorized)
					return
				}
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(`{"references":[],"rules":[]}`))
			case "/v1/cli/auth/access-tokens":
				refreshRequests++
				if request.Header.Get("Authorization") != "Personal personal-token" {
					http.Error(writer, "unauthorized", http.StatusUnauthorized)
					return
				}
				writeAccessTokenResponse(writer, "renewed-access-token")
			default:
				http.NotFound(writer, request)
			}
		},
	))
	defer server.Close()
	prepareAccessTest(
		t,
		server.URL,
		"stale-access-token",
		time.Now().Add(time.Hour),
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(
		[]string{"ask", "Design a billing page", "--json"},
		&stdout,
		&stderr,
	)
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d\n%s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty: %q", stderr.String())
	}
	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, stdout.String())
	}
	if !envelope.OK {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	if askRequests != 2 {
		t.Fatalf("expected the original request and one retry, got %d", askRequests)
	}
	if refreshRequests != 1 {
		t.Fatalf("expected one access refresh, got %d", refreshRequests)
	}
	credentials, err := store.Load()
	if err != nil {
		t.Fatalf("load refreshed credentials: %v", err)
	}
	registryCredentials, err := credentials.ForRegistry(server.URL)
	if err != nil {
		t.Fatalf("select refreshed credentials: %v", err)
	}
	if registryCredentials.AccessToken != "renewed-access-token" {
		t.Fatalf("unexpected saved access token: %q", registryCredentials.AccessToken)
	}
}

func TestAskReturnsAuthRequiredWhenAccessRefreshIsRejected(t *testing.T) {
	var askRequests int
	var refreshRequests int
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/v1/cli/ask":
				askRequests++
				http.Error(writer, "unauthorized", http.StatusUnauthorized)
			case "/v1/cli/auth/access-tokens":
				refreshRequests++
				http.Error(writer, "invalid personal token", http.StatusUnauthorized)
			default:
				http.NotFound(writer, request)
			}
		},
	))
	defer server.Close()
	prepareAccessTest(
		t,
		server.URL,
		"stale-access-token",
		time.Now().Add(time.Hour),
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute(
		[]string{"ask", "Design a billing page", "--json"},
		&stdout,
		&stderr,
	)
	if exitCode == 0 {
		t.Fatal("expected rejected personal credentials to fail")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty in JSON mode: %q", stderr.String())
	}
	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, stdout.String())
	}
	if envelope.Error == nil || envelope.Error.Code != "AUTH_REQUIRED" {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	if askRequests != 1 {
		t.Fatalf("the original request must not retry after refresh fails: %d", askRequests)
	}
	if refreshRequests != 1 {
		t.Fatalf("expected one access refresh, got %d", refreshRequests)
	}
}

func TestConcurrentAskSharesOneRefreshAfterUnauthorized(t *testing.T) {
	const commandCount = 2
	var mutex sync.Mutex
	var askRequests int
	currentAccessToken := "server-access-token"
	var refreshRequests int
	var staleRequests int
	staleBarrier := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/v1/cli/auth/access-tokens":
				mutex.Lock()
				refreshRequests++
				currentAccessToken = fmt.Sprintf("renewed-access-token-%d", refreshRequests)
				accessToken := currentAccessToken
				mutex.Unlock()
				writeAccessTokenResponse(writer, accessToken)
			case "/v1/cli/ask":
				mutex.Lock()
				askRequests++
				if request.Header.Get("Authorization") == "Bearer stale-access-token" {
					staleRequests++
					if staleRequests == commandCount {
						close(staleBarrier)
					}
					mutex.Unlock()
					<-staleBarrier
					http.Error(writer, "unauthorized", http.StatusUnauthorized)
					return
				}
				expected := "Bearer " + currentAccessToken
				mutex.Unlock()
				if request.Header.Get("Authorization") != expected {
					http.Error(writer, "unauthorized", http.StatusUnauthorized)
					return
				}
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(`{"references":[],"rules":[]}`))
			default:
				http.NotFound(writer, request)
			}
		},
	))
	defer server.Close()
	prepareAccessTest(
		t,
		server.URL,
		"stale-access-token",
		time.Now().Add(time.Hour),
	)

	start := make(chan struct{})
	var waitGroup sync.WaitGroup
	waitGroup.Add(commandCount)
	exitCodes := make([]int, commandCount)
	stderr := make([]bytes.Buffer, commandCount)
	stdout := make([]bytes.Buffer, commandCount)
	for index := range commandCount {
		go func() {
			defer waitGroup.Done()
			<-start
			exitCodes[index] = Execute(
				[]string{"ask", "Design a billing page", "--json"},
				&stdout[index],
				&stderr[index],
			)
		}()
	}
	close(start)
	waitGroup.Wait()

	for index, exitCode := range exitCodes {
		if exitCode != 0 {
			t.Fatalf(
				"command %d failed with exit code %d\nstdout: %s\nstderr: %s",
				index,
				exitCode,
				stdout[index].String(),
				stderr[index].String(),
			)
		}
		var envelope output.Envelope
		if err := json.Unmarshal(stdout[index].Bytes(), &envelope); err != nil {
			t.Fatalf("command %d did not return one JSON document: %v", index, err)
		}
		if !envelope.OK {
			t.Fatalf("command %d returned an error: %#v", index, envelope)
		}
	}
	mutex.Lock()
	defer mutex.Unlock()
	if refreshRequests != 1 {
		t.Fatalf("concurrent commands must share one refresh, got %d", refreshRequests)
	}
	if askRequests != commandCount*2 {
		t.Fatalf("unexpected ask request count: %d", askRequests)
	}
}

func TestValidAccessTokenDoesNotAcquireCredentialLock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path != "/v1/cli/ask" {
				http.NotFound(writer, request)
				return
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"references":[],"rules":[]}`))
		},
	))
	defer server.Close()
	prepareAccessTest(t, server.URL, "access-token", time.Now().Add(time.Hour))
	if err := os.Mkdir(os.Getenv("UNUI_CREDENTIALS_PATH")+".lock", 0o700); err != nil {
		t.Fatalf("block credential lock path: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := Execute(
		[]string{"ask", "Design a billing page", "--json"},
		&stdout,
		&stderr,
	); exitCode != 0 {
		t.Fatalf("unexpected exit code: %d\n%s", exitCode, stdout.String())
	}
}

func TestNetworkErrorDoesNotExposeRegistry(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	registry := server.URL
	server.Close()
	prepareAccessTest(t, registry, "access-token", time.Now().Add(time.Hour))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := Execute(
		[]string{"ask", "Design a billing page", "--json"},
		&stdout,
		&stderr,
	); exitCode == 0 {
		t.Fatal("expected network request to fail")
	}
	if strings.Contains(stdout.String(), registry) || strings.Contains(stderr.String(), registry) {
		t.Fatalf("command output exposed registry: stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
}

func TestAskMapsEvidenceAPIErrors(t *testing.T) {
	tests := []struct {
		name         string
		apiCode      string
		status       int
		expectedCode string
	}{
		{
			name:         "server timeout",
			apiCode:      "ASK_TIMEOUT",
			status:       http.StatusGatewayTimeout,
			expectedCode: "REQUEST_TIMEOUT",
		},
		{
			name:         "query too long",
			apiCode:      "QUERY_TOO_LONG",
			status:       http.StatusBadRequest,
			expectedCode: "QUERY_TOO_LONG",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(
				func(writer http.ResponseWriter, request *http.Request) {
					if request.URL.Path != "/v1/cli/ask" {
						http.NotFound(writer, request)
						return
					}
					writer.Header().Set("Content-Type", "application/json")
					writer.WriteHeader(test.status)
					_, _ = fmt.Fprintf(
						writer,
						`{"code":%q,"message":"request failed"}`,
						test.apiCode,
					)
				},
			))
			defer server.Close()
			prepareAccessTest(
				t,
				server.URL,
				"access-token",
				time.Now().Add(time.Hour),
			)

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			exitCode := Execute(
				[]string{"ask", "Design a billing page", "--json"},
				&stdout,
				&stderr,
			)
			if exitCode == 0 {
				t.Fatal("expected the API error to fail")
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr must be empty in JSON mode: %q", stderr.String())
			}
			var envelope output.Envelope
			if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
				t.Fatalf("stdout is not one JSON document: %v", err)
			}
			if envelope.Error == nil || envelope.Error.Code != test.expectedCode {
				t.Fatalf("unexpected envelope: %#v", envelope)
			}
		})
	}
}

func TestAccessRefreshUpdatesOnlyCurrentRegistry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/v1/cli/auth/access-tokens":
				writeAccessTokenResponse(writer, "renewed-access-token")
			case "/v1/cli/ask":
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(`{"references":[],"rules":[]}`))
			default:
				http.NotFound(writer, request)
			}
		},
	))
	defer server.Close()
	prepareAccessTest(t, server.URL, "expired-access-token", time.Now().Add(-time.Hour))
	credentials, err := store.Load()
	if err != nil {
		t.Fatalf("load credentials: %v", err)
	}
	production := store.RegistryCredentials{
		AccessToken:          "production-access-token",
		AccessTokenExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339Nano),
		PersonalToken:        "production-personal-token",
	}
	if err := credentials.SetRegistry(cliconfig.DefaultRegistry, production); err != nil {
		t.Fatalf("set production credentials: %v", err)
	}
	if err := store.Save(credentials); err != nil {
		t.Fatalf("save credentials: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := Execute(
		[]string{"ask", "Design a billing page", "--json"},
		&stdout,
		&stderr,
	); exitCode != 0 {
		t.Fatalf("unexpected exit code: %d\n%s", exitCode, stdout.String())
	}
	saved, err := store.Load()
	if err != nil {
		t.Fatalf("load refreshed credentials: %v", err)
	}
	current, err := saved.ForRegistry(server.URL)
	if err != nil {
		t.Fatalf("select current credentials: %v", err)
	}
	other, err := saved.ForRegistry(cliconfig.DefaultRegistry)
	if err != nil {
		t.Fatalf("select production credentials: %v", err)
	}
	if current.AccessToken != "renewed-access-token" || other != production {
		t.Fatalf("unexpected refreshed credentials: %#v", saved)
	}
}

func prepareAccessTest(
	t *testing.T,
	registry string,
	accessToken string,
	accessTokenExpiresAt time.Time,
) {
	t.Helper()
	device, err := proof.NewDevice()
	if err != nil {
		t.Fatalf("create test device: %v", err)
	}
	directory := t.TempDir()
	t.Setenv("UNUI_CONFIG_PATH", filepath.Join(directory, "config.json"))
	t.Setenv("UNUI_CREDENTIALS_PATH", filepath.Join(directory, "credentials.json"))
	if _, err := cliconfig.DefaultStore().SetRegistry(registry); err != nil {
		t.Fatalf("set registry: %v", err)
	}
	if err := store.Save(store.Credentials{
		DeviceID:   device.DeviceID,
		DeviceName: "Design Mac",
		Platform:   "darwin/arm64",
		PrivateKey: device.PrivateKey,
		PublicKey:  device.PublicKey,
		Registries: map[string]store.RegistryCredentials{
			registry: {
				AccessToken:          accessToken,
				AccessTokenExpiresAt: accessTokenExpiresAt.Format(time.RFC3339Nano),
				PersonalToken:        "personal-token",
				PersonalTokenExpires: time.Now().Add(24 * time.Hour).Format(time.RFC3339Nano),
			},
		},
	}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}
}

func writeAccessTokenResponse(writer http.ResponseWriter, accessToken string) {
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"accessToken":          accessToken,
		"accessTokenExpiresAt": time.Now().Add(time.Hour),
		"tokenType":            "Bearer",
	})
}
