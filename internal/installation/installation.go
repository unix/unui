package installation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	ConfidenceHeuristic = "heuristic"
	ConfidenceReported  = "reported"
	ConfidenceUnknown   = "unknown"
	ConfidenceVerified  = "verified"

	ManagerBun               = "bun"
	ManagerInstallPowerShell = "install.ps1"
	ManagerInstallScript     = "install.sh"
	ManagerNPM               = "npm"
	ManagerPNPM              = "pnpm"
	ManagerUnknown           = "unknown"
	ManagerYarn              = "yarn"

	NPMPackageName       = "@unix/unui"
	ReceiptFilename      = ".unui-install.json"
	ReceiptSchemaVersion = "1"

	SourceInstallScript     = "install.sh"
	SourceInstallPowerShell = "install.ps1"
	SourceNPM               = "npm"
	SourceUnknown           = "unknown"

	environmentInstallSource = "UNUI_INTERNAL_INSTALL_SOURCE"
	environmentNPMClient     = "UNUI_INTERNAL_NPM_CLIENT"
	environmentNPMGlobal     = "UNUI_INTERNAL_NPM_GLOBAL"
	environmentNPMTemporary  = "UNUI_INTERNAL_NPM_TEMPORARY"
)

var (
	ErrInvalidReceipt = errors.New("invalid installation receipt")
	ErrUnsafeRemoval  = errors.New("installation cannot be safely removed")
)

type Info struct {
	Command          []string `json:"command,omitempty"`
	Confidence       string   `json:"confidence"`
	ExecutablePath   string   `json:"executablePath"`
	Global           bool     `json:"global"`
	InstallDirectory string   `json:"installDirectory,omitempty"`
	Manager          string   `json:"manager"`
	ReceiptPath      string   `json:"receiptPath,omitempty"`
	Source           string   `json:"source"`
	Temporary        bool     `json:"temporary"`
}

type receipt struct {
	Method        string `json:"method"`
	SHA256        string `json:"sha256"`
	SchemaVersion string `json:"schemaVersion"`
	Version       string `json:"version"`
}

func Detect() (Info, error) {
	executable, err := os.Executable()
	if err != nil {
		return Info{}, err
	}
	return detect(executable, os.Getenv)
}

func Remove(info Info) error {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return fmt.Errorf("%w: self-removal is unsupported on %s", ErrUnsafeRemoval, runtime.GOOS)
	}
	if info.Source != SourceInstallScript ||
		info.Manager != ManagerInstallScript ||
		info.Confidence != ConfidenceVerified {
		return fmt.Errorf("%w: source %q is not a verified install.sh installation", ErrUnsafeRemoval, info.Source)
	}

	verified, found, err := detectScriptInstallation(info.ExecutablePath)
	if err != nil {
		return err
	}
	if !found ||
		verified.ExecutablePath != info.ExecutablePath ||
		verified.ReceiptPath != info.ReceiptPath {
		return fmt.Errorf("%w: the installation changed after detection", ErrUnsafeRemoval)
	}
	if err := os.Remove(verified.ExecutablePath); err != nil {
		return fmt.Errorf("remove executable: %w", err)
	}
	if err := os.Remove(verified.ReceiptPath); err != nil {
		return fmt.Errorf("remove receipt: %w", err)
	}
	return nil
}

func detect(executable string, getenv func(string) string) (Info, error) {
	executable = canonicalPath(executable)
	info, found, err := detectScriptInstallation(executable)
	if err != nil {
		return Info{}, err
	}
	if found {
		return info, nil
	}

	if strings.TrimSpace(getenv(environmentInstallSource)) == SourceNPM {
		return npmInfo(
			executable,
			getenv(environmentNPMClient),
			getenv(environmentNPMGlobal) == "1",
			getenv(environmentNPMTemporary) == "1",
			ConfidenceReported,
		), nil
	}

	manager, global, temporary, found := detectNPMPath(executable)
	if found {
		return npmInfo(
			executable,
			manager,
			global,
			temporary,
			ConfidenceHeuristic,
		), nil
	}

	return Info{
		Confidence:     ConfidenceUnknown,
		ExecutablePath: executable,
		Manager:        ManagerUnknown,
		Source:         SourceUnknown,
	}, nil
}

func detectScriptInstallation(executable string) (Info, bool, error) {
	executable = canonicalPath(executable)
	receiptPath := filepath.Join(filepath.Dir(executable), ReceiptFilename)
	fileInfo, err := os.Lstat(receiptPath)
	if errors.Is(err, os.ErrNotExist) {
		return Info{}, false, nil
	}
	if err != nil {
		return Info{}, false, err
	}
	if !fileInfo.Mode().IsRegular() {
		return Info{}, false, invalidReceipt(receiptPath, "receipt is not a regular file")
	}

	file, err := os.Open(receiptPath)
	if err != nil {
		return Info{}, false, err
	}
	defer file.Close()

	var value receipt
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return Info{}, false, invalidReceipt(receiptPath, err.Error())
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Info{}, false, invalidReceipt(receiptPath, "receipt contains trailing data")
	}
	if value.SchemaVersion != ReceiptSchemaVersion {
		return Info{}, false, invalidReceipt(receiptPath, "unsupported schema version")
	}
	if value.Method != SourceInstallScript &&
		value.Method != SourceInstallPowerShell {
		return Info{}, false, invalidReceipt(receiptPath, "unexpected installation method")
	}
	if strings.TrimSpace(value.Version) == "" {
		return Info{}, false, invalidReceipt(receiptPath, "version is missing")
	}
	if len(value.SHA256) != sha256.Size*2 {
		return Info{}, false, invalidReceipt(receiptPath, "SHA-256 is invalid")
	}
	if _, err := hex.DecodeString(value.SHA256); err != nil {
		return Info{}, false, invalidReceipt(receiptPath, "SHA-256 is invalid")
	}

	checksum, err := fileSHA256(executable)
	if err != nil {
		return Info{}, false, err
	}
	if !strings.EqualFold(checksum, value.SHA256) {
		return Info{}, false, invalidReceipt(receiptPath, "binary checksum does not match")
	}

	info := Info{
		Confidence:     ConfidenceVerified,
		ExecutablePath: executable,
		Global:         true,
		Manager:        value.Method,
		ReceiptPath:    receiptPath,
		Source:         value.Method,
	}
	if value.Method == SourceInstallPowerShell {
		installDirectory, err := powerShellInstallDirectory(
			executable,
			receiptPath,
		)
		if err != nil {
			return Info{}, false, err
		}
		info.Command = []string{
			"Remove-Item",
			"-LiteralPath",
			installDirectory,
			"-Recurse",
			"-Force",
		}
		info.InstallDirectory = installDirectory
	}
	return info, true, nil
}

