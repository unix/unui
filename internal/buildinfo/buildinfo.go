package buildinfo

import (
	"runtime"
	"runtime/debug"
	"strings"
)

var (
	Version = "dev"
	Commit  = ""
	Date    = ""
)

type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	Dirty     bool   `json:"dirty"`
	GoVersion string `json:"go"`
}

type stamp struct {
	version string
	commit  string
	date    string
}

func Read() Info {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		info = nil
	}
	return resolve(
		stamp{
			version: Version,
			commit:  Commit,
			date:    Date,
		},
		runtime.Version(),
		info,
	)
}

func resolve(stamped stamp, runtimeVersion string, build *debug.BuildInfo) Info {
	info := Info{
		Version:   normalizeVersion(stamped.version),
		Commit:    strings.TrimSpace(stamped.commit),
		Date:      strings.TrimSpace(stamped.date),
		GoVersion: runtimeVersion,
	}
	if build == nil {
		info.Dirty = strings.HasSuffix(info.Version, "+dirty")
		return info
	}
	if build.GoVersion != "" {
		info.GoVersion = build.GoVersion
	}
	if info.Version == "dev" {
		info.Version = normalizeVersion(build.Main.Version)
	}
	for _, setting := range build.Settings {
		switch setting.Key {
		case "vcs.revision":
			if info.Commit == "" {
				info.Commit = setting.Value
			}
		case "vcs.time":
			if info.Date == "" {
				info.Date = setting.Value
			}
		case "vcs.modified":
			info.Dirty = setting.Value == "true"
		}
	}
	if strings.HasSuffix(info.Version, "+dirty") {
		info.Dirty = true
	}
	return info
}

func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	switch strings.ToLower(version) {
	case "", "(devel)", "devel", "dev":
		return "dev"
	}
	return strings.TrimPrefix(version, "v")
}
