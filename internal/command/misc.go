package command

import (
	"bytes"
	"errors"
	"io"

	"github.com/spf13/cobra"
	"github.com/unix/unui-cli/internal/api"
	"github.com/unix/unui-cli/internal/store"
)

func (a *app) doctorCommand() *cobra.Command {
	return registryCommand(&cobra.Command{
		Use:   "doctor",
		Short: "Check local credentials and API access",
		Args:  noArgs,
		Example: `  unui doctor
  unui doctor --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			credentials, err := store.Load()
			if errors.Is(err, store.ErrNotLoggedIn) {
				return a.printer().Success(
					map[string]any{
						"apiUrl":   a.apiURL(),
						"loggedIn": false,
						"registry": a.registry,
					},
					a.printer().Warning(
						"No saved credentials were found for this device.",
					),
				)
			}
			if err != nil {
				return credentialStoreError(err)
			}
			ctx, cancel := a.context(cmd.Context())
			defer cancel()
			credentialRegistry := credentialsRegistry(credentials)
			var accessErr error
			if credentialRegistry != a.registry {
				accessErr = registryAuthenticationError(a.registry)
			} else {
				credentials, accessErr = a.credentialsWithAccess(ctx)
			}
			if accessErr == nil {
				_, credentials, accessErr = accessRequest(
					a,
					ctx,
					credentials,
					func(accessToken string) (api.AuthShowResponse, error) {
						return a.client().ShowAuth(ctx, accessToken)
					},
				)
			}
			if isVersionGateCommandError(accessErr) {
				return accessErr
			}
			human := a.printer().Done("Doctor", "All checks passed")
			var accessIssue map[string]string
			if accessErr != nil {
				accessIssue = errorSummary(accessErr)
				human = a.printer().Warning(
					"Doctor completed, but API access is not ready.",
				)
			}
			return a.printer().Success(
				map[string]any{
					"accessIssue":        accessIssue,
					"accessReady":        accessErr == nil,
					"apiUrl":             a.apiURL(),
					"credentialRegistry": credentialRegistry,
					"deviceId":           credentials.DeviceID,
					"loggedIn": credentials.PersonalToken != "" &&
						credentialRegistry == a.registry,
					"registry": a.registry,
				},
				human,
			)
		},
	})
}

func (a *app) completionCommand(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:    "completion <shell>",
		Short:  "Generate shell completion",
		Long:   "Generate a completion script for bash, zsh, fish, or PowerShell.",
		Hidden: true,
		Args: oneOfArg(
			"shell",
			"bash",
			"zsh",
			"fish",
			"powershell",
		),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		Example: `  unui completion zsh > "${fpath[1]}/_unui"
  unui completion bash > /usr/local/etc/bash_completion.d/unui`,
		RunE: func(_ *cobra.Command, args []string) error {
			var buffer bytes.Buffer
			var err error
			switch args[0] {
			case "bash":
				err = root.GenBashCompletion(&buffer)
			case "zsh":
				err = root.GenZshCompletion(&buffer)
			case "fish":
				err = root.GenFishCompletion(&buffer, true)
			case "powershell":
				err = root.GenPowerShellCompletion(&buffer)
			}
			if err != nil {
				return internalError("COMPLETION_FAILED", err)
			}
			if a.json {
				return a.printer().Success(
					map[string]any{
						"script": buffer.String(),
						"shell":  args[0],
					},
					"",
				)
			}
			_, err = io.Copy(a.stdout, &buffer)
			if err != nil {
				return internalError("OUTPUT_FAILED", err)
			}
			return nil
		},
	}
}

func (a *app) versionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print CLI version",
		Args:  noArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return a.printer().Success(
				a.buildInfo,
				a.printer().Version("unUI", a.buildInfo),
			)
		},
	}
}
