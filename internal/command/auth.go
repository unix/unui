package command

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/unix/unui/internal/api"
	"github.com/unix/unui/internal/browser"
	"github.com/unix/unui/internal/message"
	"github.com/unix/unui/internal/proof"
	"github.com/unix/unui/internal/store"
)

func (a *app) authCommand() *cobra.Command {
	auth := &cobra.Command{
		Use:   "auth [command]",
		Short: "Manage device authentication",
		Long:  "Authorize this CLI device, inspect its account access, or revoke its saved credential.",
		Args:  rootArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if a.json {
				return a.printer().Help(cmd, a.buildInfo.Version)
			}
			return cmd.Help()
		},
	}
	auth.AddCommand(a.loginCommand(), a.showAuthCommand(), a.logoutCommand())
	return auth
}

func (a *app) loginCommand() *cobra.Command {
	command := &cobra.Command{
		Use:     "login",
		Short:   "Authorize this CLI device in the browser",
		Args:    noArgs,
		Example: `  unui auth login`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := a.authorizationContext(cmd.Context())
			defer cancel()
			credentials, err := loadOrCreateDeviceCredentials(ctx)
			if err != nil {
				return err
			}

			verifier, err := proof.RandomVerifier()
			if err != nil {
				return internalError("VERIFIER_FAILED", err)
			}
			request, err := a.client().CreateAuthorization(
				ctx,
				api.CreateAuthorizationRequest{
					DeviceID:          credentials.DeviceID,
					DeviceName:        credentials.DeviceName,
					Platform:          credentials.Platform,
					PublicKey:         credentials.PublicKey,
					VerifierChallenge: proof.Challenge(verifier),
				},
			)
			if err != nil {
				return apiCommandError(err)
			}
			if !a.json {
				_, _ = fmt.Fprintln(
					a.stdout,
					a.printer().Info("Authorize", request.AuthorizeURL),
				)
			}
			if err := browser.Open(request.AuthorizeURL); err != nil {
				if a.json {
					return newCommandError(
						"BROWSER_OPEN_FAILED",
						message.BrowserOpenFailed(),
						map[string]any{
							"authorizeUrl": request.AuthorizeURL,
							"error":        err.Error(),
						},
					)
				}
				_, _ = fmt.Fprintln(
					a.stdout,
					a.printer().Warning(
						"Browser did not open automatically; open the URL above manually.",
					),
				)
			}

			interval := time.Duration(request.IntervalSeconds) * time.Second
			if interval <= 0 {
				interval = 2 * time.Second
			}
			var status api.PollAuthorizationResponse
			for {
				status, err = a.client().PollAuthorization(
					ctx,
					request.RequestID,
					request.PollSecret,
				)
				if err != nil {
					return apiCommandError(err)
				}
				if status.Status != "pending" {
					break
				}
				select {
				case <-ctx.Done():
					return newCommandError(
						"LOGIN_TIMEOUT",
						message.LoginTimedOut(),
						map[string]any{
							"authorizeUrl": request.AuthorizeURL,
						},
					)
				case <-time.After(interval):
				}
			}
			if status.Status != "approved" {
				return newCommandError(
					"LOGIN_"+strings.ToUpper(status.Status),
					message.LoginNotApproved(status.Status),
					map[string]any{"status": status.Status},
				)
			}

			deviceProof, err := proof.Sign(
				credentials.PrivateKey,
				"login",
				request.RequestID,
				time.Now(),
			)
			if err != nil {
				return internalError("DEVICE_PROOF_FAILED", err)
			}
			exchanged, err := a.client().ExchangeAuthorization(
				ctx,
				request.RequestID,
				api.ExchangeAuthorizationRequest{
					PollSecret: request.PollSecret,
					Proof:      deviceProof,
					Verifier:   verifier,
				},
			)
			if err != nil {
				return apiCommandError(err)
			}
			scoped := scopedCredentials{
				Credentials: credentials,
				RegistryCredentials: store.RegistryCredentials{
					PersonalToken: exchanged.PersonalToken,
					PersonalTokenExpires: exchanged.ExpiresAt.Format(
						time.RFC3339Nano,
					),
				},
			}
			if err := a.refreshAccess(ctx, &scoped); err != nil {
				return err
			}
			if err := updateCredentials(ctx, func(saved *store.Credentials) error {
				if saved.DeviceID != credentials.DeviceID {
					return errors.New("device credentials changed during authorization")
				}
				return saved.SetRegistry(a.registry, scoped.RegistryCredentials)
			}); err != nil {
				return err
			}
			return a.printer().Success(
				map[string]any{
					"deviceId":               scoped.DeviceID,
					"deviceName":             scoped.DeviceName,
					"personalTokenExpiresAt": scoped.PersonalTokenExpires,
				},
				a.printer().Done("Authorized", scoped.DeviceName),
			)
		},
	}
	command.Flags().SortFlags = false
	return registryCommand(command)
}

func loadOrCreateDeviceCredentials(ctx context.Context) (store.Credentials, error) {
	var credentials store.Credentials
	err := withCredentialLock(ctx, func() error {
		loaded, err := store.Load()
		if err == nil &&
			loaded.DeviceID != "" &&
			loaded.PrivateKey != "" &&
			loaded.PublicKey != "" {
			credentials = loaded
			return nil
		}
		if err != nil && !errors.Is(err, store.ErrNotLoggedIn) {
			return credentialStoreError(err)
		}
		if err == nil && len(loaded.Registries) > 0 {
			return credentialStoreError(errors.New("saved device credentials are incomplete"))
		}
		device, err := proof.NewDevice()
		if err != nil {
			return internalError("DEVICE_KEY_FAILED", err)
		}
		hostname, hostnameErr := os.Hostname()
		if hostnameErr != nil || strings.TrimSpace(hostname) == "" {
			hostname = "unUI CLI"
		}
		if len(hostname) > 120 {
			hostname = hostname[:120]
		}
		credentials = store.Credentials{
			DeviceID:   device.DeviceID,
			DeviceName: hostname,
			Platform:   runtime.GOOS + "/" + runtime.GOARCH,
			PrivateKey: device.PrivateKey,
			PublicKey:  device.PublicKey,
		}
		if err := store.Save(credentials); err != nil {
			return credentialStoreError(err)
		}
		return nil
	})
	return credentials, err
}

