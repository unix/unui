package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/unix/unui/internal/home"
)

const DefaultRegistry = "https://api.unui.cc"

const configPathEnvironment = "UNUI_CONFIG_PATH"

var ErrInvalidRegistry = errors.New("invalid registry")

type Values struct {
	Registry string `json:"registry,omitempty"`
}

type Store struct {
	FilePath string
}

func DefaultStore() Store {
	return Store{}
}

func (s Store) Load() (Values, error) {
	path, err := s.Path()
	if err != nil {
		return Values{}, err
	}
	payload, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Values{}, nil
	}
	if err != nil {
		return Values{}, err
	}

	var values Values
	if err := json.Unmarshal(payload, &values); err != nil {
		return Values{}, err
	}
	if values.Registry == "" {
		return values, nil
	}
	values.Registry, err = NormalizeRegistry(values.Registry)
	if err != nil {
		return Values{}, err
	}
	return values, nil
}

func (s Store) SetRegistry(value string) (string, error) {
	registry, err := NormalizeRegistry(value)
	if err != nil {
		return "", err
	}
	path, err := s.Path()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	payload, err := json.MarshalIndent(Values{Registry: registry}, "", "  ")
	if err != nil {
		return "", err
	}
	payload = append(payload, '\n')
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return "", err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return "", err
	}
	return registry, nil
}

func (s Store) ResetRegistry() error {
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
	if path := strings.TrimSpace(os.Getenv(configPathEnvironment)); path != "" {
		return path, nil
	}
	return home.Path("config.json")
}

func (v Values) EffectiveRegistry() string {
	if v.Registry == "" {
		return DefaultRegistry
	}
	return v.Registry
}

func NormalizeRegistry(value string) (string, error) {
	value = strings.TrimSpace(value)
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidRegistry, err)
	}
	if value == "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Hostname() == "" ||
		parsed.User != nil ||
		parsed.RawQuery != "" ||
		parsed.ForceQuery ||
		parsed.Fragment != "" {
		return "", ErrInvalidRegistry
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = strings.TrimRight(parsed.RawPath, "/")
	return parsed.String(), nil
}
