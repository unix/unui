package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/unix/unui/internal/api"
	"github.com/unix/unui/internal/buildinfo"
	cliconfig "github.com/unix/unui/internal/config"
	"github.com/unix/unui/internal/installation"
	"github.com/unix/unui/internal/message"
	"github.com/unix/unui/internal/output"
	"github.com/unix/unui/internal/proof"
	"github.com/unix/unui/internal/store"
)

const (
	cliAPIPath                 = "/v1/cli"
	authorizationTimeout       = 5 * time.Minute
	commandTimeout             = 30 * time.Second
	requiresRegistryAnnotation = "unui/requires-registry"
)

type app struct {
	buildInfo      buildinfo.Info
	configStore    cliconfig.Store
	detectInstall  func() (installation.Info, error)
	fetchRelease   func(context.Context) (releaseInfo, error)
	json           bool
	noColor        bool
	registry       string
	registrySource string
	removeInstall  func(installation.Info) error
	showVersion    bool
	stderr         io.Writer
	stdout         io.Writer
	verbose        bool
}

type scopedCredentials struct {
	store.Credentials
	store.RegistryCredentials
}

func Execute(args []string, stdout, stderr io.Writer) int {
	return execute(args, stdout, stderr, buildinfo.Read())
}

func execute(
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	buildInfo buildinfo.Info,
) int {
	application := &app{
		buildInfo:      buildInfo,
		configStore:    cliconfig.DefaultStore(),
		detectInstall:  installation.Detect,
		json:           containsEnabledFlag(args, "json"),
		noColor:        noColorByDefault() || containsEnabledFlag(args, "no-color"),
		registry:       cliconfig.DefaultRegistry,
		registrySource: "default",
		removeInstall:  installation.Remove,
		stderr:         stderr,
		stdout:         stdout,
		verbose:        containsEnabledFlag(args, "verbose"),
	}
	root := application.rootCommand()
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		err = normalizeCommandError(err)
		return application.renderError(err)
	}
	return 0
}

func containsEnabledFlag(args []string, name string) bool {
	flag := "--" + name
	for _, value := range args {
		if value == flag {
			return true
		}
		prefix := flag + "="
		if !strings.HasPrefix(value, prefix) {
			continue
		}
		enabled, err := strconv.ParseBool(strings.TrimPrefix(value, prefix))
		if err == nil {
			return enabled
		}
	}
	return false
}

func noColorByDefault() bool {
	_, disabled := os.LookupEnv("NO_COLOR")
	return disabled
}

func (a *app) printer() output.Printer {
	return output.Printer{
		JSON:    a.json,
		NoColor: a.noColor,
		Stderr:  a.stderr,
		Stdout:  a.stdout,
		Verbose: a.verbose,
	}
}

func (a *app) rootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "unui [command]",
		Short: "Design evidence for any coding agent",
		Long: "Retrieve focused unUI design evidence, manage device authentication, " +
			"manage the bundled unUI skill, and inspect CLI readiness from any coding agent.",
		Example: `  unui auth login
  unui skill update --client codex
  unui ask "Build a dense SaaS billing settings page" --json
  unui doctor`,
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          rootArgs,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Annotations[requiresRegistryAnnotation] != "true" {
				return nil
			}
			return a.loadRegistry()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if a.showVersion {
				return a.printer().Success(
					a.buildInfo,
					"unUI "+a.buildInfo.Version,
				)
			}
			if a.json {
				return a.printer().Help(cmd, a.buildInfo.Version)
			}
			if err := cmd.Help(); err != nil {
				return internalError("HELP_WRITE_FAILED", err)
			}
			return nil
		},
	}
	root.SetOut(a.stdout)
	root.SetErr(a.stderr)
	root.SetFlagErrorFunc(flagCommandError)
	root.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		_ = a.printer().Help(cmd, a.buildInfo.Version)
	})
	root.CompletionOptions.DisableDefaultCmd = true
	root.Flags().BoolVarP(
		&a.showVersion,
		"version",
		"v",
		false,
		"print CLI version",
	)
	root.PersistentFlags().BoolVar(
		&a.json,
		"json",
		a.json,
		"write one JSON document to stdout",
	)
	root.PersistentFlags().BoolVar(
		&a.noColor,
		"no-color",
		a.noColor,
		"disable ANSI colors",
	)
	root.PersistentFlags().BoolVar(
		&a.verbose,
		"verbose",
		a.verbose,
		"enable verbose diagnostics",
	)
	root.PersistentFlags().SortFlags = false
	root.AddCommand(
		a.authCommand(),
		a.askCommand(),
		a.configCommand(),
		a.doctorCommand(),
		a.completionCommand(root),
		a.uninstallCommand(),
		a.updateCommand(),
		a.skillCommand(),
	)
	return root
}

