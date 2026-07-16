package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
	"github.com/unix/unui/internal/home"
	"github.com/unix/unui/internal/installation"
	"github.com/unix/unui/internal/message"
	cliversion "github.com/unix/unui/internal/version"
	"golang.org/x/mod/semver"
)

const (
	githubLatestReleaseURL = "https://api.github.com/repos/unix/unui/releases/latest"
	githubUserAgent        = "unui-cli"
)

type updateResult struct {
	CanUpdate       bool               `json:"canUpdate"`
	Command         string             `json:"command,omitempty"`
	CurrentVersion  string             `json:"currentVersion"`
	Installation    *installation.Info `json:"installation,omitempty"`
	LatestVersion   string             `json:"latestVersion"`
	ReleaseURL      string             `json:"releaseUrl,omitempty"`
	UpdateAvailable bool               `json:"updateAvailable"`
}

type releaseInfo struct {
	URL     string
	Version string
}

type githubReleaseResponse struct {
	HTMLURL string `json:"html_url"`
	TagName string `json:"tag_name"`
}

func (a *app) updateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Check GitHub for a newer CLI release",
		Args:  noArgs,
		Example: `  unui update
  unui update --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			currentVersion, err := cliversion.Semantic(a.buildInfo.Version)
			if err != nil {
				return newCommandError(
					"CURRENT_VERSION_UNSUPPORTED",
					message.CurrentVersionUnsupported(a.buildInfo.Version),
					map[string]any{
						"error":   err.Error(),
						"version": a.buildInfo.Version,
					},
				)
			}

			ctx, cancel := a.context(cmd.Context())
			defer cancel()
			release, err := a.latestRelease(ctx)
			if err != nil {
				return updateCheckCommandError(err)
			}

			latestVersion, err := cliversion.Semantic(release.Version)
			if err != nil {
				return newCommandError(
					"LATEST_RELEASE_INVALID",
					message.LatestReleaseInvalid(),
					map[string]any{
						"error":   err.Error(),
						"version": release.Version,
					},
				)
			}

			result := updateResult{
				CurrentVersion:  displayVersion(currentVersion),
				LatestVersion:   displayVersion(latestVersion),
				ReleaseURL:      release.URL,
				UpdateAvailable: semver.Compare(currentVersion, latestVersion) < 0,
			}
			if !result.UpdateAvailable {
				return a.printer().Success(
					result,
					a.printer().Done(
						"Already up to date",
						versionLabel(result.CurrentVersion),
					),
				)
			}

			info, err := a.detectInstallation()
			if errors.Is(err, installation.ErrInvalidReceipt) {
				return newCommandError(
					"INSTALLATION_RECEIPT_INVALID",
					message.InstallationReceiptInvalid(),
					map[string]any{"error": err.Error()},
				)
			}
			if err != nil {
				return newCommandError(
					"INSTALLATION_DETECTION_FAILED",
					message.InstallationNotDetected(),
					map[string]any{"error": err.Error()},
				)
			}
			switch info.Source {
			case installation.SourceInstallScript,
				installation.SourceInstallPowerShell:
				return a.showUpdateCommand(result, info)
			case installation.SourceNPM:
				return a.updateNPM(result, info)
			default:
				return newCommandError(
					"INSTALLATION_NOT_DETECTED",
					message.InstallationNotDetected(),
					map[string]any{
						"executablePath": info.ExecutablePath,
					},
				)
			}
		},
	}
}

func (a *app) showUpdateCommand(
	result updateResult,
	info installation.Info,
) error {
	command := installation.UpdateCommand(info)
	result.CanUpdate = true
	result.Command = command
	result.Installation = &info
	human := strings.Join(
		[]string{
			a.printer().Info(
				"Update available",
				versionTransition(result),
			),
			a.printer().Info("Installation", info.Manager),
			a.printer().Info(
				"Executable",
				home.DisplayPath(info.ExecutablePath),
			),
			a.printer().Info("After this command exits, run", command),
		},
		"\n",
	)
	return a.printer().Success(result, human)
}

func (a *app) updateNPM(
	result updateResult,
	info installation.Info,
) error {
	result.Installation = &info
	if info.Temporary || !info.Global {
		label := "Local npm package"
		description := "Update it from the project that declares @unix/unui."
		if info.Temporary {
			label = "Temporary npm run"
			description = "No persistent CLI installation needs to be updated."
		}
		human := strings.Join(
			[]string{
				a.printer().Info(
					"Update available",
					versionTransition(result),
				),
				a.printer().Warning(description),
				a.printer().Info(
					label,
					home.DisplayPath(info.ExecutablePath),
				),
			},
			"\n",
		)
		return a.printer().Success(result, human)
	}

	command := installation.UpdateCommand(info)
	if command == "" {
		return newCommandError(
			"PACKAGE_MANAGER_NOT_DETECTED",
			message.PackageManagerNotDetected(),
			map[string]any{"installation": info},
		)
	}
	result.CanUpdate = true
	result.Command = command
	human := strings.Join(
		[]string{
			a.printer().Info(
				"Update available",
				versionTransition(result),
			),
			a.printer().Info(
				"Installation",
				"npm package via "+info.Manager,
			),
			a.printer().Info(
				"Executable",
				home.DisplayPath(info.ExecutablePath),
			),
			a.printer().Info("After this command exits, run", command),
		},
		"\n",
	)
	return a.printer().Success(result, human)
}

func (a *app) latestRelease(ctx context.Context) (releaseInfo, error) {
	if a.fetchRelease != nil {
		return a.fetchRelease(ctx)
	}
	return fetchLatestGitHubRelease(
		ctx,
		http.DefaultClient,
		githubLatestReleaseURL,
	)
}

func fetchLatestGitHubRelease(
	ctx context.Context,
	client *http.Client,
	releaseURL string,
) (releaseInfo, error) {
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		releaseURL,
		nil,
	)
	if err != nil {
		return releaseInfo{}, fmt.Errorf("create GitHub release request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", githubUserAgent)
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	response, err := client.Do(request)
	if err != nil {
		return releaseInfo{}, fmt.Errorf("request latest GitHub release: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		return releaseInfo{}, fmt.Errorf(
			"latest GitHub release returned %s",
			response.Status,
		)
	}

	var value githubReleaseResponse
	if err := json.NewDecoder(
		io.LimitReader(response.Body, 1<<20),
	).Decode(&value); err != nil {
		return releaseInfo{}, fmt.Errorf("decode latest GitHub release: %w", err)
	}
	if strings.TrimSpace(value.TagName) == "" {
		return releaseInfo{}, errors.New("latest GitHub release has no tag name")
	}
	return releaseInfo{
		URL:     strings.TrimSpace(value.HTMLURL),
		Version: strings.TrimSpace(value.TagName),
	}, nil
}

func displayVersion(version string) string {
	return strings.TrimPrefix(version, "v")
}

func versionLabel(version string) string {
	return "v" + strings.TrimPrefix(version, "v")
}

func versionTransition(result updateResult) string {
	return versionLabel(result.CurrentVersion) +
		" → " +
		versionLabel(result.LatestVersion)
}

func updateCheckCommandError(err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return newCommandError(
			"UPDATE_CHECK_TIMEOUT",
			message.UpdateCheckTimedOut(),
			map[string]any{"error": err.Error()},
		)
	case errors.Is(err, context.Canceled):
		return newCommandError(
			"UPDATE_CHECK_CANCELED",
			message.UpdateCheckCanceled(),
			map[string]any{"error": err.Error()},
		)
	default:
		return newCommandError(
			"UPDATE_CHECK_FAILED",
			message.UpdateCheckFailed(),
			map[string]any{"error": err.Error()},
		)
	}
}
