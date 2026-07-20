package command

import (
	"bytes"
	"errors"
	"io"

	"github.com/spf13/cobra"
	"github.com/unix/unui/internal/api"
	"github.com/unix/unui/internal/message"
	"github.com/unix/unui/internal/store"
)

func (a *app) doctorCommand() *cobra.Command {
	return registryCommand(&cobra.Command{
		Use:   "doctor",
		Short: "Check local credentials and API access",
		Args:  noArgs,
		Example: `  unui doctor
  unui doctor --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			saved, err := store.Load()
			if errors.Is(err, store.ErrNotLoggedIn) {
				return a.printer().Success(
					map[string]any{
						"accessReady": false,
						"loggedIn":    false,
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
			registryCredentials, accessErr := saved.ForRegistry(a.registry)
			if accessErr != nil {
				accessErr = credentialStoreError(accessErr)
			}
			credentials := scopedCredentials{
				Credentials:         saved,
				RegistryCredentials: registryCredentials,
			}
			if accessErr == nil && registryCredentials.Empty() {
				accessErr = newCommandError(
					"NOT_LOGGED_IN",
					message.NotLoggedIn(),
					nil,
				)
			}
			if accessErr == nil {
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
				accessIssue = a.errorSummary(accessErr)
				human = a.printer().Warning(
					"Doctor completed, but API access is not ready.",
				)
			}
			return a.printer().Success(
				map[string]any{
					"accessIssue": accessIssue,
					"accessReady": accessErr == nil,
					"deviceId":    credentials.DeviceID,
					"loggedIn":    credentials.PersonalToken != "",
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
