package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientSendsVersionAndAcceptsMinimumVersionHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			if request.Header.Get(CurrentVersionHeader) != "0.1.0" {
				t.Fatalf(
					"unexpected CLI version header: %q",
					request.Header.Get(CurrentVersionHeader),
				)
			}
			writer.Header().Set("X-UnUI-CLI-Min-Version", "v0.1.0")
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{}`))
		},
	))
	defer server.Close()

	client := Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		Version:    "0.1.0",
	}
	if _, err := client.ShowAuth(context.Background(), "access-token"); err != nil {
		t.Fatalf("show auth: %v", err)
	}
}

func TestClientUsesDefaultMinimumVersionWhenHeaderIsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{}`))
		},
	))
	defer server.Close()

	client := Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		Version:    DefaultMinimumVersion,
	}
	if _, err := client.ShowAuth(context.Background(), "access-token"); err != nil {
		t.Fatalf("show auth: %v", err)
	}
}

func TestClientRejectsMinimumVersionAboveCurrentVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set(MinimumVersionHeader, "0.2.0")
			writer.WriteHeader(http.StatusUpgradeRequired)
		},
	))
	defer server.Close()

	client := Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		Version:    "0.1.0",
	}
	_, err := client.ShowAuth(context.Background(), "access-token")
	var versionErr *MinimumVersionError
	if !errors.As(err, &versionErr) {
		t.Fatalf("expected minimum version error, received %v", err)
	}
	if versionErr.CurrentVersion != "0.1.0" ||
		versionErr.MinimumVersion != "0.2.0" {
		t.Fatalf("unexpected version error: %#v", versionErr)
	}
}

func TestClientRejectsInvalidMinimumVersionContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set(MinimumVersionHeader, "latest")
			writer.WriteHeader(http.StatusOK)
		},
	))
	defer server.Close()

	client := Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		Version:    "0.1.0",
	}
	_, err := client.ShowAuth(context.Background(), "access-token")
	var contractErr *VersionContractError
	if !errors.As(err, &contractErr) {
		t.Fatalf("expected version contract error, received %v", err)
	}
}

func TestClientSendsAndAllowsDevelopmentVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			if request.Header.Get(CurrentVersionHeader) != "dev" {
				t.Fatalf(
					"unexpected CLI version header: %q",
					request.Header.Get(CurrentVersionHeader),
				)
			}
			writer.Header().Set(MinimumVersionHeader, "99.0.0")
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{}`))
		},
	))
	defer server.Close()

	client := Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		Version:    "dev",
	}
	if _, err := client.ShowAuth(context.Background(), "access-token"); err != nil {
		t.Fatalf("show auth: %v", err)
	}
}
