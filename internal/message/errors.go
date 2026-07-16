package message

import (
	"fmt"
	"strings"
)

type ErrorText struct {
	Title   string
	Message string
	Hint    string
}

func NotLoggedIn() ErrorText {
	return ErrorText{
		Title:   "Authentication required",
		Message: "No saved CLI credentials were found.",
		Hint:    "Run `unui auth login` to continue.",
	}
}

func NothingToLogOut() ErrorText {
	return ErrorText{
		Title:   "Nothing to log out",
		Message: "This device does not have a saved CLI credential.",
		Hint:    "Run `unui auth login` to authorize this device.",
	}
}

func InvalidLimit(limit int) ErrorText {
	return ErrorText{
		Title: "Invalid limit",
		Message: fmt.Sprintf(
			"`--limit` must be between 1 and 12; received %d.",
			limit,
		),
		Hint: "Choose a value from 1 to 12 and run the command again.",
	}
}

func BrowserOpenFailed() ErrorText {
	return ErrorText{
		Title:   "Browser could not be opened",
		Message: "The authorization page could not be opened automatically.",
		Hint:    "Open the authorization URL manually to continue.",
	}
}

func LoginTimedOut() ErrorText {
	return ErrorText{
		Title:   "Authorization timed out",
		Message: "The authorization request was not approved within 5 minutes.",
		Hint:    "Run `unui auth login` again and approve the request in your browser.",
	}
}

func LoginNotApproved(status string) ErrorText {
	switch strings.ToLower(status) {
	case "denied":
		return ErrorText{
			Title:   "Authorization denied",
			Message: "The authorization request was denied in the browser.",
			Hint:    "Run `unui auth login` again when you are ready to approve this device.",
		}
	case "expired":
		return ErrorText{
			Title:   "Authorization expired",
			Message: "The authorization request expired before it was completed.",
			Hint:    "Run `unui auth login` again to create a new request.",
		}
	case "canceled", "cancelled":
		return ErrorText{
			Title:   "Authorization canceled",
			Message: "The authorization request was canceled before completion.",
			Hint:    "Run `unui auth login` again to restart authorization.",
		}
	default:
		return ErrorText{
			Title: "Authorization was not completed",
			Message: fmt.Sprintf(
				"The authorization request finished with status %q.",
				status,
			),
			Hint: "Run `unui auth login` again or use `--verbose` for diagnostics.",
		}
	}
}

func CredentialStoreUnavailable() ErrorText {
	return ErrorText{
		Title:   "Could not access saved credentials",
		Message: "unUI could not read or update credentials.json.",
		Hint:    "Check the file's JSON content and permissions, then run the command again.",
	}
}

func NetworkUnavailable() ErrorText {
	return ErrorText{
		Title:   "Could not reach unUI",
		Message: "The CLI could not connect to the unUI API.",
		Hint:    "Check your connection and configured registry, then try again.",
	}
}

func RequestTimedOut() ErrorText {
	return ErrorText{
		Title:   "Request timed out",
		Message: "The unUI API did not respond within 30 seconds.",
		Hint:    "Check your connection and try again.",
	}
}

func RequestCanceled() ErrorText {
	return ErrorText{
		Title:   "Request canceled",
		Message: "The command was canceled before the API request completed.",
		Hint:    "Run the command again when you are ready.",
	}
}

func AuthenticationExpired() ErrorText {
	return ErrorText{
		Title:   "Authentication expired",
		Message: "Your saved credentials are no longer accepted by the unUI API.",
		Hint:    "Run `unui auth login` to authorize this device again.",
	}
}

func AccessDenied() ErrorText {
	return ErrorText{
		Title:   "Access denied",
		Message: "Your account or membership does not allow this request.",
		Hint:    "Check your unUI account access, then try again.",
	}
}

func PlanRequired() ErrorText {
	return ErrorText{
		Title:   "Plan access required",
		Message: "Your current account plan cannot complete this request.",
		Hint:    "Check your unUI plan and usage status, then try again.",
	}
}

func ResourceNotFound() ErrorText {
	return ErrorText{
		Title:   "Resource not found",
		Message: "The requested unUI resource or API route could not be found.",
		Hint:    "Check the command parameters and configured registry, then try again.",
	}
}

func InvalidRegistry(registry string) ErrorText {
	return ErrorText{
		Title:   "Invalid registry",
		Message: fmt.Sprintf("`%s` is not a valid HTTP or HTTPS registry URL.", registry),
		Hint:    "Use a URL such as `https://api.unui.cc` or `http://127.0.0.1:3001`.",
	}
}

func ConfigUnavailable() ErrorText {
	return ErrorText{
		Title:   "CLI configuration is unavailable",
		Message: "unUI could not read or update the CLI configuration file.",
		Hint:    "Check the configuration file permissions, then run the command again.",
	}
}

func RegistryAuthenticationRequired(registry string) ErrorText {
	return ErrorText{
		Title: "Authentication required for this registry",
		Message: fmt.Sprintf(
			"No saved credentials belong to `%s`.",
			registry,
		),
		Hint: "Run `unui auth login` to authorize this registry.",
	}
}

