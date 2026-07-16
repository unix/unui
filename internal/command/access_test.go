package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
	if credentials.AccessToken != "renewed-access-token" {
		t.Fatalf("unexpected saved access token: %q", credentials.AccessToken)
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
		AccessToken:          accessToken,
		AccessTokenExpiresAt: accessTokenExpiresAt.Format(time.RFC3339Nano),
		DeviceID:             device.DeviceID,
		DeviceName:           "Design Mac",
		PersonalToken:        "personal-token",
		PersonalTokenExpires: time.Now().Add(24 * time.Hour).Format(time.RFC3339Nano),
		Platform:             "darwin/arm64",
		PrivateKey:           device.PrivateKey,
		PublicKey:            device.PublicKey,
		Registry:             registry,
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