func (a *app) showAuthCommand() *cobra.Command {
	return registryCommand(&cobra.Command{
		Use:   "show",
		Short: "Show account, membership, device, and credential details",
		Args:  noArgs,
		Example: `  unui auth show
  unui auth show --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := a.context(cmd.Context())
			defer cancel()
			credentials, err := a.credentialsWithAccess(ctx)
			if err != nil {
				return err
			}
			status, _, err := accessRequest(
				a,
				ctx,
				credentials,
				func(accessToken string) (api.AuthShowResponse, error) {
					return a.client().ShowAuth(ctx, accessToken)
				},
			)
			if err != nil {
				return err
			}
			return a.printer().Success(status, a.authShowOutput(status))
		},
	})
}

func (a *app) authShowOutput(status api.AuthShowResponse) string {
	access := a.printer().Done("CLI access", "Ready")
	if !status.Membership.CanUseCLI {
		access = a.printer().Warning(
			"CLI access requires an active Pro membership.",
		)
	}
	membership := status.Membership.Level
	if !status.Membership.IsAdmin && status.Membership.ExpiresAt != nil {
		membership += " · expires " + authShowTime(*status.Membership.ExpiresAt)
	}
	return strings.Join(
		[]string{
			a.printer().Done("Signed in", status.Account.Email),
			access,
			"",
			a.printer().Info("Membership", membership),
			a.printer().Info(
				"Device",
				status.Device.DeviceName+" · "+status.Device.Platform,
			),
			a.printer().Info("Account ID", status.Account.ID),
			a.printer().Info("Device ID", status.Device.DeviceID),
			a.printer().Info(
				"Credential expires",
				authShowTime(status.PersonalTokenExpiresAt),
			),
		},
		"\n",
	)
}

func authShowTime(value time.Time) string {
	return value.Local().Format("Jan 2, 2006 15:04 MST")
}

func (a *app) logoutCommand() *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "logout",
		Short: "Revoke this device credential and clear saved credentials",
		Args:  noArgs,
		Example: `  unui auth logout
  unui auth logout --yes`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			saved, err := store.Load()
			if errors.Is(err, store.ErrNotLoggedIn) {
				return newCommandError(
					"NOT_LOGGED_IN",
					message.NothingToLogOut(),
					nil,
				)
			}
			if err != nil {
				return credentialStoreError(err)
			}
			registryCredentials, err := saved.ForRegistry(a.registry)
			if err != nil {
				return credentialStoreError(err)
			}
			if registryCredentials.Empty() {
				return newCommandError(
					"NOT_LOGGED_IN",
					message.NothingToLogOut(),
					nil,
				)
			}
			credentials := scopedCredentials{
				Credentials:         saved,
				RegistryCredentials: registryCredentials,
			}
			if !a.json && !yes {
				confirmed, err := promptForConfirmation(
					cmd.InOrStdin(),
					cmd.ErrOrStderr(),
					"Revoke this CLI device authorization?",
				)
				if err != nil {
					return internalError("PROMPT_FAILED", err)
				}
				if !confirmed {
					return a.printer().Success(
						map[string]any{"revoked": false},
						a.printer().Info(
							"Canceled",
							"No credentials were changed",
						),
					)
				}
			}

			ctx, cancel := a.context(cmd.Context())
			defer cancel()
			deviceProof, proofErr := proof.Sign(
				credentials.PrivateKey,
				"logout",
				credentials.DeviceID,
				time.Now(),
			)
			remoteRevoked := false
			var remoteRevocationIssue map[string]string
			if proofErr == nil && credentials.PersonalToken != "" {
				logoutErr := a.client().Logout(
					ctx,
					credentials.PersonalToken,
					api.AccessTokenRequest{
						DeviceID: credentials.DeviceID,
						Proof:    deviceProof,
					},
				)
				remoteRevoked = logoutErr == nil
				if logoutErr != nil {
					remoteRevocationIssue = a.errorSummary(
						apiCommandError(logoutErr),
					)
				}
			}
			if proofErr != nil && credentials.PersonalToken != "" {
				remoteRevocationIssue = a.errorSummary(
					internalError("DEVICE_PROOF_FAILED", proofErr),
				)
			}
			if err := updateCredentials(ctx, func(current *store.Credentials) error {
				if err := current.SetRegistry(a.registry, store.RegistryCredentials{}); err != nil {
					return err
				}
				if len(current.Registries) == 0 {
					*current = store.Credentials{}
				}
				return nil
			}); err != nil {
				return err
			}
			human := a.printer().Done("Logged out", credentials.DeviceName)
			if remoteRevocationIssue != nil {
				human = a.printer().Warning(
					"Local credentials were removed, but remote revocation could not be confirmed.",
				)
			}
			return a.printer().Success(
				map[string]any{
					"localCredentialDeleted": true,
					"remoteRevocationIssue":  remoteRevocationIssue,
					"remoteRevoked":          remoteRevoked,
				},
				human,
			)
		},
	}
	command.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation")
	command.Flags().SortFlags = false
	return registryCommand(command)
}