func RequestConflict() ErrorText {
	return ErrorText{
		Title:   "Request could not be completed",
		Message: "The request conflicts with the current state of the resource.",
		Hint:    "Refresh the relevant status and try the command again.",
	}
}

func RequestRejected(reason string) ErrorText {
	message := "The unUI API rejected the request."
	if strings.TrimSpace(reason) != "" {
		message = strings.TrimSpace(reason)
	}
	return ErrorText{
		Title:   "Request rejected",
		Message: message,
		Hint:    "Review the command options or run again with `--verbose` for diagnostics.",
	}
}

func UsageThrottled() ErrorText {
	return ErrorText{
		Title:   "Usage temporarily limited",
		Message: "unUI has paused this request because the recent usage limit was reached.",
		Hint:    "Wait a moment, then try again.",
	}
}

func ServiceUnavailable() ErrorText {
	return ErrorText{
		Title:   "unUI is temporarily unavailable",
		Message: "The service could not complete the request right now.",
		Hint:    "Try again in a few moments.",
	}
}

func UnknownCommand(name, parent string) ErrorText {
	return ErrorText{
		Title:   "Unknown command",
		Message: fmt.Sprintf("`%s` is not a command under `%s`.", name, parent),
		Hint:    fmt.Sprintf("Run `%s --help` to see the available commands.", parent),
	}
}

func UnknownOption(name, command string) ErrorText {
	return ErrorText{
		Title:   "Unknown option",
		Message: fmt.Sprintf("`%s` is not a valid option for `%s`.", name, command),
		Hint:    fmt.Sprintf("Run `%s --help` to see the available options.", command),
	}
}

func MissingOptionValue(name, command string) ErrorText {
	return ErrorText{
		Title:   "Option value required",
		Message: fmt.Sprintf("`%s` requires a value.", name),
		Hint:    fmt.Sprintf("Run `%s --help` to see the expected value.", command),
	}
}

func MissingRequiredOption(name, command string) ErrorText {
	return ErrorText{
		Title:   "Configuration option required",
		Message: fmt.Sprintf("`%s` is required for `%s`.", name, command),
		Hint:    fmt.Sprintf("Run `%s --help` to see an example.", command),
	}
}

func InvalidOptionValue(name, command string) ErrorText {
	return ErrorText{
		Title:   "Invalid option value",
		Message: fmt.Sprintf("The value supplied for `%s` is not valid.", name),
		Hint:    fmt.Sprintf("Run `%s --help` to see the expected format.", command),
	}
}

func MissingArgument(name, command string) ErrorText {
	return ErrorText{
		Title:   "Missing argument",
		Message: fmt.Sprintf("`%s` requires the <%s> argument.", command, name),
		Hint:    fmt.Sprintf("Run `%s --help` to see an example.", command),
	}
}

func TooManyArguments(command string) ErrorText {
	return ErrorText{
		Title:   "Too many arguments",
		Message: fmt.Sprintf("`%s` received more arguments than it supports.", command),
		Hint:    fmt.Sprintf("Run `%s --help` to review the command usage.", command),
	}
}

func UnsupportedValue(name, value, command string) ErrorText {
	return ErrorText{
		Title:   fmt.Sprintf("Unsupported %s", name),
		Message: fmt.Sprintf("`%s` is not a supported %s.", value, name),
		Hint:    fmt.Sprintf("Run `%s --help` to see the supported values.", command),
	}
}

func SkillClientNotDetected() ErrorText {
	return ErrorText{
		Title:   "No supported coding agent was detected",
		Message: "unUI could not find a local Codex, Claude Code, or Cursor configuration.",
		Hint:    "Run `unui skill update --client <client>` with codex, claude, or cursor.",
	}
}

func SkillInstallFailed() ErrorText {
	return ErrorText{
		Title:   "Could not install the unUI skill",
		Message: "The bundled skill could not replace the selected client's user-level skill directory.",
		Hint:    "Check the target directory permissions and run the command again.",
	}
}

func InstallationNotDetected() ErrorText {
	return ErrorText{
		Title:   "Installation method not detected",
		Message: "unUI could not determine how this CLI executable was installed.",
		Hint:    "Reinstall with the official platform script or the global `@unix/unui` npm package, then try again.",
	}
}

func InstallationReceiptInvalid() ErrorText {
	return ErrorText{
		Title:   "Installation receipt is invalid",
		Message: "The script installation receipt does not match the current unUI executable.",
		Hint:    "Reinstall with the official platform script before updating or removing the CLI.",
	}
}

func PackageManagerNotDetected() ErrorText {
	return ErrorText{
		Title:   "Package manager not detected",
		Message: "The CLI came from the `@unix/unui` npm package, but its global package manager could not be identified.",
		Hint:    "Use the package manager that installed `@unix/unui` to update or remove the global package.",
	}
}

func CurrentVersionUnsupported(version string) ErrorText {
	return ErrorText{
		Title: "Current version cannot be compared",
		Message: fmt.Sprintf(
			"The running CLI reports version %q, which is not a semantic release version.",
			version,
		),
		Hint: "Install a released version of the unUI CLI, then run `unui update` again.",
	}
}

