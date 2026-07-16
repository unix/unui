package command

import (
	"encoding/json"
	"strings"

	"github.com/spf13/cobra"
	"github.com/unix/unui/internal/message"
)

func (a *app) askCommand() *cobra.Command {
	var category string
	var limit int
	command := &cobra.Command{
		Use:   "ask <task>",
		Short: "Request a one-shot design evidence pack",
		Long:  "Request focused design evidence that a coding agent can use to implement a specific interface task.",
		Args:  exactArgs("task"),
		Example: `  unui ask "Build a dense SaaS billing settings page"
  unui ask "Design a pricing comparison" --category pricing --limit 6 --json`,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			if limit >= 1 && limit <= 12 {
				return nil
			}
			return newCommandError(
				"INVALID_LIMIT",
				message.InvalidLimit(limit),
				map[string]any{"limit": limit},
			)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := a.context(cmd.Context())
			defer cancel()
			credentials, err := a.credentialsWithAccess(ctx)
			if err != nil {
				return err
			}
			input := map[string]any{
				"limit": limit,
				"task":  args[0],
			}
			if strings.TrimSpace(category) != "" {
				input["category"] = category
			}
			result, _, err := accessRequest(
				a,
				ctx,
				credentials,
				func(accessToken string) (json.RawMessage, error) {
					return a.client().Ask(ctx, accessToken, input)
				},
			)
			if err != nil {
				return err
			}
			return a.printer().Success(result, prettyJSON(result))
		},
	}
	command.Flags().StringVar(
		&category,
		"category",
		"",
		"optional evidence category",
	)
	command.Flags().IntVar(
		&limit,
		"limit",
		8,
		"maximum evidence references",
	)
	command.Flags().SortFlags = false
	return registryCommand(command)
}
