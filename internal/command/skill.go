package command

import (
	"errors"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/unix/unui-cli/internal/home"
	"github.com/unix/unui-cli/internal/message"
	"github.com/unix/unui-cli/internal/skillinstall"
	bundledskill "github.com/unix/unui-cli/skill"
)

type updateSkillData struct {
	CLIVersion   string                `json:"cliVersion"`
	SkillVersion string                `json:"skillVersion"`
	Targets      []skillinstall.Target `json:"targets"`
}

func (a *app) updateSkillCommand() *cobra.Command {
	var client string
	command := &cobra.Command{
		Use:   "update-skill",
		Short: "Install the bundled unUI skill for local coding agents",
		Long: "Replace the user-level unUI skill for Codex, Claude Code, Cursor, " +
			"or every detected supported client with the version bundled in this CLI.",
		Args: noArgs,
		Example: `  unui update-skill
  unui update-skill --client codex
  unui update-skill --client claude --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			userHome, err := os.UserHomeDir()
			if err != nil {
				return internalError("SKILL_HOME_UNAVAILABLE", err)
			}
			targets, err := skillinstall.Targets(userHome, client)
			switch {
			case errors.Is(err, skillinstall.ErrUnsupportedClient):
				return newCommandError(
					"UNSUPPORTED_CLIENT",
					message.UnsupportedValue(
						"client",
						client,
						cmd.CommandPath(),
					),
					map[string]any{
						"supported": skillinstall.SupportedClients(),
						"value":     client,
					},
				)
			case errors.Is(err, skillinstall.ErrNoClientDetected):
				return newCommandError(
					"SKILL_CLIENT_NOT_DETECTED",
					message.SkillClientNotDetected(),
					map[string]any{
						"supported": skillinstall.SupportedClients(),
					},
				)
			case err != nil:
				return internalError("SKILL_CLIENT_DETECTION_FAILED", err)
			}
			bundle, err := bundledskill.Bundle()
			if err != nil {
				return internalError("SKILL_BUNDLE_UNAVAILABLE", err)
			}

			lines := make([]string, 0, len(targets))
			for _, target := range targets {
				if err := skillinstall.Install(bundle, target); err != nil {
					return newCommandError(
						"SKILL_INSTALL_FAILED",
						message.SkillInstallFailed(),
						map[string]any{
							"client": target.Client,
							"error":  err.Error(),
							"path":   target.Path,
						},
					)
				}
				lines = append(
					lines,
					a.printer().Done(
						"Skill updated",
						target.DisplayName+" · "+home.DisplayPath(target.Path),
					),
				)
			}
			return a.printer().Success(
				updateSkillData{
					CLIVersion:   a.buildInfo.Version,
					SkillVersion: bundledskill.Version,
					Targets:      targets,
				},
				strings.Join(lines, "\n"),
			)
		},
	}
	command.Flags().StringVar(
		&client,
		"client",
		skillinstall.ClientAuto,
		"target client: auto, codex, claude, cursor, or all",
	)
	command.Flags().SortFlags = false
	return command
}
