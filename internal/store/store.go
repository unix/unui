package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/unix/unui/internal/home"
)

const credentialsPathEnvironment = "UNUI_CREDENTIALS_PATH"

var ErrNotLoggedIn = errors.New("not logged in")

type Credentials struct {
	AccessToken          string `json:"accessToken,omitempty"`
	AccessTokenExpiresAt string `json:"accessTokenExpiresAt,omitempty"`
	APIURL               string `json:"apiUrl,omitempty"`
	DeviceID             string `json:"deviceId"`
	DeviceName           string `json:"deviceName"`
	Platform             string `json:"platform"`
	PrivateKey           string `json:"privateKey"`
	PublicKey            string `json:"publicKey"`
	PersonalToken        string `json:"personalToken,omitempty"`
	PersonalTokenExpires string `json:"personalTokenExpiresAt,omitempty"`
	Registry             string `json:"registry,omitempty"`
}

type Store struct {
	FilePath string
}

func DefaultStore() Store {
	return Store{}
}

func Load() (Credentials, error) {
	return DefaultStore().Load()
}

func Save(credentials Credentials) error {
	return DefaultStore().Save(credentials)
}

func Delete() error {
	return DefaultStore().Delete()
}

func (s Store) Load() (Credentials, error) {
	path, err := s.Path()
	if err != nil {
		return Credentials{}, err
	}
	payload, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Credentials{}, ErrNotLoggedIn
	}
	if err != nil {
		return Credentials{}, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return Credentials{}, err
	}

	var credentials Credentials
	if err := json.Unmarshal(payload, &credentials); err != nil {
		return Credentials{}, err
	}
	return credentials, nil
}

func (s Store) Save(credentials Credentials) error {
	path, err := s.Path()
	if err != nil {
		return err
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')

	temporary, err := os.CreateTemp(directory, ".credentials-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return err
	}
	if _, err := temporary.Write(payload); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func (s Store) Delete() error {
	path, err := s.Path()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s Store) Path() (string, error) {
	if strings.TrimSpace(s.FilePath) != "" {
		return s.FilePath, nil
	}
	if path := strings.TrimSpace(os.Getenv(credentialsPathEnvironment)); path != "" {
		return path, nil
	}
	return home.Path("credentials.json")
}
