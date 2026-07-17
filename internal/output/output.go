package output

import (
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

const SchemaVersion = "1"

type Error struct {
	Code    string `json:"code"`
	Title   string `json:"title"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
	Details any    `json:"details"`
}

type Envelope struct {
	SchemaVersion string `json:"schemaVersion"`
	OK            bool   `json:"ok"`
	ExitCode      int    `json:"exitCode"`
	Data          any    `json:"data"`
	Error         *Error `json:"error"`
}

type Printer struct {
	JSON    bool
	NoColor bool
	Stdout  io.Writer
	Stderr  io.Writer
	Verbose bool
}

func (p Printer) Success(data any, human string) error {
	if p.JSON {
		return json.NewEncoder(p.Stdout).Encode(Envelope{
			SchemaVersion: SchemaVersion,
			OK:            true,
			ExitCode:      0,
			Data:          data,
			Error:         nil,
		})
	}

	if human == "" {
		encoded, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		human = string(encoded)
	}
	_, err := fmt.Fprintln(p.Stdout, human)
	return err
}

func (p Printer) Failure(
	exitCode int,
	code string,
	title string,
	message string,
	hint string,
	details any,
) {
	if p.JSON {
		_ = json.NewEncoder(p.Stdout).Encode(Envelope{
			SchemaVersion: SchemaVersion,
			OK:            false,
			ExitCode:      exitCode,
			Data:          nil,
			Error: &Error{
				Code:    code,
				Title:   title,
				Message: message,
				Hint:    hint,
				Details: details,
			},
		})
		return
	}

	_, _ = fmt.Fprintf(
		p.Stderr,
		"%s %s\n",
		p.style(p.Stderr, lipgloss.NewStyle().Foreground(lipgloss.Red), "✖"),
		p.style(p.Stderr, lipgloss.NewStyle().Bold(true), title),
	)
	if strings.TrimSpace(message) != "" {
		_, _ = fmt.Fprintf(p.Stderr, "  %s\n", message)
	}
	_, _ = fmt.Fprintf(
		p.Stderr,
		"  %s\n",
		p.style(
			p.Stderr,
			lipgloss.NewStyle().Foreground(lipgloss.BrightBlack),
			"Error code: "+code,
		),
	)
	if strings.TrimSpace(hint) != "" {
		_, _ = fmt.Fprintf(
			p.Stderr,
			"\n  %s %s\n",
			p.style(
				p.Stderr,
				lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Cyan),
				"Next:",
			),
			hint,
		)
	}
	if p.Verbose && details != nil {
		_, _ = fmt.Fprintf(
			p.Stderr,
			"\n  %s\n%s\n",
			p.style(
				p.Stderr,
				lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Yellow),
				"Details",
			),
			indent(formatDetails(details), "    "),
		)
	}
}

func (p Printer) Info(label, value string) string {
	return p.status(p.Stdout, "➜", lipgloss.Cyan, label, value)
}

func (p Printer) Done(label, value string) string {
	return p.status(p.Stdout, "✓", lipgloss.Green, label, value)
}

func (p Printer) Warning(message string) string {
	return fmt.Sprintf(
		"%s %s",
		p.style(
			p.Stdout,
			lipgloss.NewStyle().Foreground(lipgloss.Yellow),
			"⚠",
		),
		message,
	)
}

func (p Printer) status(
	writer io.Writer,
	symbol string,
	color color.Color,
	label string,
	value string,
) string {
	return fmt.Sprintf(
		"%s %s  %s",
		p.style(writer, lipgloss.NewStyle().Foreground(color), symbol),
		p.style(
			writer,
			lipgloss.NewStyle().Bold(true).Foreground(color),
			label,
		),
		value,
	)
}

func (p Printer) style(
	writer io.Writer,
	style lipgloss.Style,
	value string,
) string {
	if !p.colorEnabled(writer) {
		return value
	}
	return style.Render(value)
}

func (p Printer) colorEnabled(writer io.Writer) bool {
	if p.NoColor {
		return false
	}
	return colorprofile.Detect(writer, os.Environ()) > colorprofile.ASCII
}

func formatDetails(details any) string {
	if text, ok := details.(string); ok {
		return text
	}
	encoded, err := json.MarshalIndent(details, "", "  ")
	if err != nil {
		return fmt.Sprint(details)
	}
	return string(encoded)
}

func indent(value, prefix string) string {
	return prefix + strings.ReplaceAll(value, "\n", "\n"+prefix)
}
