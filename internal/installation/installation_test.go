package installation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestDetectVerifiesInstallScriptReceipt(t *testing.T) {
	executable, receiptPath := writeInstallScriptFixture(t, []byte("unui binary"))

	info, err := detect(executable, emptyEnvironment)
	if err != nil {
		t.Fatalf("detect installation: %v", err)
	}
	if info.Source != SourceInstallScript ||
		info.Manager != ManagerInstallScript ||
		info.Confidence != ConfidenceVerified ||
		info.ExecutablePath != executable ||
		info.ReceiptPath != receiptPath ||
		!info.Global {
		t.Fatalf("unexpected installation info: %#v", info)
	}
}

func TestDetectRejectsMismatchedInstallScriptReceipt(t *testing.T) {
	executable, receiptPath := writeInstallScriptFixture(t, []byte("unui binary"))
	value := readReceipt(t, receiptPath)
	value.SHA256 = hex.EncodeToString(make([]byte, sha256.Size))
	writeReceipt(t, receiptPath, value)

	_, err := detect(executable, emptyEnvironment)
	if !errors.Is(err, ErrInvalidReceipt) {
		t.Fatalf("expected invalid receipt error, received %v", err)
	}
}

func TestDetectVerifiesInstallPowerShellReceipt(t *testing.T) {
	executable, receiptPath, installDirectory := writeInstallPowerShellFixture(
		t,
		[]byte("unui windows binary"),
	)

	info, err := detect(executable, emptyEnvironment)
	if err != nil {
		t.Fatalf("detect installation: %v", err)
	}
	expectedCommand := []string{
		"Remove-Item",
		"-LiteralPath",
		installDirectory,
		"-Recurse",
		"-Force",
	}
	if info.Source != SourceInstallPowerShell ||
		info.Manager != ManagerInstallPowerShell ||
		info.Confidence != ConfidenceVerified ||
		info.ExecutablePath != executable ||
		info.InstallDirectory != installDirectory ||
		info.ReceiptPath != receiptPath ||
		!info.Global ||
		!reflect.DeepEqual(info.Command, expectedCommand) {
		t.Fatalf("unexpected installation info: %#v", info)
	}
}

func TestDetectUsesNPMLauncherMetadata(t *testing.T) {
	tests := []struct {
		manager string
		command []string
	}{
		{
			manager: ManagerNPM,
			command: []string{"npm", "uninstall", "-g", NPMPackageName},
		},
		{
			manager: ManagerPNPM,
			command: []string{"pnpm", "remove", "-g", NPMPackageName},
		},
		{
			manager: ManagerYarn,
			command: []string{"yarn", "global", "remove", NPMPackageName},
		},
		{
			manager: ManagerBun,
			command: []string{"bun", "remove", "-g", NPMPackageName},
		},
	}

	for _, test := range tests {
		t.Run(test.manager, func(t *testing.T) {
			environment := map[string]string{
				environmentInstallSource: SourceNPM,
				environmentNPMClient:     test.manager,
				environmentNPMGlobal:     "1",
			}
			info, err := detect(
				filepath.Join(t.TempDir(), "unui"),
				func(name string) string {
					return environment[name]
				},
			)
			if err != nil {
				t.Fatalf("detect installation: %v", err)
			}
			if info.Source != SourceNPM ||
				info.Manager != test.manager ||
				info.Confidence != ConfidenceReported ||
				!info.Global ||
				info.Temporary ||
				!reflect.DeepEqual(info.Command, test.command) {
				t.Fatalf("unexpected installation info: %#v", info)
			}
		})
	}
}

func TestUpdateCommandMatchesInstallationSource(t *testing.T) {
	tests := []struct {
		name     string
		info     Info
		expected string
	}{
		{
			name:     "install.sh",
			info:     Info{Source: SourceInstallScript},
			expected: InstallScriptUpdateCommand,
		},
		{
			name:     "install.ps1",
			info:     Info{Source: SourceInstallPowerShell},
			expected: InstallPowerShellUpdateCommand,
		},
		{
			name: "npm",
			info: Info{
				Global:  true,
				Manager: ManagerNPM,
				Source:  SourceNPM,
			},
			expected: "npm update -g @unix/unui",
		},
		{
			name: "pnpm",
			info: Info{
				Global:  true,
				Manager: ManagerPNPM,
				Source:  SourceNPM,
			},
			expected: "pnpm update -g --latest @unix/unui",
		},
		{
			name: "yarn",
			info: Info{
				Global:  true,
				Manager: ManagerYarn,
				Source:  SourceNPM,
			},
			expected: "yarn global upgrade @unix/unui@latest",
		},
		{
			name: "bun",
			info: Info{
				Global:  true,
				Manager: ManagerBun,
				Source:  SourceNPM,
			},
			expected: "bun add -g @unix/unui@latest",
		},
		{
			name: "temporary npm",
			info: Info{
				Manager:   ManagerNPM,
				Source:    SourceNPM,
				Temporary: true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if command := UpdateCommand(test.info); command != test.expected {
				t.Fatalf("unexpected update command: %q", command)
			}
		})
	}
}

func TestDetectDoesNotSuggestGlobalRemovalForTemporaryNPMRun(t *testing.T) {
	environment := map[string]string{
		environmentInstallSource: SourceNPM,
		environmentNPMClient:     ManagerNPM,
		environmentNPMTemporary:  "1",
	}
	info, err := detect(
		filepath.Join(t.TempDir(), "unui"),
		func(name string) string {
			return environment[name]
		},
	)
	if err != nil {
		t.Fatalf("detect installation: %v", err)
	}
	if info.Global || !info.Temporary || len(info.Command) != 0 {
		t.Fatalf("unexpected temporary installation info: %#v", info)
	}
}