func npmInfo(
	executable string,
	manager string,
	global bool,
	temporary bool,
	confidence string,
) Info {
	manager = normalizeManager(manager)
	info := Info{
		Confidence:     confidence,
		ExecutablePath: executable,
		Global:         global,
		Manager:        manager,
		Source:         SourceNPM,
		Temporary:      temporary,
	}
	if !global || temporary {
		return info
	}
	switch manager {
	case ManagerNPM:
		info.Command = []string{"npm", "uninstall", "-g", NPMPackageName}
	case ManagerPNPM:
		info.Command = []string{"pnpm", "remove", "-g", NPMPackageName}
	case ManagerYarn:
		info.Command = []string{"yarn", "global", "remove", NPMPackageName}
	case ManagerBun:
		info.Command = []string{"bun", "remove", "-g", NPMPackageName}
	}
	return info
}

func normalizeManager(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ManagerNPM:
		return ManagerNPM
	case ManagerPNPM:
		return ManagerPNPM
	case ManagerYarn:
		return ManagerYarn
	case ManagerBun:
		return ManagerBun
	default:
		return ManagerUnknown
	}
}

func detectNPMPath(executable string) (string, bool, bool, bool) {
	path := strings.ToLower(filepath.ToSlash(executable))
	if !strings.Contains(path, "/node_modules/@unix/unui") &&
		!strings.Contains(path, "/.pnpm/@unix+unui") {
		return "", false, false, false
	}
	switch {
	case strings.Contains(path, "/.npm/_npx/"):
		return ManagerNPM, false, true, true
	case strings.Contains(path, "/.bun/install/cache/"):
		return ManagerBun, false, true, true
	case strings.Contains(path, "/pnpm/dlx/"):
		return ManagerPNPM, false, true, true
	case strings.Contains(path, "/.bun/install/global/"):
		return ManagerBun, true, false, true
	case strings.Contains(path, "/yarn/global/"):
		return ManagerYarn, true, false, true
	case strings.Contains(path, "/pnpm/global/"):
		return ManagerPNPM, true, false, true
	case strings.Contains(path, "/.pnpm/"):
		return ManagerPNPM, false, false, true
	case strings.Contains(path, "/lib/node_modules/"),
		strings.Contains(path, "/npm/node_modules/"):
		return ManagerNPM, true, false, true
	default:
		return ManagerUnknown, false, false, true
	}
}

func canonicalPath(path string) string {
	absolute, err := filepath.Abs(path)
	if err == nil {
		path = absolute
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		path = resolved
	}
	return filepath.Clean(path)
}

func powerShellInstallDirectory(
	executable string,
	receiptPath string,
) (string, error) {
	return validatePowerShellInstallDirectory(
		executable,
		receiptPath,
		runtime.GOOS,
		os.Getenv("LOCALAPPDATA"),
	)
}

func validatePowerShellInstallDirectory(
	executable string,
	receiptPath string,
	operatingSystem string,
	localAppData string,
) (string, error) {
	if !strings.EqualFold(filepath.Base(executable), "unui.exe") {
		return "", invalidReceipt(receiptPath, "unexpected Windows executable name")
	}
	binaryDirectory := filepath.Dir(executable)
	if !strings.EqualFold(filepath.Base(binaryDirectory), "bin") {
		return "", invalidReceipt(receiptPath, "unexpected Windows binary directory")
	}
	installDirectory := filepath.Dir(binaryDirectory)
	if !strings.EqualFold(filepath.Base(installDirectory), "unUI") {
		return "", invalidReceipt(receiptPath, "unexpected Windows installation directory")
	}
	if operatingSystem != "windows" {
		return installDirectory, nil
	}
	localAppData = strings.TrimSpace(localAppData)
	if localAppData == "" {
		return "", invalidReceipt(receiptPath, "LOCALAPPDATA is unavailable")
	}
	expected := canonicalPath(filepath.Join(localAppData, "Programs", "unUI"))
	if !strings.EqualFold(installDirectory, expected) {
		return "", invalidReceipt(receiptPath, "Windows installation path does not match the fixed directory")
	}
	return installDirectory, nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func invalidReceipt(path string, reason string) error {
	return fmt.Errorf("%w at %s: %s", ErrInvalidReceipt, path, reason)
}
