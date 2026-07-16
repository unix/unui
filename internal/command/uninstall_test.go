package command

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/unix/unui-cli/internal/installation"
	"github.com/unix/unui-cli/internal/output"
)

func TestUninstallJSONShowsNPMRemovalCommand(t *testing.T) {
	info := installation.Info{
		Command: []string{
			"pnpm",
			"remove",
			"-g",
			installation.NPMPackageName,
		},
		Confidence:     installation.ConfidenceReported,
		ExecutablePath: "/tmp/unui",
		Global:         true,
		Manager:        installation.ManagerPNPM,
		Source:         installation.SourceNPM,
	}
	result, stderr := runUninstallCommand(t, info)

	if stderr != "" {
		t.Fatalf("stderr must be empty in JSON mode: %q", stderr)
	}
	if result.Removed || result.RequiresConfirmation {
		t.Fatalf("unexpected npm uninstall result: %#v", result)
	}
	if !reflect.DeepEqual(result.Installation.Command, info.Command) {
		t.Fatalf("unexpected npm uninstall command: %#v", result.Installation.Command)
	}
}

func TestExecuteUninstallUsesNPMLauncherEnvironment(t *testing.T) {
	t.Setenv("UNUI_INTERNAL_INSTALL_SOURCE", "npm")
	t.Setenv("UNUI_INTERNAL_NPM_CLIENT", "bun")
	t.Setenv("UNUI_INTERNAL_NPM_GLOBAL", "1")
	t.Setenv("UNUI_INTERNAL_NPM_TEMPORARY", "0")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := execute(
		[]string{"uninstall", "--json"},
		&stdout,
		&stderr,
		testBuildInfo,
	)
	if exitCode != 0 {
		t.Fatalf("unexpected exit code: %d\n%s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty in JSON mode: %q", stderr.String())
	}

	var envelope struct {
		Data uninstallResult `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode uninstall output: %v\n%s", err, stdout.String())
	}
	expected := []string{
		"bun",
		"remove",
		"-g",
		installation.NPMPackageName,
	}
	if !reflect.DeepEqual(envelope.Data.Installation.Command, expected) {
		t.Fatalf(
			"unexpected launcher uninstall command: %#v",
			envelope.Data.Installation.Command,
		)
	}
}

func TestUninstallJSONRequiresYesBeforeInstallScriptRemoval(t *testing.T) {
	info := verifiedInstallScriptInfo()
	removeCalls := 0
	result, stderr := runUninstallCommandWithRemover(
		t,
		info,
		nil,
		func(installation.Info) error {
			removeCalls++
			return nil
		},
	)

	if stderr != "" {
		t.Fatalf("stderr must be empty in JSON mode: %q", stderr)
	}
	if removeCalls != 0 {
		t.Fatalf("remove must not run without --yes, received %d calls", removeCalls)
	}
	if result.Removed || !result.RequiresConfirmation {
		t.Fatalf("unexpected install.sh uninstall plan: %#v", result)
	}
}

func TestUninstallYesRemovesInstallScriptInstallation(t *testing.T) {
	info := verifiedInstallScriptInfo()
	removeCalls := 0
	result, stderr := runUninstallCommandWithRemover(
		t,
		info,
		[]string{"--yes"},
		func(received installation.Info) error {
			removeCalls++
			if !reflect.DeepEqual(received, info) {
				t.Fatalf("unexpected installation passed to remover: %#v", received)
			}
			return nil
		},
	)

	if stderr != "" {
		t.Fatalf("stderr must be empty in JSON mode: %q", stderr)
	}
	if removeCalls != 1 {
		t.Fatalf("expected one remove call, received %d", removeCalls)
	}
	if !result.Removed || result.RequiresConfirmation {
		t.Fatalf("unexpected install.sh uninstall result: %#v", result)
	}
}

func TestUninstallYesOnlyShowsInstallPowerShellRemovalCommand(t *testing.T) {
	info := verifiedInstallPowerShellInfo()
	removeCalls := 0
	result, stderr := runUninstallCommandWithRemover(
		t,
		info,
		[]string{"--yes"},
		func(installation.Info) error {
			removeCalls++
			return nil
		},
	)

	if stderr != "" {
		t.Fatalf("stderr must be empty in JSON mode: %q", stderr)
	}
	if removeCalls != 0 {
		t.Fatalf(
			"install.ps1 removal must not execute, received %d calls",
			removeCalls,
		)
	}
	if result.Removed || result.RequiresConfirmation {
		t.Fatalf("unexpected install.ps1 uninstall result: %#v", result)
	}
	if !reflect.DeepEqual(result.Installation.Command, info.Command) {
		t.Fatalf(
			"unexpected install.ps1 removal command: %#v",
			result.Installation.Command,
		)
	}
}

func TestQuotePowerShellLiteral(t *testing.T) {
	value := `C:\Users\O'Brien\AppData\Local\Programs\unUI`
	expected := `'C:\Users\O''Brien\AppData\Local\Programs\unUI'`
	if received := quotePowerShellLiteral(value); received != expected {
		t.Fatalf("unexpected PowerShell literal: %q", received)
	}
}

func TestUninstallInstallPowerShellPrintsSafeRemovalCommand(t *testing.T) {
	info := verifiedInstallPowerShellInfo()
	info.InstallDirectory =
		`C:\Users\O'Brien\AppData\Local\Programs\unUI`
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	removeCalls := 0
	application := &app{
		detectInstall: func() (installation.Info, error) {
			return info, nil
		},
		noColor: true,
		removeInstall: func(installation.Info) error {
			removeCalls++
			return nil
		},
		stderr: &stderr,
		stdout: &stdout,
	}
	command := application.uninstallCommand()
	command.SetArgs([]string{"--yes"})
	if err := command.Execute(); err != nil {
		t.Fatalf("execute uninstall command: %v", err)
	}

	if stderr.Len() != 0 {
		t.Fatalf("stderr must be empty: %q", stderr.String())
	}
	if removeCalls != 0 {
		t.Fatalf(
			"install.ps1 removal must not execute, received %d calls",
			removeCalls,
		)
	}
	expected := `Remove-Item -LiteralPath 'C:\Users\O''Brien\AppData\Local\Programs\unUI' -Recurse -Force`
	if !strings.Contains(stdout.String(), expected) {
		t.Fatalf(
			"PowerShell removal command was not printed safely:\n%s",
			stdout.String(),
		)
	}
}

func runUninstallCommand(
	t *testing.T,
	info installation.Info,
) (uninstallResult, string) {
	t.Helper()
	return runUninstallCommandWithRemover(t, info, nil, nil)
}

func runUninstallCommandWithRemover(
	t *testing.T,
	info installation.Info,
	args []string,
	remover func(installation.Info) error,
) (uninstallResult, string) {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	application := &app{
		detectInstall: func() (installation.Info, error) {
			return info, nil
		},
		json:          true,
		noColor:       true,
		removeInstall: remover,
		stderr:        &stderr,
		stdout:        &stdout,
	}
	command := application.uninstallCommand()
	command.SetArgs(args)
	if err := command.Execute(); err != nil {
		t.Fatalf("execute uninstall command: %v", err)
	}

	var envelope struct {
		SchemaVersion string          `json:"schemaVersion"`
		OK            bool            `json:"ok"`
		Data          uninstallResult `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode uninstall output: %v\n%s", err, stdout.String())
	}
	if !envelope.OK || envelope.SchemaVersion != output.SchemaVersion {
		t.Fatalf("unexpected uninstall envelope: %#v", envelope)
	}
	return envelope.Data, stderr.String()
}

func verifiedInstallScriptInfo() installation.Info {
	return installation.Info{
		Confidence:     installation.ConfidenceVerified,
		ExecutablePath: "/tmp/unui",
		Global:         true,
		Manager:        installation.ManagerInstallScript,
		ReceiptPath:    "/tmp/.unui-install.json",
		Source:         installation.SourceInstallScript,
	}
}

func verifiedInstallPowerShellInfo() installation.Info {
	installDirectory := `C:\Users\example\AppData\Local\Programs\unUI`
	return installation.Info{
		Command: []string{
			"Remove-Item",
			"-LiteralPath",
			installDirectory,
			"-Recurse",
			"-Force",
		},
		Confidence:       installation.ConfidenceVerified,
		ExecutablePath:   installDirectory + `\bin\unui.exe`,
		Global:           true,
		InstallDirectory: installDirectory,
		Manager:          installation.ManagerInstallPowerShell,
		ReceiptPath:      installDirectory + `\bin\.unui-install.json`,
		Source:           installation.SourceInstallPowerShell,
	}
}