func TestDetectFallsBackToNPMPathHeuristics(t *testing.T) {
	executable := filepath.Join(
		t.TempDir(),
		"pnpm",
		"global",
		"5",
		".pnpm",
		"@unix+unui-darwin-arm64@0.1.0",
		"node_modules",
		"@unix",
		"unui-darwin-arm64",
		"bin",
		"unui",
	)

	info, err := detect(executable, emptyEnvironment)
	if err != nil {
		t.Fatalf("detect installation: %v", err)
	}
	if info.Source != SourceNPM ||
		info.Manager != ManagerPNPM ||
		info.Confidence != ConfidenceHeuristic ||
		!info.Global {
		t.Fatalf("unexpected installation info: %#v", info)
	}
}

func TestRemoveDeletesVerifiedInstallScriptFiles(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("install.sh self-removal is only supported on macOS and Linux")
	}
	executable, receiptPath := writeInstallScriptFixture(t, []byte("unui binary"))
	info, err := detect(executable, emptyEnvironment)
	if err != nil {
		t.Fatalf("detect installation: %v", err)
	}

	if err := Remove(info); err != nil {
		t.Fatalf("remove installation: %v", err)
	}
	if _, err := os.Stat(executable); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected executable to be removed, received %v", err)
	}
	if _, err := os.Stat(receiptPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected receipt to be removed, received %v", err)
	}
}

func TestRemoveRejectsNPMInstallation(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "unui")
	if err := os.WriteFile(executable, []byte("unui binary"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}

	err := Remove(Info{
		Confidence:     ConfidenceReported,
		ExecutablePath: executable,
		Global:         true,
		Manager:        ManagerNPM,
		Source:         SourceNPM,
	})
	if !errors.Is(err, ErrUnsafeRemoval) {
		t.Fatalf("expected unsafe removal error, received %v", err)
	}
	if _, err := os.Stat(executable); err != nil {
		t.Fatalf("npm executable must be preserved: %v", err)
	}
}

func TestRemoveRejectsInstallPowerShellInstallation(t *testing.T) {
	executable, receiptPath, _ := writeInstallPowerShellFixture(
		t,
		[]byte("unui windows binary"),
	)
	info, err := detect(executable, emptyEnvironment)
	if err != nil {
		t.Fatalf("detect installation: %v", err)
	}

	err = Remove(info)
	if !errors.Is(err, ErrUnsafeRemoval) {
		t.Fatalf("expected unsafe removal error, received %v", err)
	}
	if _, err := os.Stat(executable); err != nil {
		t.Fatalf("PowerShell executable must be preserved: %v", err)
	}
	if _, err := os.Stat(receiptPath); err != nil {
		t.Fatalf("PowerShell receipt must be preserved: %v", err)
	}
}

func TestPowerShellInstallDirectoryRejectsCustomWindowsPath(t *testing.T) {
	localAppData := t.TempDir()
	customInstallDirectory := filepath.Join(
		t.TempDir(),
		"unUI",
	)
	executable := filepath.Join(
		customInstallDirectory,
		"bin",
		"unui.exe",
	)
	receiptPath := filepath.Join(
		filepath.Dir(executable),
		ReceiptFilename,
	)

	_, err := validatePowerShellInstallDirectory(
		executable,
		receiptPath,
		"windows",
		localAppData,
	)
	if !errors.Is(err, ErrInvalidReceipt) {
		t.Fatalf("expected invalid receipt error, received %v", err)
	}
}

func writeInstallScriptFixture(
	t *testing.T,
	payload []byte,
) (string, string) {
	t.Helper()
	directory := t.TempDir()
	executable := filepath.Join(directory, "unui")
	if err := os.WriteFile(executable, payload, 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
	checksum := sha256.Sum256(payload)
	receiptPath := filepath.Join(directory, ReceiptFilename)
	writeReceipt(t, receiptPath, receipt{
		Method:        SourceInstallScript,
		SHA256:        hex.EncodeToString(checksum[:]),
		SchemaVersion: ReceiptSchemaVersion,
		Version:       "0.1.0",
	})
	return canonicalPath(executable), canonicalPath(receiptPath)
}

func writeInstallPowerShellFixture(
	t *testing.T,
	payload []byte,
) (string, string, string) {
	t.Helper()
	localAppData := t.TempDir()
	t.Setenv("LOCALAPPDATA", localAppData)
	installDirectory := filepath.Join(localAppData, "Programs", "unUI")
	binaryDirectory := filepath.Join(installDirectory, "bin")
	if err := os.MkdirAll(binaryDirectory, 0o755); err != nil {
		t.Fatalf("create binary directory: %v", err)
	}
	executable := filepath.Join(binaryDirectory, "unui.exe")
	if err := os.WriteFile(executable, payload, 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
	checksum := sha256.Sum256(payload)
	receiptPath := filepath.Join(binaryDirectory, ReceiptFilename)
	writeReceipt(t, receiptPath, receipt{
		Method:        SourceInstallPowerShell,
		SHA256:        hex.EncodeToString(checksum[:]),
		SchemaVersion: ReceiptSchemaVersion,
		Version:       "0.1.0",
	})
	return canonicalPath(executable),
		canonicalPath(receiptPath),
		canonicalPath(installDirectory)
}

func readReceipt(t *testing.T, path string) receipt {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read receipt: %v", err)
	}
	var value receipt
	if err := json.Unmarshal(payload, &value); err != nil {
		t.Fatalf("decode receipt: %v", err)
	}
	return value
}

func writeReceipt(t *testing.T, path string, value receipt) {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode receipt: %v", err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("write receipt: %v", err)
	}
}

func emptyEnvironment(string) string {
	return ""
}
