package command

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/unix/unui/internal/api"
	cliconfig "github.com/unix/unui/internal/config"
	"github.com/unix/unui/internal/message"
)

var optionPattern = regexp.MustCompile(`-{1,2}[a-zA-Z0-9][a-zA-Z0-9-]*`)

type commandError struct {
	Code     string
	Details  any
	ExitCode int
	Hint     string
	Message  string
	Title    string
}

func (e *commandError) Error() string {
	return e.Message
}

func newCommandError(
	code string,
	text message.ErrorText,
	details any,
) *commandError {
	return &commandError{
		Code:     code,
		Details:  details,
		ExitCode: 1,
		Hint:     text.Hint,
		Message:  text.Message,
		Title:    text.Title,
	}
}

func (a *app) renderError(err error) int {
	var commandErr *commandError
	if errors.As(err, &commandErr) {
		a.printer().Failure(
			commandErr.ExitCode,
			commandErr.Code,
			commandErr.Title,
			commandErr.Message,
			commandErr.Hint,
			commandErr.Details,
		)
		return commandErr.ExitCode
	}
	text := message.Internal("COMMAND_FAILED")
	a.printer().Failure(
		1,
		"COMMAND_FAILED",
		text.Title,
		text.Message,
		text.Hint,
		map[string]any{"error": err.Error()},
	)
	return 1
}

func normalizeCommandError(err error) error {
	var commandErr *commandError
	if errors.As(err, &commandErr) {
		return err
	}

	errorMessage := err.Error()
	const unknownCommandPrefix = "unknown command "
	if strings.HasPrefix(errorMessage, unknownCommandPrefix) {
		values := quotedValues(errorMessage)
		if len(values) >= 2 {
			return newCommandError(
				"UNKNOWN_COMMAND",
				message.UnknownCommand(values[0], values[1]),
				map[string]any{
					"command": values[0],
					"parent":  values[1],
				},
			)
		}
	}
	return internalError("COMMAND_FAILED", err)
}

func flagCommandError(cmd *cobra.Command, err error) error {
	command := cmd.CommandPath()
	option := optionPattern.FindString(err.Error())
	details := map[string]any{"error": err.Error()}
	switch {
	case strings.Contains(err.Error(), "unknown flag"):
		return newCommandError(
			"UNKNOWN_OPTION",
			message.UnknownOption(option, command),
			details,
		)
	case strings.Contains(err.Error(), "needs an argument"):
		return newCommandError(
			"MISSING_OPTION_VALUE",
			message.MissingOptionValue(option, command),
			details,
		)
	default:
		return newCommandError(
			"INVALID_OPTION_VALUE",
			message.InvalidOptionValue(option, command),
			details,
		)
	}
}

func noArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	return newCommandError(
		"TOO_MANY_ARGUMENTS",
		message.TooManyArguments(cmd.CommandPath()),
		map[string]any{"arguments": args},
	)
}

func rootArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	return newCommandError(
		"UNKNOWN_COMMAND",
		message.UnknownCommand(args[0], cmd.CommandPath()),
		map[string]any{
			"arguments": args,
			"command":   args[0],
		},
	)
}

func exactArgs(names ...string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < len(names) {
			return newCommandError(
				"MISSING_ARGUMENT",
				message.MissingArgument(names[len(args)], cmd.CommandPath()),
				map[string]any{
					"arguments": args,
					"expected":  names,
				},
			)
		}
		if len(args) > len(names) {
			return newCommandError(
				"TOO_MANY_ARGUMENTS",
				message.TooManyArguments(cmd.CommandPath()),
				map[string]any{
					"arguments": args,
					"expected":  names,
				},
			)
		}
		return nil
	}
}

func oneOfArg(name string, values ...string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := exactArgs(name)(cmd, args); err != nil {
			return err
		}
		for _, value := range values {
			if args[0] == value {
				return nil
			}
		}
		return newCommandError(
			"UNSUPPORTED_"+strings.ToUpper(name),
			message.UnsupportedValue(name, args[0], cmd.CommandPath()),
			map[string]any{
				"supported": values,
				"value":     args[0],
			},
		)
	}
}