func (a *app) client() api.Client {
	return api.Client{
		BaseURL: a.apiURL(),
		HTTPClient: &http.Client{
			Timeout: commandTimeout,
		},
		Version: a.buildInfo.Version,
	}
}

func (a *app) apiURL() string {
	return strings.TrimRight(a.registry, "/") + cliAPIPath
}

func (a *app) loadRegistry() error {
	values, err := a.configStore.Load()
	if err != nil {
		return configCommandError(err)
	}
	a.registry = values.EffectiveRegistry()
	a.registrySource = "default"
	if values.Registry != "" {
		a.registrySource = "configFile"
	}
	return nil
}

func (a *app) setRegistry(value string) error {
	registry, err := a.configStore.SetRegistry(value)
	if err != nil {
		return registryCommandError(err)
	}
	a.registry = registry
	a.registrySource = "configFile"
	return nil
}

func (a *app) context(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, commandTimeout)
}

func (a *app) authorizationContext(
	parent context.Context,
) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, authorizationTimeout)
}

func (a *app) credentialsWithAccess(
	ctx context.Context,
) (scopedCredentials, error) {
	credentials, err := a.credentialsForRegistry()
	if err != nil {
		return scopedCredentials{}, err
	}
	if accessTokenReady(credentials.RegistryCredentials) {
		return credentials, nil
	}
	return a.credentialsAfterUnauthorized(ctx, "")
}

func (a *app) credentialsForRegistry() (scopedCredentials, error) {
	credentials, err := store.Load()
	if errors.Is(err, store.ErrNotLoggedIn) {
		return scopedCredentials{}, newCommandError(
			"NOT_LOGGED_IN",
			message.NotLoggedIn(),
			nil,
		)
	}
	if err != nil {
		return scopedCredentials{}, credentialStoreError(err)
	}
	registryCredentials, err := credentials.ForRegistry(a.registry)
	if err != nil {
		return scopedCredentials{}, credentialStoreError(err)
	}
	if registryCredentials.Empty() {
		return scopedCredentials{}, newCommandError(
			"NOT_LOGGED_IN",
			message.NotLoggedIn(),
			nil,
		)
	}
	return scopedCredentials{
		Credentials:         credentials,
		RegistryCredentials: registryCredentials,
	}, nil
}

func registryCommand(command *cobra.Command) *cobra.Command {
	if command.Annotations == nil {
		command.Annotations = make(map[string]string)
	}
	command.Annotations[requiresRegistryAnnotation] = "true"
	return command
}

func accessTokenReady(credentials store.RegistryCredentials) bool {
	if credentials.AccessToken == "" {
		return false
	}
	expiresAt, err := time.Parse(
		time.RFC3339Nano,
		credentials.AccessTokenExpiresAt,
	)
	return err == nil && expiresAt.After(time.Now().Add(30*time.Second))
}

func (a *app) refreshAccess(
	ctx context.Context,
	credentials *scopedCredentials,
) error {
	deviceProof, err := proof.Sign(
		credentials.PrivateKey,
		"access",
		credentials.DeviceID,
		time.Now(),
	)
	if err != nil {
		return internalError("DEVICE_PROOF_FAILED", err)
	}
	response, err := a.client().IssueAccessToken(
		ctx,
		credentials.PersonalToken,
		api.AccessTokenRequest{
			DeviceID: credentials.DeviceID,
			Proof:    deviceProof,
		},
	)
	if err != nil {
		return apiCommandError(err)
	}
	credentials.AccessToken = response.AccessToken
	credentials.AccessTokenExpiresAt = response.AccessTokenExpiresAt.Format(
		time.RFC3339Nano,
	)
	return nil
}

func prettyJSON(value json.RawMessage) string {
	var buffer bytes.Buffer
	if err := json.Indent(&buffer, value, "", "  "); err != nil {
		return string(value)
	}
	return buffer.String()
}
