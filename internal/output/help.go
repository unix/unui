package output

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type HelpItem struct {
	Description string `json:"description"`
	Name        string `json:"name"`
}

type HelpDocument struct {
	Command       string     `json:"command"`
	Commands      []HelpItem `json:"commands"`
	Description   string     `json:"description"`
	Examples      []string   `json:"examples"`
	GlobalOptions []HelpItem `json:"globalOptions"`
	Options       []HelpItem `json:"options"`
	Usage         string     `json:"usage"`
}

func (p Printer) Help(cmd *cobra.Command, version string) error {
	document := helpDocument(cmd)
	if p.JSON {
		return p.Success(document, "")
	}

	sections := []string{
		p.helpHeader(cmd, version, document.Description),
		p.helpSection(
			"Usage",
			"  "+p.style(
				p.Stdout,
				lipgloss.NewStyle().Foreground(lipgloss.Cyan),
				"$",
			)+" "+document.Usage,
		),
	}
	if len(document.Commands) > 0 {
		sections = append(
			sections,
			p.helpSection(
				"Commands",
				p.helpRows(document.Commands),
			),
		)
	}
	if len(document.Options) > 0 {
		sections = append(
			sections,
			p.helpSection(
				"Options",
				p.helpRows(document.Options),
			),
		)
	}
	if len(document.GlobalOptions) > 0 {
		sections = append(
			sections,
			p.helpSection(
				"Global options",
				p.helpRows(document.GlobalOptions),
			),
		)
	}
	if len(document.Examples) > 0 {
		sections = append(
			sections,
			p.helpSection(
				"Examples",
				indent(strings.Join(document.Examples, "\n"), "  "),
			),
		)
	}
	if len(document.Commands) > 0 {
		sections = append(
			sections,
			p.style(
				p.Stdout,
				lipgloss.NewStyle().Foreground(lipgloss.BrightBlack),
				fmt.Sprintf(
					"Run `%s <command> --help` for more information.",
					cmd.CommandPath(),
				),
			),
		)
	}
	_, err := fmt.Fprintln(p.Stdout, strings.Join(sections, "\n\n"))
	return err
}

func helpDocument(cmd *cobra.Command) HelpDocument {
	cmd.InitDefaultHelpFlag()
	if helpFlag := cmd.Flags().Lookup("help"); helpFlag != nil {
		helpFlag.Usage = "show help for this command"
	}

	description := cmd.Long
	if strings.TrimSpace(description) == "" {
		description = cmd.Short
	}
	return HelpDocument{
		Command:       cmd.CommandPath(),
		Commands:      commandItems(cmd),
		Description:   description,
		Examples:      exampleLines(cmd.Example),
		GlobalOptions: flagItems(cmd.InheritedFlags()),
		Options:       flagItems(cmd.NonInheritedFlags()),
		Usage:         strings.ReplaceAll(cmd.UseLine(), "[flags]", "[options]"),
	}
}

func commandItems(cmd *cobra.Command) []HelpItem {
	items := make([]HelpItem, 0, len(cmd.Commands()))
	for _, child := range cmd.Commands() {
		if !child.IsAvailableCommand() || child.Name() == "help" {
			continue
		}
		items = append(items, HelpItem{
			Description: child.Short,
			Name:        strings.ReplaceAll(child.Use, "[flags]", "[options]"),
		})
	}
	return items
}

func flagItems(flags *pflag.FlagSet) []HelpItem {
	items := make([]HelpItem, 0, flags.NFlag())
	flags.VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		items = append(items, HelpItem{
			Description: flag.Usage + flagDefault(flag),
			Name:        flagLabel(flag),
		})
	})
	return items
}

func flagLabel(flag *pflag.Flag) string {
	name := "    --" + flag.Name
	if flag.Shorthand != "" {
		name = "  -" + flag.Shorthand + ", --" + flag.Name
	}
	if flag.NoOptDefVal == "" {
		name += " <" + flagType(flag) + ">"
	}
	return name
}

func flagType(flag *pflag.Flag) string {
	switch flag.Value.Type() {
	case "int", "int32", "int64", "uint", "uint32", "uint64":
		return "number"
	default:
		return flag.Value.Type()
	}
}

func flagDefault(flag *pflag.Flag) string {
	if flag.Value.Type() == "bool" {
		return ""
	}
	switch flag.DefValue {
	case "", "0", "0s":
		return ""
	}
	if flag.Value.Type() == "string" {
		return " (default: " + strconv.Quote(flag.DefValue) + ")"
	}
	return " (default: " + flag.DefValue + ")"
}

func exampleLines(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(value), "\n")
	for index, line := range lines {
		lines[index] = strings.TrimSpace(line)
	}
	return lines
}

func (p Printer) helpHeader(
	cmd *cobra.Command,
	version string,
	description string,
) string {
	path := strings.Replace(cmd.CommandPath(), "unui", "unUI", 1)
	title := p.style(
		p.Stdout,
		lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Magenta),
		path,
	)
	versionText := p.style(
		p.Stdout,
		lipgloss.NewStyle().Foreground(lipgloss.BrightBlack),
		versionLabel(version),
	)
	return title + " " + versionText + "\n" + description
}

func versionLabel(version string) string {
	if version == "" || version == "dev" {
		return "dev"
	}
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

func (p Printer) helpSection(title string, content string) string {
	return p.style(
		p.Stdout,
		lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Cyan),
		title,
	) + "\n" + content
}

func (p Printer) helpRows(items []HelpItem) string {
	width := 0
	for _, item := range items {
		if len(item.Name) > width {
			width = len(item.Name)
		}
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		name := item.Name + strings.Repeat(" ", width-len(item.Name))
		lines = append(
			lines,
			p.style(
				p.Stdout,
				lipgloss.NewStyle().Foreground(lipgloss.Green),
				name,
			)+"  "+item.Description,
		)
	}
	return strings.Join(lines, "\n")
}
