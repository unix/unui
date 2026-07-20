package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/unix/unui/internal/config"
	"github.com/unix/unui/internal/home"
)

const credentialsPathEnvironment = "UNUI_CREDENTIALS_PATH"

var ErrNotLoggedIn = errors.New("not logged in")

type RegistryCredentials struct {
	AccessToken          string `json:"accessToken,omitempty"`
	AccessTokenExpiresAt string `json:"accessTokenExpiresAt,omitempty"`
	PersonalToken        string `json:"personalToken,omitempty"`
	PersonalTokenExpires string `json:"personalTokenExpiresAt,omitempty"`
}

type Credentials struct {
	DeviceID              string                         `json:"deviceId,omitempty"`
	DeviceName            string                         `json:"deviceName,omitempty"`
	Platform              string                         `json:"platform,omitempty"`
	PrivateKey            string                         `json:"privateKey,omitempty"`
	PublicKey             string                         `json:"publicKey,omitempty"`
	Registries            map[string]RegistryCredentials `json:"registries,omitempty"`
	LegacyAccessToken     string                         `json:"accessToken,omitempty"`
	LegacyAccessExpiresAt string                         `json:"accessTokenExpiresAt,omitempty"`
	LegacyAPIURL          string                         `json:"apiUrl,omitempty"`
	LegacyPersonalToken   string                         `json:"personalToken,omitempty"`
	LegacyPersonalExpires string                         `json:"personalTokenExpiresAt,omitempty"`
	LegacyRegistry        string                         `json:"registry,omitempty"`
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
	var credentials Credentials
	if err := json.Unmarshal(payload, &credentials); err != nil {
		return Credentials{}, err
	}
	credentials, err = normalizeCredentials(credentials)
	if err != nil {
		return Credentials{}, err
	}
	return credentials, nil
}

func (s Store) Save(credentials Credentials) error {
	credentials, err := normalizeCredentials(credentials)
	if err != nil {
		return err
	}
	if credentials.Empty() {
		return s.Delete()
	}
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
	return nil
}

func (c Credentials) Empty() bool {
	return c.DeviceID == "" &&
		c.DeviceName == "" &&
		c.Platform == "" &&
		c.PrivateKey == "" &&
		c.PublicKey == "" &&
		len(c.Registries) == 0
}

func (c Credentials) ForRegistry(registry string) (RegistryCredentials, error) {
	registry, err := config.NormalizeRegistry(registry)
	if err != nil {
		return RegistryCredentials{}, err
	}
	return c.Registries[registry], nil
}

func (c *Credentials) SetRegistry(
	registry string,
	credentials RegistryCredentials,
) error {
	registry, err := config.NormalizeRegistry(registry)
	if err != nil {
		return err
	}
	if credentials.Empty() {
		delete(c.Registries, registry)
		return nil
	}
	if c.Registries == nil {
		c.Registries = make(map[string]RegistryCredentials)
	}
	c.Registries[registry] = credentials
	return nil
}

func (c RegistryCredentials) Empty() bool {
	return c.AccessToken == "" &&
		c.AccessTokenExpiresAt == "" &&
		c.PersonalToken == "" &&
		c.PersonalTokenExpires == ""
}

func normalizeCredentials(credentials Credentials) (Credentials, error) {
	normalized := make(map[string]RegistryCredentials, len(credentials.Registries))
	for registry, registryCredentials := range credentials.Registries {
		normalizedRegistry, err := config.NormalizeRegistry(registry)
		if err != nil {
			return Credentials{}, fmt.Errorf("invalid credentials registry: %w", err)
		}
		if _, exists := normalized[normalizedRegistry]; exists {
			return Credentials{}, errors.New("duplicate normalized credentials registry")
		}
		if registryCredentials.Empty() {
			continue
		}
		normalized[normalizedRegistry] = registryCredentials
	}
	credentials.Registries = normalized
	legacy := RegistryCredentials{
		AccessToken:          credentials.LegacyAccessToken,
		AccessTokenExpiresAt: credentials.LegacyAccessExpiresAt,
		PersonalToken:        credentials.LegacyPersonalToken,
		PersonalTokenExpires: credentials.LegacyPersonalExpires,
	}
	if !legacy.Empty() {
		registry, err := legacyCredentialsRegistry(credentials)
		if err != nil {
			return Credentials{}, err
		}
		if _, exists := credentials.Registries[registry]; exists {
			return Credentials{}, errors.New("legacy credentials conflict with registry credentials")
		}
		credentials.Registries[registry] = legacy
	}
	credentials.LegacyAccessToken = ""
	credentials.LegacyAccessExpiresAt = ""
	credentials.LegacyAPIURL = ""
	credentials.LegacyPersonalToken = ""
	credentials.LegacyPersonalExpires = ""
	credentials.LegacyRegistry = ""
	return credentials, nil
}

func legacyCredentialsRegistry(credentials Credentials) (string, error) {
	registryValue := credentials.LegacyRegistry
	if strings.TrimSpace(registryValue) == "" && strings.TrimSpace(credentials.LegacyAPIURL) != "" {
		apiURL := strings.TrimRight(credentials.LegacyAPIURL, "/")
		registryValue = strings.TrimSuffix(apiURL, "/v1/cli")
	}
	if strings.TrimSpace(registryValue) == "" {
		registryValue = config.DefaultRegistry
	}
	registry, err := config.NormalizeRegistry(registryValue)
	if err != nil {
		return "", fmt.Errorf("invalid legacy credentials registry: %w", err)
	}
	return registry, nil
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