func CLIUpdateRequired(currentVersion, minimumVersion string) ErrorText {
	return ErrorText{
		Title: "CLI update required",
		Message: fmt.Sprintf(
			"The unUI API requires CLI version %s or newer; the current version is %s.",
			minimumVersion,
			currentVersion,
		),
		Hint: "Run `unui update`, upgrade the CLI, then run the command again.",
	}
}

func InvalidCLIVersion(version string) ErrorText {
	return ErrorText{
		Title: "CLI version is invalid",
		Message: fmt.Sprintf(
			"The running CLI reports version %q, which the unUI API cannot compare.",
			version,
		),
		Hint: "Install a released version of the unUI CLI, then run the command again.",
	}
}

func InvalidAPIVersionContract(version string) ErrorText {
	return ErrorText{
		Title: "API version requirement is invalid",
		Message: fmt.Sprintf(
			"The unUI API returned invalid minimum CLI version %q.",
			version,
		),
		Hint: "Try again later or contact unUI support if the problem continues.",
	}
}

func LatestReleaseInvalid() ErrorText {
	return ErrorText{
		Title:   "Latest release version is invalid",
		Message: "The latest unUI GitHub Release does not use a semantic version tag.",
		Hint:    "Check the unUI GitHub Releases page and try again after the release tag is corrected.",
	}
}

func UpdateCheckFailed() ErrorText {
	return ErrorText{
		Title:   "Could not check for updates",
		Message: "The CLI could not retrieve the latest unUI release from GitHub.",
		Hint:    "Check your connection and GitHub availability, then try again.",
	}
}

func UpdateCheckTimedOut() ErrorText {
	return ErrorText{
		Title:   "Update check timed out",
		Message: "GitHub did not return the latest unUI release within 30 seconds.",
		Hint:    "Check your connection and try again.",
	}
}

func UpdateCheckCanceled() ErrorText {
	return ErrorText{
		Title:   "Update check canceled",
		Message: "The command was canceled before GitHub returned the latest unUI release.",
		Hint:    "Run `unui update` again when you are ready.",
	}
}

func UninstallFailed() ErrorText {
	return ErrorText{
		Title:   "Could not uninstall the unUI CLI",
		Message: "The verified install.sh executable or its receipt could not be removed.",
		Hint:    "Check the installation directory permissions and run `unui uninstall --yes` again.",
	}
}

func Internal(code string) ErrorText {
	switch code {
	case "DEVICE_KEY_FAILED":
		return ErrorText{
			Title:   "Could not create device credentials",
			Message: "The CLI could not create the secure key used to identify this device.",
			Hint:    "Try again or run with `--verbose` for diagnostics.",
		}
	case "DEVICE_PROOF_FAILED":
		return ErrorText{
			Title:   "Could not verify this device",
			Message: "The CLI could not sign the proof required by the unUI API.",
			Hint:    "Run `unui auth login` again. Use `--verbose` if the problem continues.",
		}
	case "VERIFIER_FAILED":
		return ErrorText{
			Title:   "Could not start authorization",
			Message: "The CLI could not prepare a secure browser authorization request.",
			Hint:    "Try again or run with `--verbose` for diagnostics.",
		}
	case "PROMPT_FAILED":
		return ErrorText{
			Title:   "Confirmation prompt unavailable",
			Message: "The CLI could not read your confirmation.",
			Hint:    "Run the command again with `--yes` to skip the prompt.",
		}
	case "COMPLETION_FAILED":
		return ErrorText{
			Title:   "Could not generate completion",
			Message: "The shell completion script could not be generated.",
			Hint:    "Run the command again with `--verbose` for diagnostics.",
		}
	case "OUTPUT_FAILED", "HELP_WRITE_FAILED":
		return ErrorText{
			Title:   "Could not write command output",
			Message: "The CLI could not write to the selected output stream.",
			Hint:    "Check the pipe or destination and run the command again.",
		}
	case "SKILL_HOME_UNAVAILABLE":
		return ErrorText{
			Title:   "Could not locate the user home directory",
			Message: "The CLI could not determine where user-level coding agent skills should be installed.",
			Hint:    "Check the current user's home directory configuration and run the command again.",
		}
	case "SKILL_BUNDLE_UNAVAILABLE":
		return ErrorText{
			Title:   "Bundled skill is unavailable",
			Message: "The CLI could not read the unUI skill included in this release.",
			Hint:    "Reinstall or update the unUI CLI, then run the command again.",
		}
	case "SKILL_CLIENT_DETECTION_FAILED":
		return ErrorText{
			Title:   "Could not inspect local coding agents",
			Message: "The CLI could not check the local client configuration directories.",
			Hint:    "Run the command with an explicit `--client` value or use `--verbose` for diagnostics.",
		}
	default:
		return ErrorText{
			Title:   "Command failed",
			Message: "An unexpected error occurred while running the command.",
			Hint:    "Run the command again with `--verbose` for technical details.",
		}
	}
}