func apiCommandError(err error) error {
	var minimumErr *api.MinimumVersionError
	if errors.As(err, &minimumErr) {
		return newCommandError(
			"CLI_UPDATE_REQUIRED",
			message.CLIUpdateRequired(
				minimumErr.CurrentVersion,
				minimumErr.MinimumVersion,
			),
			map[string]any{
				"currentVersion": minimumErr.CurrentVersion,
				"minimumVersion": minimumErr.MinimumVersion,
			},
		)
	}
	var contractErr *api.VersionContractError
	if errors.As(err, &contractErr) {
		return newCommandError(
			"API_VERSION_CONTRACT_INVALID",
			message.InvalidAPIVersionContract(contractErr.MinimumVersion),
			map[string]any{
				"minimumVersion": contractErr.MinimumVersion,
			},
		)
	}
	var currentErr *api.CurrentVersionError
	if errors.As(err, &currentErr) {
		return newCommandError(
			"INVALID_CLI_VERSION",
			message.InvalidCLIVersion(currentErr.CurrentVersion),
			map[string]any{
				"currentVersion": currentErr.CurrentVersion,
			},
		)
	}
	var apiErr *api.Error
	if errors.As(err, &apiErr) {
		if apiErr.Code == "CLI_UPDATE_REQUIRED" {
			currentVersion := detailString(apiErr.Body, "currentVersion")
			minimumVersion := detailString(apiErr.Body, "minimumVersion")
			return newCommandError(
				"CLI_UPDATE_REQUIRED",
				message.CLIUpdateRequired(currentVersion, minimumVersion),
				apiErr.Body,
			)
		}
		if apiErr.Code == "INVALID_CLI_VERSION" {
			currentVersion := detailString(apiErr.Body, "currentVersion")
			return newCommandError(
				"INVALID_CLI_VERSION",
				message.InvalidCLIVersion(currentVersion),
				apiErr.Body,
			)
		}
		code := fmt.Sprintf("API_%d", apiErr.Status)
		text := message.RequestRejected(apiErr.Message)
		switch apiErr.Status {
		case http.StatusUnauthorized:
			code = "AUTH_REQUIRED"
			text = message.AuthenticationExpired()
		case http.StatusPaymentRequired:
			code = "PLAN_REQUIRED"
			text = message.PlanRequired()
		case http.StatusForbidden:
			code = "ACCESS_FORBIDDEN"
			text = message.AccessDenied()
		case http.StatusNotFound:
			code = "NOT_FOUND"
			text = message.ResourceNotFound()
		case http.StatusConflict:
			code = "REQUEST_CONFLICT"
			text = message.RequestConflict()
		case http.StatusTooManyRequests:
			code = "USAGE_THROTTLED"
			text = message.UsageThrottled()
		default:
			if apiErr.Status >= http.StatusInternalServerError {
				code = "SERVICE_UNAVAILABLE"
				text = message.ServiceUnavailable()
			}
		}
		return newCommandError(code, text, apiErr.Body)
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return newCommandError(
			"REQUEST_TIMEOUT",
			message.RequestTimedOut(),
			map[string]any{"error": err.Error()},
		)
	case errors.Is(err, context.Canceled):
		return newCommandError(
			"REQUEST_CANCELED",
			message.RequestCanceled(),
			map[string]any{"error": err.Error()},
		)
	default:
		return newCommandError(
			"NETWORK_ERROR",
			message.NetworkUnavailable(),
			map[string]any{"error": err.Error()},
		)
	}
}

func isVersionGateCommandError(err error) bool {
	var commandErr *commandError
	if !errors.As(err, &commandErr) {
		return false
	}
	switch commandErr.Code {
	case "API_VERSION_CONTRACT_INVALID",
		"CLI_UPDATE_REQUIRED",
		"INVALID_CLI_VERSION":
		return true
	default:
		return false
	}
}

func detailString(details any, key string) string {
	values, ok := details.(map[string]any)
	if !ok {
		return ""
	}
	value, _ := values[key].(string)
	return value
}

func credentialStoreError(err error) error {
	return newCommandError(
		"CREDENTIAL_STORE_UNAVAILABLE",
		message.CredentialStoreUnavailable(),
		map[string]any{"error": err.Error()},
	)
}

func registryCommandError(value string, err error) error {
	if errors.Is(err, cliconfig.ErrInvalidRegistry) {
		return newCommandError(
			"INVALID_REGISTRY",
			message.InvalidRegistry(value),
			map[string]any{
				"error":    err.Error(),
				"registry": value,
			},
		)
	}
	return configCommandError(err)
}

func configCommandError(err error) error {
	return newCommandError(
		"CONFIG_UNAVAILABLE",
		message.ConfigUnavailable(),
		map[string]any{"error": err.Error()},
	)
}

func registryAuthenticationError(registry string) error {
	return newCommandError(
		"REGISTRY_AUTH_REQUIRED",
		message.RegistryAuthenticationRequired(registry),
		map[string]any{"registry": registry},
	)
}

func missingRequiredOptionError(
	option string,
	cmd *cobra.Command,
) error {
	return newCommandError(
		"MISSING_OPTION",
		message.MissingRequiredOption(option, cmd.CommandPath()),
		map[string]any{"option": option},
	)
}

func internalError(code string, err error) error {
	return newCommandError(
		code,
		message.Internal(code),
		map[string]any{"error": err.Error()},
	)
}

func errorSummary(err error) map[string]string {
	var commandErr *commandError
	if errors.As(err, &commandErr) {
		return map[string]string{
			"code":    commandErr.Code,
			"hint":    commandErr.Hint,
			"message": commandErr.Message,
			"title":   commandErr.Title,
		}
	}
	text := message.Internal("COMMAND_FAILED")
	return map[string]string{
		"code":    "COMMAND_FAILED",
		"hint":    text.Hint,
		"message": text.Message,
		"title":   text.Title,
	}
}

func quotedValues(value string) []string {
	values := make([]string, 0, 2)
	for {
		start := strings.IndexByte(value, '"')
		if start < 0 {
			return values
		}
		value = value[start+1:]
		end := strings.IndexByte(value, '"')
		if end < 0 {
			return values
		}
		values = append(values, value[:end])
		value = value[end+1:]
	}
}
