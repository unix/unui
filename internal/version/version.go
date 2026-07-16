package version

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/mod/semver"
)

var semanticVersionPattern = regexp.MustCompile(
	`^[vV]?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)` +
		`(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?` +
		`(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`,
)

func Semantic(value string) (string, error) {
	version := strings.TrimSpace(value)
	if !semanticVersionPattern.MatchString(version) {
		return "", fmt.Errorf("%q is not a semantic version", value)
	}
	if strings.HasPrefix(version, "V") {
		version = "v" + strings.TrimPrefix(version, "V")
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	version = semver.Canonical(version)
	if version == "" {
		return "", fmt.Errorf("%q is not a semantic version", value)
	}
	return version, nil
}

func Compare(left, right string) (int, error) {
	leftVersion, err := Semantic(left)
	if err != nil {
		return 0, err
	}
	rightVersion, err := Semantic(right)
	if err != nil {
		return 0, err
	}
	return semver.Compare(leftVersion, rightVersion), nil
}

func Display(value string) string {
	return strings.TrimPrefix(value, "v")
}

func IsDevelopment(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "(devel)", "devel", "dev":
		return true
	default:
		return false
	}
}
