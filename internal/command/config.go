package command

import (
	"fmt"

	"github.com/spf13/cobra"
	cliconfig "github.com/unix/unui/internal/config"
	"github.com/unix/unui/internal/home"
)

func (a *app) configCommand() *cobra.Command {
	config := &cobra.Command{
		Use:   "config [command]",
		Short: "Manage CLI configuration",
		Long:  "Inspect and manage the local unUI CLI configuration.",
	}
	config.AddCommand(
		a.configShowCommand(),
		a.configSetCommand(),
		a.configResetCommand(),
		a.configPathCommand(),
	)
	return config
}

func (a *app) configShowCommand() *cobra.Command {
	return registryCommand(&cobra.Command{
		Use:   "show",
		Short: "Show the effective CLI configuration",
		Args:  noArgs,
		Example: `  unui config show
  unui config show --json`,
		RunE: func(_ *cobra.Command, _ []string) error {
			path, err := a.configStore.Path()
			if err != nil {
				return configCommandError(err)
			}
			return a.printer().Success(
				map[string]any{
					"configFile": path,
					"source":     a.registrySource,
				},
				fmt.Sprintf(
					"%s\n%s",
					a.printer().Info("Source", a.registrySource),
					a.printer().Info("Config file", home.DisplayPath(path)),
				),
			)
		},
	})
}

func (a *app) configSetCommand() *cobra.Command {
	var registry string
	command := &cobra.Command{
		Use:     "set",
		Short:   "Set a CLI configuration value",
		Args:    noArgs,
		Example: `  unui config set --registry <url>`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !cmd.Flags().Changed("registry") {
				return missingRequiredOptionError("--registry", cmd)
			}
			if err := a.setRegistry(registry); err != nil {
				return err
			}
			return a.printer().Success(
				map[string]any{"configured": true},
				a.printer().Done("API environment", "Configured"),
			)
		},
	}
	command.Flags().StringVar(
		&registry,
		"registry",
		"",
		"registry URL",
	)
	command.Flags().SortFlags = false
	return command
}

func (a *app) configResetCommand() *cobra.Command {
	var registry bool
	command := &cobra.Command{
		Use:     "reset",
		Short:   "Reset a CLI configuration value",
		Args:    noArgs,
		Example: `  unui config reset --registry`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !registry {
				return missingRequiredOptionError("--registry", cmd)
			}
			if err := a.configStore.ResetRegistry(); err != nil {
				return configCommandError(err)
			}
			a.registry = cliconfig.DefaultRegistry
			a.registrySource = "default"
			return a.printer().Success(
				map[string]any{
					"reset": true,
				},
				a.printer().Done("API environment", "Reset"),
			)
		},
	}
	command.Flags().BoolVar(
		&registry,
		"registry",
		false,
		"reset the registry to its default",
	)
	command.Flags().SortFlags = false
	return command
}

func (a *app) configPathCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the CLI configuration file path",
		Args:  noArgs,
		Example: `  unui config path
  unui config path --json`,
		RunE: func(_ *cobra.Command, _ []string) error {
			path, err := a.configStore.Path()
			if err != nil {
				return configCommandError(err)
			}
			return a.printer().Success(
				map[string]any{"path": path},
				path,
			)
		},
	}
}
