package command

import (
	"errors"
	"fmt"
	"strings"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"
	"github.com/unix/unui-cli/internal/home"
	"github.com/unix/unui-cli/internal/installation"
	"github.com/unix/unui-cli/internal/message"
)

var preservedAfterUninstall = []string{
	"credentials",
	"configuration",
	"agentSkills",
}

type uninstallResult struct {
	Installation         installation.Info `json:"installation"`
	PreservedUserData    []string          `json:"preservedUserData"`
	Removed              bool              `json:"removed"`
	RequiresConfirmation bool              `json:"requiresConfirmation"`
}

func (a *app) uninstallCommand() *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the CLI or show its package-manager uninstall command",
		Args:  noArgs,
		Example: `  unui uninstall
  unui uninstall --yes
  unui uninstall --json`,
		RunE: func(_ *cobra.Command, _ []string) error {
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
			case installation.SourceInstallScript:
				return a.uninstallInstallScript(info, yes)
			case installation.SourceInstallPowerShell:
				return a.uninstallInstallPowerShell(info)
			case installation.SourceNPM:
				return a.uninstallNPM(info)
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
	command.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation")
	command.Flags().SortFlags = false
	return command
}

func (a *app) uninstallInstallScript(
	info installation.Info,
	yes bool,
) error {
	result := uninstallResult{
		Installation:         info,
		PreservedUserData:    preservedAfterUninstall,
		RequiresConfirmation: !yes,
	}
	if a.json && !yes {
		return a.printer().Success(result, "")
	}
	if !yes {
		confirmed := false
		description := fmt.Sprintf(
			"%s\nCredentials, configuration, and agent skills will be preserved.",
			home.DisplayPath(info.ExecutablePath),
		)
		if err := huh.NewConfirm().
			Title("Uninstall the unUI CLI?").
			Description(description).
			Affirmative("Uninstall").
			Negative("Cancel").
			Value(&confirmed).
			Run(); err != nil {
			return internalError("PROMPT_FAILED", err)
		}
		if !confirmed {
			result.RequiresConfirmation = false
			return a.printer().Success(
				result,
				a.printer().Info("Canceled", "No files were removed"),
			)
		}
	}

	if err := a.removeInstallation(info); err != nil {
		return newCommandError(
			"UNINSTALL_FAILED",
			message.UninstallFailed(),
			map[string]any{
				"error":        err.Error(),
				"installation": info,
			},
		)
	}
	result.Removed = true
	result.RequiresConfirmation = false
	human := strings.Join(
		[]string{
			a.printer().Done(
				"Uninstalled",
				home.DisplayPath(info.ExecutablePath),
			),
			a.printer().Info(
				"User data",
				"Credentials, configuration, and agent skills were preserved",
			),
		},
		"\n",
	)
	return a.printer().Success(result, human)
}

func (a *app) uninstallInstallPowerShell(info installation.Info) error {
	result := uninstallResult{
		Installation:      info,
		PreservedUserData: preservedAfterUninstall,
	}
	command := strings.Join([]string{
		"Remove-Item",
		"-LiteralPath",
		quotePowerShellLiteral(info.InstallDirectory),
		"-Recurse",
		"-Force",
	}, " ")
	human := strings.Join(
		[]string{
			a.printer().Info(
				"Installation",
				"install.ps1",
			),
			a.printer().Info(
				"Directory",
				home.DisplayPath(info.InstallDirectory),
			),
			a.printer().Info(
				"After this command exits, run",
				command,
			),
			a.printer().Info(
				"User PATH",
				"The fixed installation directory will remain in PATH for future reinstalls",
			),
			a.printer().Info(
				"User data",
				"Credentials, configuration, and agent skills will be preserved",
			),
		},
		"\n",
	)
	return a.printer().Success(result, human)
}

func (a *app) uninstallNPM(info installation.Info) error {
	result := uninstallResult{
		Installation:      info,
		PreservedUserData: preservedAfterUninstall,
	}
	if info.Temporary || !info.Global {
		label := "Local npm package"
		if info.Temporary {
			label = "Temporary npm run"
		}
		human := strings.Join(
			[]string{
				a.printer().Warning(
					label + " detected; no global CLI installation will be removed.",
				),
				a.printer().Info(
					"Executable",
					home.DisplayPath(info.ExecutablePath),
				),
			},
			"\n",
		)
		return a.printer().Success(result, human)
	}
	if len(info.Command) == 0 {
		return newCommandError(
			"PACKAGE_MANAGER_NOT_DETECTED",
			message.PackageManagerNotDetected(),
			map[string]any{"installation": info},
		)
	}

	human := strings.Join(
		[]string{
			a.printer().Info(
				"Installation",
				"npm package via "+info.Manager,
			),
			a.printer().Info(
				"Executable",
				home.DisplayPath(info.ExecutablePath),
			),
			a.printer().Info(
				"Uninstall",
				strings.Join(info.Command, " "),
			),
			a.printer().Info(
				"User data",
				"Credentials, configuration, and agent skills will be preserved",
			),
		},
		"\n",
	)
	return a.printer().Success(result, human)
}

func (a *app) detectInstallation() (installation.Info, error) {
	if a.detectInstall == nil {
		return installation.Detect()
	}
	return a.detectInstall()
}

func (a *app) removeInstallation(info installation.Info) error {
	if a.removeInstall == nil {
		return installation.Remove(info)
	}
	return a.removeInstall(info)
}

func quotePowerShellLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
